package feeds

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// RSS parsing
// ---------------------------------------------------------------------------

func TestParseRSS_Basic(t *testing.T) {
	const rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>My Feed</title>
    <link>https://example.com</link>
    <description>A test feed</description>
    <item>
      <title>First Post</title>
      <link>https://example.com/1</link>
      <guid>guid-1</guid>
      <description>Short summary</description>
      <content:encoded><![CDATA[<p>Full content here</p>]]></content:encoded>
      <dc:creator>Alice</dc:creator>
      <pubDate>Mon, 02 Jan 2006 15:04:05 -0700</pubDate>
    </item>
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertEq(t, "feed.Title", feed.Title, "My Feed")
	assertEq(t, "feed.Description", feed.Description, "A test feed")
	assertEq(t, "feed.URL", feed.URL, "https://example.com")

	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	assertEq(t, "GUID", item.GUID, "guid-1")
	assertEq(t, "Title", item.Title, "First Post")
	assertEq(t, "URL", item.URL, "https://example.com/1")
	assertEq(t, "Author", item.Author, "Alice")
	assertEq(t, "Content", item.Content, "<p>Full content here</p>")
	assertEq(t, "Summary", item.Summary, "Short summary")
	if item.PublishedAt == nil {
		t.Error("PublishedAt is nil")
	}
}

func TestParseRSS_GUIDFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		wantGUID string
	}{
		{
			name: "guid present",
			xml: `<item>
				<guid>explicit-guid</guid>
				<title>T</title>
				<link>https://example.com/1</link>
			</item>`,
			wantGUID: "explicit-guid",
		},
		{
			name: "no guid, fallback to link",
			xml: `<item>
				<title>T</title>
				<link>https://example.com/2</link>
			</item>`,
			wantGUID: "https://example.com/2",
		},
		{
			name: "no guid no link, fallback to title",
			xml: `<item><title>Only Title</title></item>`,
			wantGUID: "Only Title",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rss := wrapRSS(tc.xml)
			feed, err := Parse(strings.NewReader(rss))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if len(feed.Items) == 0 {
				t.Fatal("no items")
			}
			assertEq(t, "GUID", feed.Items[0].GUID, tc.wantGUID)
		})
	}
}

func TestParseRSS_AuthorFallback(t *testing.T) {
	// <author> takes priority; dc:creator is fallback
	rss := wrapRSSNS(`
		<item>
			<title>A</title><link>https://example.com/a</link>
			<author>Bob</author>
			<dc:creator>Should Not Use</dc:creator>
		</item>
		<item>
			<title>B</title><link>https://example.com/b</link>
			<dc:creator>Charlie</dc:creator>
		</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "item0.Author", feed.Items[0].Author, "Bob")
	assertEq(t, "item1.Author", feed.Items[1].Author, "Charlie")
}

func TestParseRSS_ContentFallbackToDescription(t *testing.T) {
	rss := wrapRSS(`<item>
		<title>No Content</title>
		<link>https://example.com/x</link>
		<description>Desc only</description>
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "Content", feed.Items[0].Content, "Desc only")
}

func TestParseRSS_MediaTitleDoesNotOverwriteTitle(t *testing.T) {
	const rss = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <item>
      <title>Real Article Title</title>
      <link>https://example.com/article-1</link>
      <media:title>image-slug-name</media:title>
      <media:thumbnail url="https://example.com/thumb.jpg" width="1200" height="675" />
    </item>
    <item>
      <title>Another Article</title>
      <link>https://example.com/article-2</link>
    </item>
    <item>
      <link>https://example.com/article-3</link>
      <media:title>only-media-title</media:title>
    </item>
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(feed.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(feed.Items))
	}

	assertEq(t, "item0 title", feed.Items[0].Title, "Real Article Title")
	assertEq(t, "item1 title", feed.Items[1].Title, "Another Article")
	// Item with only media:title should fall back to it
	assertEq(t, "item2 title (media fallback)", feed.Items[2].Title, "only-media-title")
}

