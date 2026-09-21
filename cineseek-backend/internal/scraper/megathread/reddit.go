package megathread

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"

	"cineseek-backend/internal/domain"
)

type RedditScraper struct {
	client *http.Client
}

func NewRedditScraper(client *http.Client) *RedditScraper {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &RedditScraper{client: client}
}

// FetchSectionWikiLinks parses specific target sections from public Reddit wiki HTML pages.
func (r *RedditScraper) FetchSectionWikiLinks(ctx context.Context, wikiURL string, targetSections []string) ([]domain.Stream, error) {
	cleanURL := strings.Split(wikiURL, "?")[0]
	cleanURL = strings.Split(cleanURL, "#")[0]
	cleanURL = strings.TrimSuffix(cleanURL, "/")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("reddit wiki HTML returned status: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html document: %w", err)
	}

	var streams []domain.Stream
	seen := make(map[string]bool)

	// Clean up targets for flexible matching
	cleanTargets := make([]string, len(targetSections))
	for i, t := range targetSections {
		cleanTargets[i] = strings.ToLower(cleanSectionText(t))
	}

	// Find all headings and potential section container elements
	doc.Find("h1, h2, h3, h4, p, div").Each(func(i int, s *goquery.Selection) {
		rawText := strings.ToLower(cleanSectionText(s.Text()))

		// Check if this element represents one of our target section headers
		isTargetSection := false
		for _, target := range cleanTargets {
			if target != "" && strings.Contains(rawText, target) {
				isTargetSection = true
				break
			}
		}

		if !isTargetSection {
			return
		}

		// Look for <a> tags inside the element itself, its immediate parent, or subsequent siblings
		container := s.Parent()
		if container.Length() == 0 {
			container = s
		}

		container.Find("a").Each(func(j int, anchor *goquery.Selection) {
			rawURL, exists := anchor.Attr("href")
			if !exists {
				return
			}

			rawURL = strings.TrimSpace(rawURL)
			title := stripHTMLTags(strings.TrimSpace(anchor.Text()))

			// Filter out internal Reddit links, empty links, or duplicates
			if strings.Contains(rawURL, "reddit.com") ||
				strings.Contains(rawURL, "redd.it") ||
				strings.HasPrefix(rawURL, "#") ||
				title == "" ||
				seen[rawURL] {
				return
			}

			// Filter utility/meta sites
			if strings.Contains(rawURL, "urlvoid.com") ||
				strings.Contains(rawURL, "virustotal.com") ||
				strings.Contains(rawURL, "github.com") ||
				strings.Contains(rawURL, "gitlab.com") {
				return
			}

			seen[rawURL] = true

			stream := domain.Stream{
				Provider: "reddit_megathread",
				Title:    title,
				URL:      rawURL,
			}
			stream.Type = stream.InferType()

			streams = append(streams, stream)
		})
	})

	return streams, nil
}

// Clean emojis, arrows, and non-alphanumeric characters for flexible matching
func cleanSectionText(input string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9\s/]+`)
	return re.ReplaceAllString(input, "")
}

func stripHTMLTags(input string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	return re.ReplaceAllString(input, "")
}
