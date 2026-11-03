package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/felipemarinho97/torrent-indexer/logging"
	"github.com/felipemarinho97/torrent-indexer/magnet"
	"github.com/felipemarinho97/torrent-indexer/schema"
	goscrape "github.com/felipemarinho97/torrent-indexer/scrape"
	"github.com/felipemarinho97/torrent-indexer/utils"
)

var rede_torrent = IndexerMeta{
	Label:       "rede_torrent",
	URL:         utils.GetIndexerURLFromEnv("INDEXER_REDE_TORRENT_URL", "https://redestorrents.com/"),
	SearchURL:   "index.php?busca=",
	PagePattern: "pagina/%s/",
}

func (i *Indexer) HandlerRedeTorrentIndexer(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	metadata := rede_torrent

	defer func() {
		i.metrics.IndexerDuration.WithLabelValues(metadata.Label).Observe(time.Since(start).Seconds())
		i.metrics.IndexerRequests.WithLabelValues(metadata.Label).Inc()
	}()

	ctx := r.Context()
	// supported query params: q, season, episode, page, filter_results
	q := r.URL.Query().Get("q")
	page := r.URL.Query().Get("page")

	// URL encode query param
	q = url.QueryEscape(q)
	url := metadata.URL
	if q != "" {
		url = fmt.Sprintf("%s%s%s", url, metadata.SearchURL, q)
	} else if page != "" {
		url = fmt.Sprintf(fmt.Sprintf("%s%s", url, metadata.PagePattern), page)
	}

	logging.InfoWithRequest(r).Str("target_url", url).Msg("Processing indexer request")
	resp, err := i.requester.GetDocument(ctx, url)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			logging.ErrorWithRequest(r).Err(err).Msg("Failed to encode error response")
		}
		i.metrics.IndexerErrors.WithLabelValues(metadata.Label).Inc()
		return
	}
	defer resp.Close()

	doc, err := goquery.NewDocumentFromReader(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		err = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		if err != nil {
			logging.ErrorWithRequest(r).Err(err).Msg("Failed to encode error response")
		}

		i.metrics.IndexerErrors.WithLabelValues(metadata.Label).Inc()
		return
	}

	links := parseRedeTorrentLinks(doc)

	// if no links were indexed, expire the document in cache
	if len(links) == 0 {
		_ = i.requester.ExpireDocument(ctx, url)
	}

	// extract each torrent link
	indexedTorrents := utils.ParallelFlatMap(links, func(link string) ([]schema.IndexedTorrent, error) {
		return getTorrentsRedeTorrent(ctx, i, link, url)
	})

	// Apply post-processors
	postProcessedTorrents := indexedTorrents
	for _, processor := range i.postProcessors {
		postProcessedTorrents = processor(i, r, postProcessedTorrents)
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(Response{
		Results:      postProcessedTorrents,
		Count:        len(postProcessedTorrents),
		IndexedCount: len(indexedTorrents),
	})
	if err != nil {
		logging.Error().Err(err).Msg("Failed to encode response")
	}
}

func getTorrentsRedeTorrent(ctx context.Context, i *Indexer, link, referer string) ([]schema.IndexedTorrent, error) {
	var indexedTorrents []schema.IndexedTorrent
	doc, err := getDocument(ctx, i, link, referer)
	if err != nil {
		return nil, err
	}

	post, err := parseRedeTorrentPost(doc)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, link)
	}
	title, year, date := post.title, post.year, post.date
	magnetLinks, audio, size, imdbLink := post.magnets, post.audio, post.sizes, post.imdb

	var chanIndexedTorrent = make(chan schema.IndexedTorrent)

	// for each magnet link, create a new indexed torrent
	for it, magnetLink := range magnetLinks {
		it := it
		go func(it int, magnetLink string) {
			magnet, err := magnet.ParseMagnetUri(magnetLink)
			if err != nil {
				logging.Error().Err(err).Str("magnet_link", magnetLink).Msg("Failed to parse magnet URI")
			}
			releaseTitle := magnet.DisplayName
			infoHash := magnet.InfoHash.String()
			trackers := magnet.Trackers
			magnetAudio := getAudioFromTitle(releaseTitle, audio)

			peer, seed, err := goscrape.GetLeechsAndSeeds(ctx, i.redis, i.metrics, infoHash, trackers)
			if err != nil {
				logging.Error().Err(err).Str("info_hash", infoHash).Msg("Failed to get leechers and seeders")
			}

			title := processTitle(title, magnetAudio)

			// if the number of sizes is equal to the number of magnets, then assign the size to each indexed torrent in order
			var mySize string
			if len(size) == len(magnetLinks) {
				mySize = size[it]
			}
			if mySize == "" {
				go func() {
					_, _ = i.magnetMetadataAPI.FetchMetadata(ctx, magnetLink)
				}()
			}

			ixt := schema.IndexedTorrent{
				Title:         releaseTitle,
				OriginalTitle: title,
				Details:       link,
				Year:          year,
				IMDB:          imdbLink,
				Audio:         magnetAudio,
				MagnetLink:    magnetLink,
				Date:          date,
				InfoHash:      infoHash,
				Trackers:      trackers,
				LeechCount:    peer,
				SeedCount:     seed,
				Size:          mySize,
			}
			chanIndexedTorrent <- ixt
		}(it, magnetLink)
	}

	for i := 0; i < len(magnetLinks); i++ {
		it := <-chanIndexedTorrent
		indexedTorrents = append(indexedTorrents, it)
	}

	return indexedTorrents, nil
}

