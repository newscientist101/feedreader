package export

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecode(t *testing.T) {
	exp := &Export{
		Version:    Version,
		ExportedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Folders: []Folder{
			{
				Name:      "Tech",
				SortOrder: 1,
				Exclusions: []Exclusion{
					{Type: "keyword", Pattern: "crypto", IsRegex: false},
				},
				Settings: []Setting{
					{Key: "view_mode", Value: "list"},
				},
			},
			{
				Name:      "Sub-Tech",
				SortOrder: 0,
				Parent:    "Tech",
			},
		},
		Feeds: []Feed{
			{
				Name:           "Hacker News",
				URL:            "https://news.ycombinator.com/rss",
				FeedType:       "rss",
				FetchInterval:  30,
				ContentFilters: `[{"type":"remove","selector":".ad"}]`,
				Folders:        []string{"Tech"},
			},
			{
				Name:          "My Scraper",
				URL:           "https://example.com/page",
				FeedType:      "scraper",
				FetchInterval: 60,
				ScraperModule: "example-scraper",
				ScraperConfig: `{"url":"https://example.com"}`,
				SkipRetention: true,
			},
		},
		Scrapers: []Scraper{
			{
				Name:        "example-scraper",
				Description: "Scrapes example.com",
				Script:      "document.querySelector('.content')",
				ScriptType:  "javascript",
				Enabled:     true,
			},
		},
		Alerts: []Alert{
			{
				Name:       "Go releases",
				Pattern:    "go 1\\.\\.+",
				IsRegex:    true,
				MatchField: "title",
			},
		},
		Settings: []Setting{
			{Key: "theme", Value: "dark"},
			{Key: "articles_per_page", Value: "50"},
		},
	}

	// Encode
	var buf bytes.Buffer
	err := Encode(&buf, exp)
	require.NoError(t, err)

	encoded := buf.String()
	assert.Contains(t, encoded, `"version": 1`)
	assert.Contains(t, encoded, `"Hacker News"`)
	assert.Contains(t, encoded, `"example-scraper"`)

	// Decode
	decoded, err := Decode(strings.NewReader(encoded))
	require.NoError(t, err)

	assert.Equal(t, exp.Version, decoded.Version)
	assert.Equal(t, len(exp.Folders), len(decoded.Folders))
	assert.Equal(t, len(exp.Feeds), len(decoded.Feeds))
	assert.Equal(t, len(exp.Scrapers), len(decoded.Scrapers))
	assert.Equal(t, len(exp.Alerts), len(decoded.Alerts))
	assert.Equal(t, len(exp.Settings), len(decoded.Settings))

	// Check folder details
	assert.Equal(t, "Tech", decoded.Folders[0].Name)
	assert.Equal(t, int64(1), decoded.Folders[0].SortOrder)
	assert.Equal(t, "Sub-Tech", decoded.Folders[1].Name)
	assert.Equal(t, "Tech", decoded.Folders[1].Parent)
	assert.Len(t, decoded.Folders[0].Exclusions, 1)
	assert.Equal(t, "keyword", decoded.Folders[0].Exclusions[0].Type)
	assert.Len(t, decoded.Folders[0].Settings, 1)

	// Check feed details
	assert.Equal(t, "Hacker News", decoded.Feeds[0].Name)
	assert.Equal(t, []string{"Tech"}, decoded.Feeds[0].Folders)
	assert.Equal(t, "scraper", decoded.Feeds[1].FeedType)
	assert.Equal(t, "example-scraper", decoded.Feeds[1].ScraperModule)
	assert.True(t, decoded.Feeds[1].SkipRetention)

	// Check alert details
	assert.Equal(t, "Go releases", decoded.Alerts[0].Name)
	assert.True(t, decoded.Alerts[0].IsRegex)
	assert.Equal(t, "title", decoded.Alerts[0].MatchField)
}

func TestDecodeInvalidJSON(t *testing.T) {
	_, err := Decode(strings.NewReader("not json"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode export")
}

func TestDecodeMissingVersion(t *testing.T) {
	_, err := Decode(strings.NewReader(`{"feeds":[]}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing version")
}

func TestDecodeFutureVersion(t *testing.T) {
	_, err := Decode(strings.NewReader(`{"version":999}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported export version")
}

func TestDecodeEmptyExport(t *testing.T) {
	exp, err := Decode(strings.NewReader(`{"version":1}`))
	require.NoError(t, err)
	assert.Equal(t, 1, exp.Version)
	assert.Empty(t, exp.Feeds)
	assert.Empty(t, exp.Folders)
}
