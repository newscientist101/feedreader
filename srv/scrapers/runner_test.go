package scrapers

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// HTML scraper (type: "html")
// ---------------------------------------------------------------------------

func TestRunHTMLScraper_Basic(t *testing.T) {
	config := `{
		"type": "html",
		"itemSelector": ".post",
		"titleSelector": "h2",
		"urlSelector": "a.link",
		"urlAttr": "href",
		"summarySelector": "p.summary",
		"authorSelector": ".author",
		"imageSelector": "img",
		"imageAttr": "src",
		"dateSelector": "time",
		"dateAttr": "datetime",
		"baseUrl": "https://example.com"
	}`

	html := `<html><body>
		<div class="post">
			<h2>First Post</h2>
			<a class="link" href="/articles/1">Read more</a>
			<p class="summary">This is the first post.</p>
			<span class="author">Alice</span>
			<img src="/img/1.jpg" />
			<time datetime="2024-01-15">Jan 15</time>
		</div>
		<div class="post">
			<h2>Second Post</h2>
			<a class="link" href="https://other.example/2">Read more</a>
			<p class="summary">Second summary.</p>
			<span class="author">Bob</span>
		</div>
	</body></html>`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, html, "https://example.com/page", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First item
	i0 := items[0]
	assertEq(t, "Title", i0.Title, "First Post")
	assertEq(t, "URL", i0.URL, "https://example.com/articles/1") // relative resolved
	assertEq(t, "Summary", i0.Summary, "This is the first post.")
	assertEq(t, "Author", i0.Author, "Alice")
	assertEq(t, "ImageURL", i0.ImageURL, "https://example.com/img/1.jpg")
	if i0.PublishedAt == nil {
		t.Error("item0: PublishedAt is nil")
	}
	assertEq(t, "GUID", i0.GUID, "https://example.com/articles/1")

	// Second item: absolute URL stays absolute
	assertEq(t, "item1.URL", items[1].URL, "https://other.example/2")
}

func TestRunHTMLScraper_MissingItemSelector(t *testing.T) {
	config := `{"type": "html", "titleSelector": "h2"}`
	r := NewRunner()
	_, err := r.Run(context.Background(), config, "<html></html>", "", "")
	if err == nil {
		t.Error("expected error for missing itemSelector")
	}
}

func TestRunHTMLScraper_DefaultAttrs(t *testing.T) {
	// urlAttr defaults to "href", imageAttr defaults to "src"
	config := `{
		"type": "html",
		"itemSelector": ".item",
		"titleSelector": "h3",
		"urlSelector": "a",
		"imageSelector": "img"
	}`

	html := `<div class="item">
		<h3>Title</h3>
		<a href="https://example.com/link">Link</a>
		<img src="https://example.com/pic.jpg" />
	</div>`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, html, "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	assertEq(t, "URL", items[0].URL, "https://example.com/link")
	assertEq(t, "ImageURL", items[0].ImageURL, "https://example.com/pic.jpg")
}

func TestRunHTMLScraper_SkipsEmptyItems(t *testing.T) {
	config := `{"type": "html", "itemSelector": "li", "titleSelector": "span"}`
	html := `<ul>
		<li><span>Has title</span></li>
		<li></li>
	</ul>`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, html, "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Empty item has no title, no summary, no URL -> skipped
	if len(items) != 1 {
		t.Errorf("expected 1 item (empty skipped), got %d", len(items))
	}
}

// ---------------------------------------------------------------------------
// JSON scraper (type: "json")
// ---------------------------------------------------------------------------

func TestRunJSONScraper_Basic(t *testing.T) {
	config := `{
		"type": "json",
		"itemsPath": "data.posts",
		"titlePath": "title",
		"urlPath": "link",
		"authorPath": "author.name",
		"summaryPath": "excerpt",
		"datePath": "date",
		"baseUrl": "https://example.com"
	}`

	json := `{
		"data": {
			"posts": [
				{
					"title": "Post One",
					"link": "/p/1",
					"author": {"name": "Alice"},
					"excerpt": "First post.",
					"date": "2024-06-15"
				},
				{
					"title": "Post Two",
					"link": "https://other.com/2",
					"author": {"name": "Bob"},
					"excerpt": "Second."
				}
			]
		}
	}`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, json, "https://example.com/api", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	assertEq(t, "item0.Title", items[0].Title, "Post One")
	assertEq(t, "item0.URL", items[0].URL, "https://example.com/p/1")
	assertEq(t, "item0.Author", items[0].Author, "Alice")
	assertEq(t, "item0.Summary", items[0].Summary, "First post.")
	if items[0].PublishedAt == nil {
		t.Error("item0: expected PublishedAt")
	}

	// Absolute URL stays as-is
	assertEq(t, "item1.URL", items[1].URL, "https://other.com/2")
}