type redeTorrentPost struct {
	title   string
	year    string
	date    time.Time
	imdb    string
	magnets []string
	audio   []schema.Audio
	sizes   []string
}

// parseRedeTorrentLinks extracts the post links from a listing or search page.
// It prefers the JSON-LD ItemList (more stable than CSS classes) and falls back
// to the Bootstrap cards (a.cover-link) and the legacy layout (.capa_lista).
func parseRedeTorrentLinks(doc *goquery.Document) []string {
	var links []string
	doc.Find("script[type=\"application/ld+json\"]").Each(func(_ int, s *goquery.Selection) {
		var data struct {
			MainEntity struct {
				Type            string `json:"@type"`
				ItemListElement []struct {
					URL  string `json:"url"`
					Item struct {
						URL string `json:"url"`
					} `json:"item"`
				} `json:"itemListElement"`
			} `json:"mainEntity"`
		}
		if err := json.Unmarshal([]byte(s.Text()), &data); err != nil || data.MainEntity.Type != "ItemList" {
			return
		}
		for _, el := range data.MainEntity.ItemListElement {
			if el.Item.URL != "" {
				links = append(links, el.Item.URL)
			} else if el.URL != "" {
				links = append(links, el.URL)
			}
		}
	})

	doc.Find("a.cover-link, .capa_lista > a").Each(func(_ int, s *goquery.Selection) {
		if link, ok := s.Attr("href"); ok && link != "" {
			links = append(links, link)
		}
	})

	return utils.StableUniq(links)
}

// title pattern: "Something Torrent (dddd) Dual Áudio Download" - extract "Something" and "dddd"
var redeTorrentTitleRe = regexp.MustCompile(`^(.*?)(?:\s+Torrent)?\s*\((\d{4})\)`)

// parseRedeTorrentPost extracts the metadata and magnet links from a post page.
func parseRedeTorrentPost(doc *goquery.Document) (redeTorrentPost, error) {
	var post redeTorrentPost

	h1 := strings.Join(strings.Fields(doc.Find("h1").First().Text()), " ")
	titleP := redeTorrentTitleRe.FindStringSubmatch(h1)
	if len(titleP) < 3 {
		return post, fmt.Errorf("could not extract title")
	}
	post.title = strings.TrimSpace(titleP[1])
	post.year = titleP[2]
	post.date = getPublishedDateFromMeta(doc)
	if post.date.IsZero() {
		if dt, ok := doc.Find("time[itemprop=\"dateCreated\"]").Attr("datetime"); ok {
			post.date, _ = time.Parse(time.RFC3339, dt)
		}
	}

	doc.Find("a[href^=\"magnet\"]").Each(func(_ int, s *goquery.Selection) {
		magnetLink, _ := s.Attr("href")
		post.magnets = append(post.magnets, magnetLink)
	})
	post.magnets = utils.StableUniq(post.magnets)

	// technical specs: <div class="spec-text"><small>Label</small><strong>Value</strong></div>
	// legacy layout: div#informacoes > p with "Label: Value<br>" lines
	var text strings.Builder
	doc.Find(".spec-text").Each(func(_ int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Find("small").Text())
		value := strings.TrimSpace(s.Find("strong").Text())
		switch {
		case strings.HasPrefix(label, "Idioma"):
			text.WriteString("Idioma: " + value + "\n")
		case strings.HasPrefix(label, "Tamanho"):
			text.WriteString("Tamanho: " + value + "\n")
		}
	})
	brRe := regexp.MustCompile(`<br\s*\/?>`)
	tagRe := regexp.MustCompile(`<[^>]*>`)
	doc.Find("div#informacoes > p").Each(func(_ int, s *goquery.Selection) {
		htmlContent, err := s.Html()
		if err != nil {
			return
		}
		htmlContent = strings.NewReplacer("\n", "", "\t", "").Replace(htmlContent)
		for _, line := range strings.Split(brRe.ReplaceAllString(htmlContent, "<br>"), "<br>") {
			text.WriteString(strings.TrimSpace(tagRe.ReplaceAllString(line, "")) + "\n")
		}
	})
	post.audio = findAudioFromText(text.String())
	if y := findYearFromText(text.String(), post.title); y != "" {
		post.year = y
	}
	post.sizes = utils.StableUniq(findSizesFromText(text.String()))

	// find any link from imdb
	doc.Find("a").Each(func(_ int, s *goquery.Selection) {
		link, _ := s.Attr("href")
		if imdbLink, err := getIMDBLink(link); err == nil {
			post.imdb = imdbLink
		}
	})

	return post, nil
}
