package handler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func Test_StarckFilmesListing(t *testing.T) {
	doc := loadFixture(t, "starck_filmes_list.html")
	var links []string
	doc.Find(".item").Each(func(_ int, s *goquery.Selection) {
		if link, ok := s.Find("div.sub-item > a").Attr("href"); ok {
			links = append(links, link)
		}
	})
	if len(links) == 0 {
		t.Fatal("no links found")
	}
	for _, l := range links {
		if !strings.Contains(l, "/catalog/") {
			t.Errorf("unexpected link %q", l)
		}
	}
}

func Test_extractStarckMagnets(t *testing.T) {
	doc := loadFixture(t, "starck_filmes_post.html")
	post := doc.Find(".post")
	if title := post.Find(".capa .post-description > h2").Text(); title != "Código: Vingança" {
		t.Errorf("title = %q", title)
	}
	magnets := extractStarckMagnets(post.Find(".post-buttons"))
	if len(magnets) != 2 {
		t.Fatalf("got %d magnets, want 2: %v", len(magnets), magnets)
	}
	for _, m := range magnets {
		if !strings.HasPrefix(m, "magnet:?xt=urn:btih:") {
			t.Errorf("invalid magnet %q", m)
		}
	}
	if !strings.Contains(magnets[0], "1080p") || !strings.Contains(magnets[1], "2160p") {
		t.Errorf("unexpected magnet order: %v", magnets)
	}
}
