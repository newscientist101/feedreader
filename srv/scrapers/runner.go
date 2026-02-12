package scrapers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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

// Run executes a scraper script against fetched content
func (r *Runner) Run(ctx context.Context, script, content, pageURL, config string) ([]FeedItem, error) {
	// Detect config type
	var probe struct {
		Type string `json:"type"`
	}
	json.Unmarshal([]byte(script), &probe)

	if probe.Type == "json" {
		var jsonConfig JSONScraperConfig
		if err := json.Unmarshal([]byte(script), &jsonConfig); err != nil {
			return nil, fmt.Errorf("parse json scraper config: %w", err)
		}
		return r.runJSONScraper(jsonConfig, content, pageURL)
	}

	// Default: CSS selector scraper
	var scraperConfig ScraperConfig
	if err := json.Unmarshal([]byte(script), &scraperConfig); err != nil {
		return nil, fmt.Errorf("parse scraper config: %w", err)
	}
	return r.runConfigScraper(scraperConfig, content, pageURL)
}

// ScraperConfig defines how to extract items from a page using CSS selectors
type ScraperConfig struct {
	// ItemSelector is a CSS selector to match item containers
	ItemSelector string `json:"itemSelector"`
	// Field extractors (CSS selectors)
	TitleSelector   string `json:"titleSelector"`
	URLSelector     string `json:"urlSelector"`
	URLAttr         string `json:"urlAttr"`         // attribute to get URL from (default: "href")
	AuthorSelector  string `json:"authorSelector"`
	SummarySelector string `json:"summarySelector"`
	ImageSelector   string `json:"imageSelector"`
	ImageAttr       string `json:"imageAttr"`       // attribute to get image from (default: "src")
	DateSelector    string `json:"dateSelector"`
	DateAttr        string `json:"dateAttr"`        // attribute to get date from (default: text content)
	// BaseURL for relative URLs
	BaseURL string `json:"baseUrl"`
}

func (r *Runner) runConfigScraper(config ScraperConfig, html, pageURL string) ([]FeedItem, error) {
	if config.ItemSelector == "" {
		return nil, fmt.Errorf("itemSelector is required")
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	if config.URLAttr == "" {
		config.URLAttr = "href"
	}
	if config.ImageAttr == "" {
		config.ImageAttr = "src"
	}

	var items []FeedItem
	doc.Find(config.ItemSelector).Each(func(i int, s *goquery.Selection) {
		item := FeedItem{}

		// Extract title
		if config.TitleSelector != "" {
			item.Title = strings.TrimSpace(s.Find(config.TitleSelector).First().Text())
		}

		// Extract URL
		if config.URLSelector != "" {
			if val, exists := s.Find(config.URLSelector).First().Attr(config.URLAttr); exists {
				item.URL = resolveURL(config.BaseURL, pageURL, strings.TrimSpace(val))
			}
		}

		// Extract author
		if config.AuthorSelector != "" {
			item.Author = strings.TrimSpace(s.Find(config.AuthorSelector).First().Text())
		}

		// Extract summary
		if config.SummarySelector != "" {
			item.Summary = strings.TrimSpace(s.Find(config.SummarySelector).First().Text())
		}

		// Extract image
		if config.ImageSelector != "" {
			if val, exists := s.Find(config.ImageSelector).First().Attr(config.ImageAttr); exists {
				item.ImageURL = resolveURL(config.BaseURL, pageURL, strings.TrimSpace(val))
			}
		}

		// Extract date
		if config.DateSelector != "" {
			var dateStr string
			sel := s.Find(config.DateSelector).First()
			if config.DateAttr != "" {
				dateStr, _ = sel.Attr(config.DateAttr)
			} else {
				dateStr = sel.Text()
			}
			if t, err := parseFlexibleDate(strings.TrimSpace(dateStr)); err == nil {
				item.PublishedAt = &t
			}
		}

		// Generate GUID
		if item.URL != "" {
			item.GUID = item.URL
		} else if item.Title != "" {
			item.GUID = fmt.Sprintf("%s-%d", item.Title, i)
		} else {
			return // Skip items without identifiable content
		}

		if item.Title != "" || item.Summary != "" {
			items = append(items, item)
		}
	})

	return items, nil
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
	if strings.HasPrefix(href, "//") {
		if strings.HasPrefix(base, "https:") {
			return "https:" + href
		}
		return "http:" + href
	}
	if strings.HasPrefix(href, "/") {
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
