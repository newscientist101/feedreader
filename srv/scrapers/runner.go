package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Runner executes scraper scripts
type Runner struct{}

// NewRunner creates a new scraper runner
func NewRunner() *Runner {
	return &Runner{}
}

// ScriptResult is what the script should return
type ScriptResult struct {
	Items []ScriptItem `json:"items"`
}

type ScriptItem struct {
	GUID        string `json:"guid"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	Content     string `json:"content"`
	Summary     string `json:"summary"`
	ImageURL    string `json:"imageUrl"`
	PublishedAt string `json:"publishedAt"`
}

// FeedItem represents a single item extracted by the scraper
type FeedItem struct {
	GUID        string
	Title       string
	URL         string
	Author      string
	Content     string
	Summary     string
	ImageURL    string
	PublishedAt *time.Time
}

// Run executes a scraper script against HTML content
// Scripts are simple JSON-based selector configs for now
func (r *Runner) Run(ctx context.Context, script, html, pageURL, config string) ([]FeedItem, error) {
	// Parse the script as a scraper config
	var scraperConfig ScraperConfig
	if err := json.Unmarshal([]byte(script), &scraperConfig); err != nil {
		return nil, fmt.Errorf("parse scraper config: %w", err)
	}

	return r.runConfigScraper(scraperConfig, html, pageURL)
}

// ScraperConfig defines how to extract items from a page
type ScraperConfig struct {
	// ItemSelector is a regex pattern to match item blocks
	ItemPattern string `json:"itemPattern"`
	// Field extractors (regex patterns with capture groups)
	TitlePattern    string `json:"titlePattern"`
	URLPattern      string `json:"urlPattern"`
	AuthorPattern   string `json:"authorPattern"`
	SummaryPattern  string `json:"summaryPattern"`
	ImagePattern    string `json:"imagePattern"`
	DatePattern     string `json:"datePattern"`
	// BaseURL for relative URLs
	BaseURL string `json:"baseUrl"`
}

func (r *Runner) runConfigScraper(config ScraperConfig, html, pageURL string) ([]FeedItem, error) {
	if config.ItemPattern == "" {
		return nil, fmt.Errorf("itemPattern is required")
	}

	itemRe, err := regexp.Compile("(?s)" + config.ItemPattern)
	if err != nil {
		return nil, fmt.Errorf("invalid itemPattern: %w", err)
	}

	matches := itemRe.FindAllString(html, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	var items []FeedItem
	for i, match := range matches {
		item := FeedItem{}

		// Extract title
		if config.TitlePattern != "" {
			if re, err := regexp.Compile(config.TitlePattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					item.Title = cleanHTML(m[1])
				}
			}
		}

		// Extract URL
		if config.URLPattern != "" {
			if re, err := regexp.Compile(config.URLPattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					item.URL = resolveURL(config.BaseURL, pageURL, m[1])
				}
			}
		}

		// Extract author
		if config.AuthorPattern != "" {
			if re, err := regexp.Compile(config.AuthorPattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					item.Author = cleanHTML(m[1])
				}
			}
		}

		// Extract summary
		if config.SummaryPattern != "" {
			if re, err := regexp.Compile(config.SummaryPattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					item.Summary = cleanHTML(m[1])
				}
			}
		}

		// Extract image
		if config.ImagePattern != "" {
			if re, err := regexp.Compile(config.ImagePattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					item.ImageURL = resolveURL(config.BaseURL, pageURL, m[1])
				}
			}
		}

		// Extract date
		if config.DatePattern != "" {
			if re, err := regexp.Compile(config.DatePattern); err == nil {
				if m := re.FindStringSubmatch(match); len(m) > 1 {
					if t, err := parseFlexibleDate(m[1]); err == nil {
						item.PublishedAt = &t
					}
				}
			}
		}

		// Generate GUID if not found
		if item.URL != "" {
			item.GUID = item.URL
		} else if item.Title != "" {
			item.GUID = fmt.Sprintf("%s-%d", item.Title, i)
		} else {
			continue // Skip items without identifiable content
		}

		if item.Title != "" || item.Summary != "" {
			items = append(items, item)
		}
	}

	return items, nil
}

func cleanHTML(s string) string {
	// Remove HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	s = re.ReplaceAllString(s, "")
	// Decode common entities
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	return strings.TrimSpace(s)
}

func resolveURL(baseURL, pageURL, href string) string {
	href = strings.TrimSpace(href)
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	base := baseURL
	if base == "" {
		base = pageURL
	}
	if base == "" {
		return href
	}
	// Simple URL resolution
	if strings.HasPrefix(href, "//") {
		if strings.HasPrefix(base, "https:") {
			return "https:" + href
		}
		return "http:" + href
	}
	if strings.HasPrefix(href, "/") {
		// Extract origin from base
		parts := strings.SplitN(base, "://", 2)
		if len(parts) == 2 {
			host := strings.SplitN(parts[1], "/", 2)[0]
			return parts[0] + "://" + host + href
		}
	}
	return href
}

func parseFlexibleDate(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
		"Jan 2, 2006",
		"January 2, 2006",
		"02 Jan 2006",
		"2 Jan 2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(s)); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}
