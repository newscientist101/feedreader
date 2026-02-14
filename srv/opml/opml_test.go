package opml

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

func TestParse_FlatFeeds(t *testing.T) {
	const data = `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head><title>My Feeds</title></head>
  <body>
    <outline text="Hacker News" type="rss" xmlUrl="https://hn.example/feed" htmlUrl="https://hn.example" />
    <outline text="Lobsters" type="rss" xmlUrl="https://lobste.rs/rss" />
  </body>
</opml>`

	feeds, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(feeds) != 2 {
		t.Fatalf("expected 2 feeds, got %d", len(feeds))
	}

	assertFeed(t, feeds[0], "Hacker News", "https://hn.example/feed", "https://hn.example", "")
	assertFeed(t, feeds[1], "Lobsters", "https://lobste.rs/rss", "", "")
}

func TestParse_NestedFolders(t *testing.T) {
	const data = `<?xml version="1.0"?>
<opml version="2.0">
  <head><title>Nested</title></head>
  <body>
    <outline text="Tech">
      <outline text="Ars" title="Ars Technica" type="rss" xmlUrl="https://ars.example/feed" htmlUrl="https://ars.example" />
      <outline text="Wired" type="rss" xmlUrl="https://wired.example/feed" />
    </outline>
    <outline text="News">
      <outline text="BBC" type="rss" xmlUrl="https://bbc.example/feed" />
    </outline>
    <outline text="Uncategorized Feed" type="rss" xmlUrl="https://example.com/feed" />
  </body>
</opml>`

	feeds, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(feeds) != 4 {
		t.Fatalf("expected 4 feeds, got %d", len(feeds))
	}

	// Title attr takes priority over text
	assertFeed(t, feeds[0], "Ars Technica", "https://ars.example/feed", "https://ars.example", "Tech")
	assertFeed(t, feeds[1], "Wired", "https://wired.example/feed", "", "Tech")
	assertFeed(t, feeds[2], "BBC", "https://bbc.example/feed", "", "News")
	assertFeed(t, feeds[3], "Uncategorized Feed", "https://example.com/feed", "", "")
}

func TestParse_TitleFallbackToText(t *testing.T) {
	const data = `<?xml version="1.0"?>
<opml version="2.0">
  <body>
    <outline text="TextOnly" type="rss" xmlUrl="https://example.com/feed" />
  </body>
</opml>`

	feeds, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if feeds[0].Name != "TextOnly" {
		t.Errorf("Name = %q, want %q", feeds[0].Name, "TextOnly")
	}
}

func TestParse_EmptyBody(t *testing.T) {
	const data = `<?xml version="1.0"?>
<opml version="2.0"><body></body></opml>`

	feeds, err := Parse(strings.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(feeds) != 0 {
		t.Errorf("expected 0 feeds, got %d", len(feeds))
	}
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse(strings.NewReader("not xml"))
	if err == nil {
		t.Error("expected error")
	}
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func TestExport_Basic(t *testing.T) {
	feeds := []ExportFeed{
		{Name: "Uncategorized", URL: "https://example.com/feed", SiteURL: "https://example.com"},
		{Name: "Tech Blog", URL: "https://tech.example/rss", SiteURL: "https://tech.example", Category: "Tech"},
		{Name: "Another Tech", URL: "https://another.example/rss", Category: "Tech"},
	}

	data, err := Export(feeds, "Test Export")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	xml := string(data)

	// Should have XML header
	if !strings.HasPrefix(xml, "<?xml") {
		t.Error("missing XML header")
	}

	// Should have version attribute
	if !strings.Contains(xml, `version="2.0"`) {
		t.Error("missing version 2.0")
	}

	// Should contain title
	if !strings.Contains(xml, "Test Export") {
		t.Error("missing title")
	}

	// Should contain all feed URLs
	for _, f := range feeds {
		if !strings.Contains(xml, f.URL) {
			t.Errorf("missing feed URL %q", f.URL)
		}
	}

	// Should be parseable back
	parsed, err := Parse(strings.NewReader(xml))
	if err != nil {
		t.Fatalf("re-parse exported OPML: %v", err)
	}
	if len(parsed) != 3 {
		t.Errorf("round-trip: expected 3 feeds, got %d", len(parsed))
	}
}

func TestExport_Empty(t *testing.T) {
	data, err := Export(nil, "Empty")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if !strings.Contains(string(data), "<body") {
		t.Error("missing body element")
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`<opml version="2.0">...`, true},
		{`<OPML>`, true},
		{`<?xml version="1.0"?><opml>`, true},
		{`<rss version="2.0">`, false},
		{`just text`, false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := Validate(tc.input)
			if got != tc.want {
				t.Errorf("Validate(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Roundtrip: Export → Parse
// ---------------------------------------------------------------------------

func TestRoundtrip(t *testing.T) {
	original := []ExportFeed{
		{Name: "Feed A", URL: "https://a.example/rss", SiteURL: "https://a.example", Category: "Cat1"},
		{Name: "Feed B", URL: "https://b.example/rss", SiteURL: "https://b.example", Category: "Cat1"},
		{Name: "Feed C", URL: "https://c.example/rss", SiteURL: "https://c.example", Category: "Cat2"},
		{Name: "Standalone", URL: "https://standalone.example/rss", SiteURL: "https://standalone.example"},
	}

	data, err := Export(original, "Roundtrip Test")
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	parsed, err := Parse(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(parsed) != len(original) {
		t.Fatalf("expected %d feeds, got %d", len(original), len(parsed))
	}

	// Build lookup by URL
	byURL := make(map[string]Feed)
	for _, f := range parsed {
		byURL[f.URL] = f
	}

	for _, orig := range original {
		f, ok := byURL[orig.URL]
		if !ok {
			t.Errorf("missing feed %q", orig.URL)
			continue
		}
		if f.Name != orig.Name {
			t.Errorf("feed %q: Name = %q, want %q", orig.URL, f.Name, orig.Name)
		}
		if f.Category != orig.Category {
			t.Errorf("feed %q: Category = %q, want %q", orig.URL, f.Category, orig.Category)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertFeed(t *testing.T, f Feed, name, url, siteURL, category string) {
	t.Helper()
	if f.Name != name {
		t.Errorf("Name = %q, want %q", f.Name, name)
	}
	if f.URL != url {
		t.Errorf("URL = %q, want %q", f.URL, url)
	}
	if f.SiteURL != siteURL {
		t.Errorf("SiteURL = %q, want %q", f.SiteURL, siteURL)
	}
	if f.Category != category {
		t.Errorf("Category = %q, want %q", f.Category, category)
	}
}
