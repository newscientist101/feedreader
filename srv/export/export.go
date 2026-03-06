// Package export provides full JSON export/import for user data migration.
// This complements OPML by capturing all user configuration including
// scraper modules, exclusion rules, folder structure, and settings.
package export

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Version is the export format version.
const Version = 1

// Export represents a full user data export.
type Export struct {
	Version    int       `json:"version"`
	ExportedAt time.Time `json:"exported_at"`
	Folders    []Folder  `json:"folders"`
	Feeds      []Feed    `json:"feeds"`
	Scrapers   []Scraper `json:"scrapers"`
	Alerts     []Alert   `json:"alerts"`
	Settings   []Setting `json:"settings"`
}

// Folder represents a category/folder with its exclusion rules and settings.
type Folder struct {
	Name       string      `json:"name"`
	SortOrder  int64       `json:"sort_order"`
	Parent     string      `json:"parent,omitempty"`
	Exclusions []Exclusion `json:"exclusions,omitempty"`
	Settings   []Setting   `json:"settings,omitempty"`
}

// Exclusion represents a folder exclusion rule.
type Exclusion struct {
	Type    string `json:"type"` // "keyword" or "author"
	Pattern string `json:"pattern"`
	IsRegex bool   `json:"is_regex"`
}

// Feed represents a feed with all its configuration.
type Feed struct {
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	SiteURL        string   `json:"site_url,omitempty"`
	FeedType       string   `json:"feed_type"`
	FetchInterval  int64    `json:"fetch_interval_minutes"`
	ScraperModule  string   `json:"scraper_module,omitempty"`
	ScraperConfig  string   `json:"scraper_config,omitempty"`
	ContentFilters string   `json:"content_filters,omitempty"`
	SkipRetention  bool     `json:"skip_retention"`
	Folders        []string `json:"folders,omitempty"`
}

// Scraper represents a scraper module.
type Scraper struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Script      string `json:"script"`
	ScriptType  string `json:"script_type"`
	Enabled     bool   `json:"enabled"`
}

// Alert represents a news alert.
type Alert struct {
	Name       string `json:"name"`
	Pattern    string `json:"pattern"`
	IsRegex    bool   `json:"is_regex"`
	MatchField string `json:"match_field"`
}

// Setting represents a key-value user or category setting.
type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Encode writes the export as formatted JSON to the writer.
func Encode(w io.Writer, exp *Export) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(exp)
}

// Decode reads an export from the reader.
func Decode(r io.Reader) (*Export, error) {
	var exp Export
	if err := json.NewDecoder(r).Decode(&exp); err != nil {
		return nil, fmt.Errorf("decode export: %w", err)
	}
	if exp.Version == 0 {
		return nil, fmt.Errorf("invalid export: missing version field")
	}
	if exp.Version > Version {
		return nil, fmt.Errorf("unsupported export version %d (max supported: %d)", exp.Version, Version)
	}
	return &exp, nil
}

// ImportResult tracks what was created vs skipped during import.
type ImportResult struct {
	FeedsCreated    int `json:"feeds_created"`
	FeedsSkipped    int `json:"feeds_skipped"`
	FoldersCreated  int `json:"folders_created"`
	FoldersSkipped  int `json:"folders_skipped"`
	ScrapersCreated int `json:"scrapers_created"`
	ScrapersSkipped int `json:"scrapers_skipped"`
	AlertsCreated   int `json:"alerts_created"`
	AlertsSkipped   int `json:"alerts_skipped"`
	SettingsApplied int `json:"settings_applied"`
}