func TestRunJSONScraper_ItemsPathEmpty(t *testing.T) {
	// Top-level array
	config := `{"type": "json", "itemsPath": "", "titlePath": "t"}`
	json := `[{"t": "A"}, {"t": "B"}]`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, json, "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
}

func TestRunJSONScraper_InvalidPath(t *testing.T) {
	config := `{"type": "json", "itemsPath": "nonexistent", "titlePath": "t"}`
	json := `{"data": []}`

	r := NewRunner()
	_, err := r.Run(context.Background(), config, json, "", "")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestRunJSONScraper_ConsolidateDuplicates(t *testing.T) {
	config := `{
		"type": "json",
		"itemsPath": "",
		"titlePath": "title",
		"summaryPath": "text",
		"consolidateDuplicates": true
	}`
	json := `[
		{"title": "Same", "text": "Line 1"},
		{"title": "Same", "text": "Line 2"},
		{"title": "Same", "text": "Line 3"},
		{"title": "Different", "text": "Other"}
	]`

	r := NewRunner()
	items, err := r.Run(context.Background(), config, json, "", "")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// 3 "Same" items consolidated into 1, plus 1 "Different"
	if len(items) != 2 {
		t.Fatalf("expected 2 items after consolidation, got %d", len(items))
	}

	if items[0].Title != "Same - Combined(3)" {
		t.Errorf("consolidated title = %q", items[0].Title)
	}
	assertEq(t, "item1.Title", items[1].Title, "Different")
}

func TestRunJSONScraper_NotArray(t *testing.T) {
	config := `{"type": "json", "itemsPath": "data", "titlePath": "t"}`
	json := `{"data": "not an array"}`

	r := NewRunner()
	_, err := r.Run(context.Background(), config, json, "", "")
	if err == nil {
		t.Error("expected error when items path resolves to non-array")
	}
}

func TestRunJSONScraper_InvalidJSON(t *testing.T) {
	config := `{"type": "json", "itemsPath": "", "titlePath": "t"}`
	r := NewRunner()
	_, err := r.Run(context.Background(), config, "not json", "", "")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ---------------------------------------------------------------------------
// Run dispatch
// ---------------------------------------------------------------------------

func TestRun_UnknownType(t *testing.T) {
	r := NewRunner()
	_, err := r.Run(context.Background(), `{"type": "lua"}`, "", "", "")
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestRun_MissingType(t *testing.T) {
	r := NewRunner()
	_, err := r.Run(context.Background(), `{"itemSelector": "div"}`, "<div></div>", "", "")
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestRun_InvalidConfig(t *testing.T) {
	r := NewRunner()
	_, err := r.Run(context.Background(), "not json at all", "", "", "")
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

// ---------------------------------------------------------------------------
// resolveURL
// ---------------------------------------------------------------------------

func TestResolveURL(t *testing.T) {
	tests := []struct {
		name, base, page, href, want string
	}{
		{"absolute", "https://example.com", "", "https://other.com/x", "https://other.com/x"},
		{"relative path", "https://example.com", "", "/foo/bar", "https://example.com/foo/bar"},
		{"protocol-relative", "https://example.com", "", "//cdn.example.com/img.jpg", "https://cdn.example.com/img.jpg"},
		{"protocol-relative http", "http://example.com", "", "//cdn.example.com/img.jpg", "http://cdn.example.com/img.jpg"},
		{"page fallback", "", "https://page.example.com/p", "/path", "https://page.example.com/path"},
		{"no base", "", "", "/orphan", "/orphan"},
		{"empty href", "https://example.com", "", "", ""},
		{"whitespace", "https://example.com", "", "  /trimmed  ", "https://example.com/trimmed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveURL(tc.base, tc.page, tc.href)
			if got != tc.want {
				t.Errorf("resolveURL(%q, %q, %q) = %q, want %q", tc.base, tc.page, tc.href, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseFlexibleDate
// ---------------------------------------------------------------------------

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"2024-01-15T10:30:00Z", false},
		{"2024-01-15", false},
		{"Jan 2, 2006", false},
		{"January 2, 2006", false},
		{"02 Jan 2006", false},
		{"2 Jan 2006", false},
		{"not a date", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			_, err := parseFlexibleDate(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseFlexibleDate(%q): err=%v, wantErr=%v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// navigatePath / getStringPath
// ---------------------------------------------------------------------------

func TestNavigatePath(t *testing.T) {
	data := map[string]any{
		"a": map[string]any{
			"b": map[string]any{
				"c": "deep",
			},
		},
		"top": "level",
	}

	t.Run("top-level key", func(t *testing.T) {
		got, err := navigatePath(data, "top")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "level" {
			t.Errorf("got %v, want %q", got, "level")
		}
	})

	t.Run("deep path", func(t *testing.T) {
		got, err := navigatePath(data, "a.b.c")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "deep" {
			t.Errorf("got %v, want %q", got, "deep")
		}
	})

	t.Run("empty path returns root", func(t *testing.T) {
		got, err := navigatePath(data, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Can't compare maps directly; just check it's not nil
		if got == nil {
			t.Error("expected non-nil result")
		}
		if m, ok := got.(map[string]any); !ok || m["top"] != "level" {
			t.Errorf("expected original map, got %v", got)
		}
	})

	t.Run("nonexistent key", func(t *testing.T) {
		_, err := navigatePath(data, "nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("deep nonexistent key", func(t *testing.T) {
		_, err := navigatePath(data, "a.nonexistent")
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestGetStringPath(t *testing.T) {
	data := map[string]any{
		"str":  "hello",
		"num":  float64(42),
		"bool": true,
		"null": nil,
	}

	assertEq(t, "str", getStringPath(data, "str"), "hello")
	assertEq(t, "num", getStringPath(data, "num"), "42")
	assertEq(t, "bool", getStringPath(data, "bool"), "true")
	assertEq(t, "null", getStringPath(data, "null"), "")
	assertEq(t, "missing", getStringPath(data, "missing"), "")
	assertEq(t, "empty path", getStringPath(data, ""), "")
}

// ---------------------------------------------------------------------------
// consolidateItems
// ---------------------------------------------------------------------------

func TestConsolidateItems_NoDuplicates(t *testing.T) {
	items := []FeedItem{
		{Title: "A", GUID: "1", Summary: "s1"},
		{Title: "B", GUID: "2", Summary: "s2"},
	}
	result := consolidateItems(items)
	if len(result) != 2 {
		t.Errorf("expected 2, got %d", len(result))
	}
}

func TestConsolidateItems_Empty(t *testing.T) {
	result := consolidateItems(nil)
	if len(result) != 0 {
		t.Errorf("expected 0, got %d", len(result))
	}
}

func TestConsolidateItems_AllSame(t *testing.T) {
	items := []FeedItem{
		{Title: "X", GUID: "1", Summary: "a"},
		{Title: "X", GUID: "2", Summary: "b"},
	}
	result := consolidateItems(items)
	if len(result) != 1 {
		t.Fatalf("expected 1 consolidated, got %d", len(result))
	}
	if result[0].Title != "X - Combined(2)" {
		t.Errorf("title = %q", result[0].Title)
	}
}

func TestConsolidateItems_NonConsecutiveDupsNotMerged(t *testing.T) {
	items := []FeedItem{
		{Title: "A", GUID: "1", Summary: "s"},
		{Title: "B", GUID: "2", Summary: "s"},
		{Title: "A", GUID: "3", Summary: "s"},
	}
	result := consolidateItems(items)
	// Not consecutive, so no merging
	if len(result) != 3 {
		t.Errorf("expected 3 (non-consecutive not merged), got %d", len(result))
	}
}

// ---------------------------------------------------------------------------
// splitSentences
// ---------------------------------------------------------------------------

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single line", "Hello world", 1},
		{"newline split", "line1\nline2", 2},
		{"paragraph split", "para1\n\npara2", 2},
		{"mixed", "a\n\nb\nc\n\nd", 4},
		{"whitespace only", "  \n  \n\n  ", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitSentences(tc.input)
			if len(got) != tc.want {
				t.Errorf("splitSentences(%q) = %d sentences, want %d", tc.input, len(got), tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertEq(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}