func TestParseRSS_HTMLEntitiesInTitle(t *testing.T) {
	rss := wrapRSS(`<item>
		<title>&quot;Hello &amp; World&#039;s&quot;</title>
		<link>https://example.com/ent</link>
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "Title", feed.Items[0].Title, `"Hello & World's"`)
}

func TestParseRSS_HTMLTagsStrippedFromTitle(t *testing.T) {
	// XML decoder consumes child elements, so bare tags in <title> lose their
	// text children. CDATA is the realistic case for HTML-in-title.
	rss := wrapRSS(`<item>
		<title><![CDATA[<b>Bold</b> and <i>italic</i>]]></title>
		<link>https://example.com/tags</link>
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "Title", feed.Items[0].Title, "Bold and italic")
}

func TestParseRSS_ChannelLinkFromAtomLink(t *testing.T) {
	const rss = `<?xml version="1.0"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>T</title>
    <atom:link href="https://example.com/feed" rel="self" type="application/rss+xml" />
    <atom:link href="https://example.com" rel="alternate" />
    <item><title>X</title><link>https://example.com/x</link></item>
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Should pick atom:link with rel="alternate"
	assertEq(t, "URL", feed.URL, "https://example.com")
}

func TestParseRSS_EmptyFeed(t *testing.T) {
	rss := wrapRSS("")
	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(feed.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(feed.Items))
	}
}

func TestParseRSS_CDATA(t *testing.T) {
	rss := wrapRSS(`<item>
		<title><![CDATA[CDATA Title <with> angle brackets]]></title>
		<link>https://example.com/cdata</link>
		<description><![CDATA[<p>HTML inside CDATA</p>]]></description>
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// stripTags should remove the angle-bracket content from title
	assertEq(t, "Title", feed.Items[0].Title, "CDATA Title  angle brackets")
	assertEq(t, "Summary", feed.Items[0].Summary, "<p>HTML inside CDATA</p>")
}

// ---------------------------------------------------------------------------
// RSS Image extraction
// ---------------------------------------------------------------------------

