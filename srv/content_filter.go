package srv

import (
	"encoding/json"
	"regexp"
	"strings"
)

// ContentFilter represents a pattern to remove from article content
type ContentFilter struct {
	Pattern string `json:"pattern"`
	IsRegex bool   `json:"is_regex"`
}

// ApplyContentFilters removes matching patterns from content
func ApplyContentFilters(content string, filtersJSON *string) string {
	if filtersJSON == nil || *filtersJSON == "" {
		return content
	}

	var filters []ContentFilter
	if err := json.Unmarshal([]byte(*filtersJSON), &filters); err != nil {
		return content
	}

	result := content
	for _, filter := range filters {
		if filter.Pattern == "" {
			continue
		}

		if filter.IsRegex {
			// Use regex replacement
			re, err := regexp.Compile("(?s)" + filter.Pattern) // (?s) makes . match newlines
			if err != nil {
				continue
			}
			result = re.ReplaceAllString(result, "")
		} else {
			// Plain string replacement
			result = strings.ReplaceAll(result, filter.Pattern, "")
		}
	}

	return result
}
