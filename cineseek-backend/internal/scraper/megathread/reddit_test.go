package megathread

import (
	"context"
	"testing"
	"time"
)

func TestLiveRedditWikiFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}

	scraper := NewRedditScraper(nil)
	wikiURL := "https://www.reddit.com/r/Piracy/wiki/megathread/movies_and_tv/"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	streams, err := scraper.FetchWikiLinks(ctx, wikiURL, 5)
	if err != nil {
		t.Fatalf("Live fetch failed: %v", err)
	}

	if len(streams) == 0 {
		t.Fatal("Expected extracted streams, got 0")
	}

	for i, s := range streams {
		t.Logf("[%d] %s -> %s (%s)", i+1, s.Title, s.URL, s.Type)
	}
}
