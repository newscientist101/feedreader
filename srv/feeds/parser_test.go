package feeds

import (
	"strings"
	"testing"
)

func TestRSSMediaTitleDoesNotOverwriteTitle(t *testing.T) {
	const rssData = `<?xml version="1.0" encoding="UTF-8"?>
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
  </channel>
</rss>`

	feed, err := Parse(strings.NewReader(rssData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(feed.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(feed.Items))
	}

	// The first item has both <title> and <media:title>.
	// The parser must use the real <title>, not <media:title>.
	if feed.Items[0].Title != "Real Article Title" {
		t.Errorf("expected title %q, got %q", "Real Article Title", feed.Items[0].Title)
	}

	if feed.Items[1].Title != "Another Article" {
		t.Errorf("expected title %q, got %q", "Another Article", feed.Items[1].Title)
	}
}
