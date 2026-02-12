package scrapers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSONScraperConfig defines how to extract items from a JSON API
type JSONScraperConfig struct {
	Type string `json:"type"` // must be "json"
	// ItemsPath is a dot-separated path to the array of items (e.g. "logs" or "data.items")
	ItemsPath string `json:"itemsPath"`
	// Field paths (dot-separated keys within each item)
	TitlePath   string `json:"titlePath"`
	URLPath     string `json:"urlPath"`
	AuthorPath  string `json:"authorPath"`
	ContentPath string `json:"contentPath"`
	SummaryPath string `json:"summaryPath"`
	ImagePath   string `json:"imagePath"`
	DatePath    string `json:"datePath"`
	GUIDPath    string `json:"guidPath"`
	// BaseURL for constructing full URLs
	BaseURL string `json:"baseUrl"`
	// ConsolidateDuplicates merges consecutive items with the same title
	ConsolidateDuplicates bool `json:"consolidateDuplicates"`
}

func (r *Runner) runJSONScraper(config JSONScraperConfig, content, pageURL string) ([]FeedItem, error) {
	var raw any
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse JSON content: %w", err)
	}

	// Navigate to items array
	itemsRaw, err := navigatePath(raw, config.ItemsPath)
	if err != nil {
		return nil, fmt.Errorf("navigate to items: %w", err)
	}

	items, ok := itemsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("itemsPath did not resolve to an array")
	}

	var feedItems []FeedItem
	for i, item := range items {
		fi := FeedItem{}

		fi.Title = getStringPath(item, config.TitlePath)
		fi.URL = getStringPath(item, config.URLPath)
		if fi.URL != "" {
			fi.URL = resolveURL(config.BaseURL, pageURL, fi.URL)
		}
		fi.Author = getStringPath(item, config.AuthorPath)
		fi.Content = getStringPath(item, config.ContentPath)
		fi.Summary = getStringPath(item, config.SummaryPath)
		fi.ImageURL = getStringPath(item, config.ImagePath)
		if fi.ImageURL != "" {
			fi.ImageURL = resolveURL(config.BaseURL, pageURL, fi.ImageURL)
		}

		// Parse date
		if dateStr := getStringPath(item, config.DatePath); dateStr != "" {
			if t, err := parseFlexibleDate(dateStr); err == nil {
				fi.PublishedAt = &t
			}
		}

		// GUID
		fi.GUID = getStringPath(item, config.GUIDPath)
		if fi.GUID == "" && fi.URL != "" {
			fi.GUID = fi.URL
		} else if fi.GUID == "" {
			dateStr := getStringPath(item, config.DatePath)
			if fi.Title != "" && dateStr != "" {
				fi.GUID = fmt.Sprintf("%s-%s", fi.Title, dateStr)
			} else if fi.Title != "" {
				fi.GUID = fmt.Sprintf("%s-%d", fi.Title, i)
			} else {
				continue
			}
		}

		if fi.Title != "" || fi.Summary != "" || fi.Content != "" {
			feedItems = append(feedItems, fi)
		}
	}

	if config.ConsolidateDuplicates {
		feedItems = consolidateItems(feedItems)
	}

	return feedItems, nil
}

// navigatePath follows a dot-separated path through a JSON structure
func navigatePath(data any, path string) (any, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected object at %q, got %T", part, current)
		}
		current, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("key %q not found", part)
		}
	}
	return current, nil
}

// getStringPath extracts a string value from a nested JSON structure
func getStringPath(data any, path string) string {
	if path == "" {
		return ""
	}
	val, err := navigatePath(data, path)
	if err != nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%v", v)
	case bool:
		return fmt.Sprintf("%v", v)
	case nil:
		return ""
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// consolidateItems merges consecutive items with the same title.
// Combined descriptions are sentence-deduped (up to 50 unique sentences).
func consolidateItems(items []FeedItem) []FeedItem {
	if len(items) == 0 {
		return items
	}

	var result []FeedItem
	i := 0
	for i < len(items) {
		// Find run of consecutive items with the same title
		j := i + 1
		for j < len(items) && items[j].Title == items[i].Title {
			j++
		}

		if j-i == 1 {
			// No duplicates, keep as-is
			result = append(result, items[i])
		} else {
			// Merge consecutive duplicates
			merged := items[i] // Start with the first item
			count := j - i
			merged.Title = fmt.Sprintf("%s - Combined(%d)", items[i].Title, count)

			// Collect and dedupe sentences from content/summary
			seen := make(map[string]bool)
			var uniqueSentences []string

			for k := i; k < j; k++ {
				text := items[k].Content
				if text == "" {
					text = items[k].Summary
				}
				for _, sentence := range splitSentences(text) {
					norm := strings.TrimSpace(sentence)
					if norm == "" || seen[norm] {
						continue
					}
					seen[norm] = true
					uniqueSentences = append(uniqueSentences, norm)
					if len(uniqueSentences) >= 50 {
						break
					}
				}
				if len(uniqueSentences) >= 50 {
					break
				}
			}

			combinedText := strings.Join(uniqueSentences, "\n\n")
			if merged.Content != "" {
				merged.Content = combinedText
			} else {
				merged.Summary = combinedText
			}

			// Use the earliest date
			for k := i; k < j; k++ {
				if items[k].PublishedAt != nil {
					if merged.PublishedAt == nil || items[k].PublishedAt.Before(*merged.PublishedAt) {
						merged.PublishedAt = items[k].PublishedAt
					}
				}
			}

			// GUID should be unique for the combined item
			merged.GUID = fmt.Sprintf("%s-combined-%d", merged.GUID, count)

			result = append(result, merged)
		}
		i = j
	}

	return result
}

// splitSentences splits text into sentences on common delimiters
func splitSentences(text string) []string {
	if text == "" {
		return nil
	}

	// Split on double newlines first (paragraph boundaries)
	paragraphs := strings.Split(text, "\n\n")
	var sentences []string
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		// Split on single newlines within paragraphs
		lines := strings.Split(para, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				sentences = append(sentences, line)
			}
		}
	}
	return sentences
}