func TestParseRSS_ImageFromEnclosure(t *testing.T) {
	rss := wrapRSS(`<item>
		<title>Img</title><link>https://example.com/i</link>
		<enclosure url="https://example.com/photo.jpg" type="image/jpeg" />
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "ImageURL", feed.Items[0].ImageURL, "https://example.com/photo.jpg")
}

func TestParseRSS_ImageFromMediaContent(t *testing.T) {
	const rss = `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel><title>T</title><link>https://example.com</link>
    <item>
      <title>Img</title><link>https://example.com/i</link>
      <media:content url="https://example.com/vid.mp4" type="video/mp4" />
      <media:content url="https://example.com/pic.jpg" type="image/jpeg" />
    </item>
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// First image-type media:content wins
	assertEq(t, "ImageURL", feed.Items[0].ImageURL, "https://example.com/pic.jpg")
}

func TestParseRSS_ImageFromMediaThumbnail(t *testing.T) {
	const rss = `<?xml version="1.0"?>
<rss version="2.0" xmlns:media="http://search.yahoo.com/mrss/">
  <channel><title>T</title><link>https://example.com</link>
    <item>
      <title>Img</title><link>https://example.com/i</link>
      <media:thumbnail url="https://example.com/thumb.png" />
    </item>
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "ImageURL", feed.Items[0].ImageURL, "https://example.com/thumb.png")
}

func TestParseRSS_ImageFromHTMLContent(t *testing.T) {
	rss := wrapRSS(`<item>
		<title>Img</title><link>https://example.com/i</link>
		<description><![CDATA[<p>Text</p><img src="https://example.com/inline.jpg" />]]></description>
	</item>`)

	feed, err := Parse(strings.NewReader(rss))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "ImageURL", feed.Items[0].ImageURL, "https://example.com/inline.jpg")
}

// ---------------------------------------------------------------------------
// Atom parsing
// ---------------------------------------------------------------------------

func TestParseAtom_Basic(t *testing.T) {
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <subtitle>A subtitle</subtitle>
  <link href="https://example.com" rel="alternate" />
  <link href="https://example.com/feed" rel="self" />
  <entry>
    <id>urn:uuid:1</id>
    <title>Entry One</title>
    <link href="https://example.com/entry-1" rel="alternate" />
    <author><name>Dana</name></author>
    <summary>Summary text</summary>
    <content type="html">Full content</content>
    <published>2024-01-15T10:30:00Z</published>
  </entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertEq(t, "Title", feed.Title, "Atom Feed")
	assertEq(t, "Description", feed.Description, "A subtitle")
	assertEq(t, "URL", feed.URL, "https://example.com")

	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	e := feed.Items[0]
	assertEq(t, "GUID", e.GUID, "urn:uuid:1")
	assertEq(t, "Title", e.Title, "Entry One")
	assertEq(t, "URL", e.URL, "https://example.com/entry-1")
	assertEq(t, "Author", e.Author, "Dana")
	assertEq(t, "Content", e.Content, "Full content")
	assertEq(t, "Summary", e.Summary, "Summary text")
	if e.PublishedAt == nil {
		t.Error("PublishedAt is nil")
	}
}

func TestParseAtom_UpdatedFallback(t *testing.T) {
	const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>T</title>
  <entry>
    <id>1</id>
    <title>E</title>
    <updated>2024-06-01T12:00:00Z</updated>
  </entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if feed.Items[0].PublishedAt == nil {
		t.Fatal("expected PublishedAt from <updated>")
	}
	if feed.Items[0].PublishedAt.Month() != time.June {
		t.Errorf("wrong month: %v", feed.Items[0].PublishedAt)
	}
}

func TestParseAtom_YouTubeEmbed(t *testing.T) {
	const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom"
      xmlns:media="http://search.yahoo.com/mrss/">
  <title>YT</title>
  <entry>
    <id>yt:video:abc123</id>
    <title>Cool Video</title>
    <link href="https://www.youtube.com/watch?v=abc123" rel="alternate" />
    <media:group>
      <media:content url="https://www.youtube.com/v/abc123" type="application/x-shockwave-flash" />
      <media:thumbnail url="https://i.ytimg.com/vi/abc123/hq.jpg" width="480" height="360" />
      <media:description>Video description here</media:description>
    </media:group>
  </entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e := feed.Items[0]

	// Content should have iframe with embed URL
	if !strings.Contains(e.Content, "youtube.com/embed/abc123") {
		t.Errorf("expected youtube embed URL in content, got: %s", e.Content)
	}
	if !strings.Contains(e.Content, "<iframe") {
		t.Errorf("expected <iframe> in content")
	}
	// Description should be in content
	if !strings.Contains(e.Content, "Video description here") {
		t.Errorf("expected description in content")
	}
	// Image from thumbnail
	assertEq(t, "ImageURL", e.ImageURL, "https://i.ytimg.com/vi/abc123/hq.jpg")
}

func TestParseAtom_YouTubeShortsEmbed(t *testing.T) {
	const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom"
      xmlns:media="http://search.yahoo.com/mrss/">
  <title>YT</title>
  <entry>
    <id>yt:video:short1</id>
    <title>Short</title>
    <link href="https://www.youtube.com/shorts/short1" rel="alternate" />
    <media:group>
      <media:content url="https://www.youtube.com/v/short1" type="application/x-shockwave-flash" />
      <media:description>A short</media:description>
    </media:group>
  </entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !strings.Contains(feed.Items[0].Content, "youtube.com/embed/short1") {
		t.Errorf("expected shorts→embed conversion, got: %s", feed.Items[0].Content)
	}
}

func TestParseAtom_ImageFromContentHTML(t *testing.T) {
	const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>T</title>
  <entry>
    <id>1</id><title>E</title>
    <content type="html">&lt;img src="https://example.com/pic.jpg" /&gt; text</content>
  </entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "ImageURL", feed.Items[0].ImageURL, "https://example.com/pic.jpg")
}

func TestParseAtom_NoAuthor(t *testing.T) {
	const atom = `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>T</title>
  <entry><id>1</id><title>No Author</title></entry>
</feed>`

	feed, err := Parse(strings.NewReader(atom))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertEq(t, "Author", feed.Items[0].Author, "")
}

// ---------------------------------------------------------------------------
// Parse dispatch (RSS vs Atom) and errors
// ---------------------------------------------------------------------------

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse(strings.NewReader("this is not xml"))
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestParse_EmptyInput(t *testing.T) {
	_, err := Parse(strings.NewReader(""))
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParse_HTMLInput(t *testing.T) {
	_, err := Parse(strings.NewReader("<html><body>Not a feed</body></html>"))
	if err == nil {
		t.Error("expected error for HTML input")
	}
}

// ---------------------------------------------------------------------------
// parseTime
// ---------------------------------------------------------------------------

func TestParseTime(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		// RFC1123Z
		{"Mon, 02 Jan 2006 15:04:05 -0700", false},
		// RFC1123
		{"Mon, 02 Jan 2006 15:04:05 MST", false},
		// RFC3339
		{"2006-01-02T15:04:05Z", false},
		{"2006-01-02T15:04:05+07:00", false},
		// Date-time without timezone
		{"2006-01-02 15:04:05", false},
		// Day without leading zero
		{"Mon, 2 Jan 2006 15:04:05 MST", false},
		{"Mon, 2 Jan 2006 15:04:05 -0700", false},
		// Two-digit day first
		{"02 Jan 2006 15:04:05 MST", false},
		{"02 Jan 2006 15:04:05 -0700", false},
		// Invalid
		{"not a date", true},
		{"", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			_, err := parseTime(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseTime(%q) err=%v, wantErr=%v", tc.input, err, tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// stripTags
// ---------------------------------------------------------------------------

func TestStripTags(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"plain text", "plain text"},
		{"<b>bold</b>", "bold"},
		{"<a href='x'>link</a> &amp; more", "link & more"},
		{"&lt;not a tag&gt;", "<not a tag>"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{"&#039;apostrophe&#039;", "'apostrophe'"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := stripTags(tc.input)
			assertEq(t, "stripTags", got, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// sanitizeUTF8
// ---------------------------------------------------------------------------

func TestSanitizeUTF8(t *testing.T) {
	// Valid UTF-8 passes through unchanged
	valid := []byte("hello world")
	got := sanitizeUTF8(valid)
	if string(got) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", got)
	}

	// Invalid bytes are stripped
	invalid := []byte{'h', 'i', 0xff, 0xfe, '!'}
	got = sanitizeUTF8(invalid)
	if string(got) != "hi!" {
		t.Errorf("expected %q, got %q", "hi!", got)
	}
}

// ---------------------------------------------------------------------------
// extractImageFromHTML
// ---------------------------------------------------------------------------

func TestExtractImageFromHTML(t *testing.T) {
	tests := []struct {
		name, html, want string
	}{
		{"basic img", `<img src="https://example.com/a.jpg" />`, "https://example.com/a.jpg"},
		{"single quotes", `<img src='https://example.com/b.png' />`, "https://example.com/b.png"},
		{"with attrs", `<img class="thumb" src="https://example.com/c.gif" width="100" />`, "https://example.com/c.gif"},
		{"no img", `<p>no image here</p>`, ""},
		{"empty", "", ""},
		{"entity in src", `<img src="https://example.com/a&amp;b.jpg" />`, "https://example.com/a&b.jpg"},
		{"multiple imgs", `<img src="first.jpg" /><img src="second.jpg" />`, "first.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractImageFromHTML(tc.html)
			assertEq(t, "extractImageFromHTML", got, tc.want)
		})
	}
}

// ---------------------------------------------------------------------------
// looksLikeImage
// ---------------------------------------------------------------------------

func TestLooksLikeImage(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/photo.jpg", true},
		{"https://example.com/photo.JPEG", true},
		{"https://example.com/pic.png", true},
		{"https://example.com/anim.gif", true},
		{"https://example.com/modern.webp", true},
		{"https://example.com/vector.svg", true},
		{"https://example.com/video.mp4", false},
		{"https://example.com/page.html", false},
		{"https://example.com/no-extension", false},
	}
	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			got := looksLikeImage(tc.url)
			if got != tc.want {
				t.Errorf("looksLikeImage(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func wrapRSS(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test</title>
    <link>https://example.com</link>
    ` + items + `
  </channel>
</rss>`
}

func wrapRSSNS(items string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:content="http://purl.org/rss/1.0/modules/content/"
  xmlns:media="http://search.yahoo.com/mrss/">
  <channel>
    <title>Test</title>
    <link>https://example.com</link>
    ` + items + `
  </channel>
</rss>`
}

func assertEq(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}
