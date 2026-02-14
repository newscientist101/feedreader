package srv

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ContentFilter represents a pattern to remove from article content
type ContentFilter struct {
	Selector string `json:"selector"` // CSS selector to remove
}

// ApplyContentFilters removes elements matching CSS selectors from content
func ApplyContentFilters(content string, filtersJSON *string) string {
	if filtersJSON == nil || *filtersJSON == "" {
		return content
	}

	var filters []ContentFilter
	if err := json.Unmarshal([]byte(*filtersJSON), &filters); err != nil {
		return content
	}

	if len(filters) == 0 {
		return content
	}

	// Wrap content in a root element for parsing
	wrapped := "<div id=\"feedreader-root\">" + content + "</div>"
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(wrapped))
	if err != nil {
		return content
	}

	// Remove elements matching each selector
	for _, filter := range filters {
		if filter.Selector == "" {
			continue
		}
		doc.Find(filter.Selector).Remove()
	}

	// Extract the inner HTML of our wrapper
	result, err := doc.Find("#feedreader-root").Html()
	if err != nil {
		return content
	}

	return strings.TrimSpace(result)
}

// SerializeFilters converts a list of selectors to JSON
func SerializeFilters(selectors []string) string {
	if len(selectors) == 0 {
		return ""
	}
	filters := make([]ContentFilter, len(selectors))
	for i, s := range selectors {
		filters[i] = ContentFilter{Selector: s}
	}
	b, _ := json.Marshal(filters)
	return string(b)
}

// ParseFilters extracts selectors from JSON
func ParseFilters(filtersJSON string) []string {
	if filtersJSON == "" {
		return nil
	}
	var filters []ContentFilter
	if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
		return nil
	}
	selectors := make([]string, len(filters))
	for i, f := range filters {
		selectors[i] = f.Selector
	}
	return selectors
}

// ValidateSelector checks if a CSS selector is valid
func ValidateSelector(selector string) bool {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader([]byte("<div></div>")))
	if err != nil {
		return false
	}
	// goquery will panic on invalid selectors, so we need to recover
	defer func() { _ = recover() }()
	doc.Find(selector)
	return true
}
