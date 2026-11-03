package handler

import (
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func loadFixture(t *testing.T, name string) *goquery.Document {
	t.Helper()
	f, err := os.Open("testdata/" + name)
	if err != nil {
		t.Fatalf("failed to open fixture %s: %v", name, err)
	}
	defer f.Close()
	doc, err := goquery.NewDocumentFromReader(f)
	if err != nil {
		t.Fatalf("failed to parse fixture %s: %v", name, err)
	}
	return doc
}

func Test_parseRedeTorrentLinks(t *testing.T) {
	tests := []struct {
		fixture  string
		want     int
		contains string
	}{
		{"rede_torrent_list.html", 20, "https://redestorrents.com/territorio-hostil-2025-torrent-dublado-dual-audio-legendado-download/"},
		{"rede_torrent_search.html", 9, "https://redestorrents.com/duna-parte-2-torrent-dublado-dual-audio-legendado-download/"},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			links := parseRedeTorrentLinks(loadFixture(t, tt.fixture))
			if len(links) != tt.want {
				t.Errorf("got %d links, want %d: %v", len(links), tt.want, links)
			}
			found := false
			for _, l := range links {
				if !strings.HasPrefix(l, "https://redestorrents.com/") {
					t.Errorf("unexpected link %q", l)
				}
				found = found || l == tt.contains
			}
			if !found {
				t.Errorf("links do not contain %q", tt.contains)
			}
		})
	}
}

func Test_parseRedeTorrentPost(t *testing.T) {
	post, err := parseRedeTorrentPost(loadFixture(t, "rede_torrent_post.html"))
	if err != nil {
		t.Fatalf("parseRedeTorrentPost() error = %v", err)
	}
	if post.title != "Duna - Parte 2" {
		t.Errorf("title = %q, want %q", post.title, "Duna - Parte 2")
	}
	if post.year != "2024" {
		t.Errorf("year = %q, want %q", post.year, "2024")
	}
	if post.imdb != "https://www.imdb.com/title/tt15239678/" {
		t.Errorf("imdb = %q", post.imdb)
	}
	if post.date.IsZero() {
		t.Errorf("date should not be zero")
	}
	if len(post.magnets) != 7 {
		t.Fatalf("got %d magnets, want 7", len(post.magnets))
	}
	for _, m := range post.magnets {
		if !strings.HasPrefix(m, "magnet:?xt=urn:btih:") {
			t.Errorf("invalid magnet %q", m)
		}
	}
	if len(post.audio) != 2 {
		t.Errorf("audio = %v, want 2 languages", post.audio)
	}
	if strings.Join(post.sizes, ",") != "1.49 GB,3.50 GB" {
		t.Errorf("sizes = %v", post.sizes)
	}
}

func Test_parseRedeTorrentLinks_JSONLDOnly(t *testing.T) {
	doc := loadFixture(t, "rede_torrent_list.html")
	doc.Find("a.cover-link").Remove()
	if links := parseRedeTorrentLinks(doc); len(links) != 20 {
		t.Errorf("got %d links from JSON-LD, want 20", len(links))
	}
}
