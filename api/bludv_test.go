package handler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func Test_BluDVSearchListing(t *testing.T) {
	doc := loadFixture(t, "bludv_search.html")
	var links []string
	doc.Find(".post").Each(func(_ int, s *goquery.Selection) {
		if link, ok := s.Find("div.title > a").Attr("href"); ok {
			links = append(links, link)
		}
	})
	if len(links) != 7 {
		t.Fatalf("got %d links, want 7: %v", len(links), links)
	}
	for _, l := range links {
		if !strings.HasPrefix(l, "https://bludvfilmes2.xyz/") {
			t.Errorf("unexpected link %q", l)
		}
	}
}

func Test_countBluDVGateLinks(t *testing.T) {
	doc := loadFixture(t, "bludv_post.html")
	content := doc.Find(".post div.content")
	if content.Length() == 0 {
		t.Fatal("div.content not found")
	}
	if n := content.Find("a[href^=\"magnet\"]").Length(); n != 0 {
		t.Errorf("expected no direct magnets, got %d", n)
	}
	if n := countBluDVGateLinks(content); n != 1 {
		t.Errorf("countBluDVGateLinks() = %d, want 1", n)
	}
}
