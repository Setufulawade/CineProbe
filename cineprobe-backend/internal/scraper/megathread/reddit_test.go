package megathread

import (
	"context"
	"testing"
	"time"
)

func TestLiveRedditWikiFetchAllSections(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}

	scraper := NewRedditScraper(nil)
	wikiURL := "https://www.reddit.com/r/Piracy/wiki/megathread/movies_and_tv/"

	// Target sections specified in prompt
	targetSections := []string{
		"East / South Asian Drama",
		"Streaming Sites",
		"Torrent Sites",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	streams, err := scraper.FetchSectionWikiLinks(ctx, wikiURL, targetSections)
	if err != nil {
		t.Fatalf("Live fetch failed: %v", err)
	}

	if len(streams) == 0 {
		t.Fatal("Expected extracted streams, got 0")
	}

	t.Logf("Total streams extracted across sections: %d\n", len(streams))

	for i, s := range streams {
		t.Logf("[%d] %s -> %s (%s)", i+1, s.Title, s.URL, s.Type)
	}
}
