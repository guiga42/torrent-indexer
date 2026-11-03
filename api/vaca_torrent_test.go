package handler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func Test_VacaTorrentListing(t *testing.T) {
	doc := loadFixture(t, "vaca_torrent_list.html")
	var links []string
	doc.Find(".i-tem_ht").Each(func(_ int, s *goquery.Selection) {
		if link, ok := s.Find("a").Attr("href"); ok {
			links = append(links, link)
		}
	})
	if len(links) == 0 {
		t.Fatal("no links found")
	}
	for _, l := range links {
		if !strings.HasPrefix(l, "https://vaqueirofilmes.com/") {
			t.Errorf("unexpected link %q", l)
		}
	}
}

func Test_VacaTorrentPostHasProtectedLinks(t *testing.T) {
	doc := loadFixture(t, "vaca_torrent_post.html")
	if title := strings.TrimSpace(doc.Find(".custom-main-title").First().Text()); title == "" {
		t.Error("title not found")
	}
	if n := doc.Find("a[href^=\"magnet\"]").Length(); n != 0 {
		t.Errorf("expected no direct magnets, got %d", n)
	}
	if n := doc.Find("a[href*=\"/movie-links/\"]").Length(); n != 1 {
		t.Errorf("expected 1 movie-links card, got %d", n)
	}
}
