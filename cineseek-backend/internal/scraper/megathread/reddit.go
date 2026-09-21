package megathread

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"cineseek-backend/internal/domain"
)

// Regex to extract href links and link titles from standard HTML <a> tags
var htmlLinkRegex = regexp.MustCompile(`<a\s+[^>]*href=["'](https?://[^"']+)["'][^>]*>(.*?)</a>`)

type RedditScraper struct {
	client *http.Client
}

func NewRedditScraper(client *http.Client) *RedditScraper {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &RedditScraper{client: client}
}

// FetchWikiLinks parses public Reddit wiki HTML pages without triggering JSON 403 blocks.
func (r *RedditScraper) FetchWikiLinks(ctx context.Context, wikiURL string, limit int) ([]domain.Stream, error) {
	// Clean query parameters and anchors
	cleanURL := strings.Split(wikiURL, "?")[0]
	cleanURL = strings.Split(cleanURL, "#")[0]
	cleanURL = strings.TrimSuffix(cleanURL, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit wiki HTML returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}

	htmlContent := string(bodyBytes)
	matches := htmlLinkRegex.FindAllStringSubmatch(htmlContent, -1)

	var streams []domain.Stream
	seen := make(map[string]bool)

	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		rawURL := strings.TrimSpace(match[1])
		title := stripHTMLTags(strings.TrimSpace(match[2]))

		// Skip internal Reddit, anchor, or empty links
		if strings.Contains(rawURL, "reddit.com") || strings.Contains(rawURL, "redd.it") || title == "" || seen[rawURL] {
			continue
		}

		if strings.Contains(rawURL, "urlvoid.com") ||
			strings.Contains(rawURL, "virustotal.com") ||
			strings.Contains(rawURL, "github.com") ||
			strings.Contains(rawURL, "gitlab.com") {
			continue
		}

		seen[rawURL] = true

		stream := domain.Stream{
			Provider: "reddit_megathread",
			Title:    title,
			URL:      rawURL,
		}
		stream.Type = stream.InferType()

		streams = append(streams, stream)

		if limit > 0 && len(streams) >= limit {
			break
		}
	}

	return streams, nil
}

// Helper to strip inner HTML tags from link title text
func stripHTMLTags(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(input, "")
}
