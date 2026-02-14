package feeds

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// FeedItem represents a single item from any feed type
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

// ParsedFeed contains the parsed feed data
type ParsedFeed struct {
	Title         string
	Description   string
	URL           string
	LastBuildDate *time.Time
	Items         []FeedItem
}

// Parse attempts to parse the feed content as RSS or Atom
func Parse(r io.Reader) (*ParsedFeed, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read feed: %w", err)
	}

	// Sanitize invalid UTF-8 bytes that can cause XML parsing to fail
	data = sanitizeUTF8(data)

	// Try RSS first
	if feed, err := parseRSS(data); err == nil {
		return feed, nil
	}

	// Try Atom
	if feed, err := parseAtom(data); err == nil {
		return feed, nil
	}

	return nil, fmt.Errorf("unable to parse as RSS or Atom feed")
}

// sanitizeUTF8 removes invalid UTF-8 sequences from the data
func sanitizeUTF8(data []byte) []byte {
	if utf8.Valid(data) {
		return data
	}

	// Replace invalid UTF-8 sequences with the replacement character
	var buf bytes.Buffer
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size == 1 {
			// Invalid byte, skip it
			data = data[1:]
			continue
		}
		buf.WriteRune(r)
		data = data[size:]
	}
	return buf.Bytes()
}

// RSS structures
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

// rssLink captures both plain <link> text and <atom:link> attributes.
type rssLink struct {
	XMLName xml.Name
	Href    string `xml:"href,attr"`
	Rel     string `xml:"rel,attr"`
	Text    string `xml:",chardata"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Links         []rssLink `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate"`
	Items         []rssItem `xml:"item"`
}

// ChannelLink returns the site URL from the channel's <link> elements,
// preferring the plain-text <link> over atom:link.
func (ch rssChannel) ChannelLink() string {
	for _, l := range ch.Links {
		if l.Text != "" {
			return strings.TrimSpace(l.Text)
		}
	}
	// Fall back to atom:link with rel="alternate"
	for _, l := range ch.Links {
		if l.Href != "" && (l.Rel == "alternate" || l.Rel == "") {
			return l.Href
		}
	}
	return ""
}

// xmlTitle captures a <title> element along with its namespace, so we can
// distinguish <title> (no namespace) from <media:title> (MRSS namespace).
type xmlTitle struct {
	XMLName xml.Name
	Value   string `xml:",chardata"`
}

type rssItem struct {
	Titles         []xmlTitle       `xml:"title"`
	Link           string           `xml:"link"`
	Description    string           `xml:"description"`
	Content        string           `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	GUID           string           `xml:"guid"`
	PubDate        string           `xml:"pubDate"`
	Author         string           `xml:"author"`
	Creator        string           `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Enclosure      *rssEnclosure    `xml:"enclosure"`
	MediaContent   []mediaContent   `xml:"http://search.yahoo.com/mrss/ content"`
	MediaThumbnail []mediaThumbnail `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	MediaGroup     *mediaGroup      `xml:"http://search.yahoo.com/mrss/ group"`
}

// Title returns the non-namespaced <title> value, ignoring <media:title> etc.
func (item rssItem) Title() string {
	for _, t := range item.Titles {
		if t.XMLName.Space == "" {
			return t.Value
		}
	}
	// Fallback: return any title if no un-namespaced one exists
	if len(item.Titles) > 0 {
		return item.Titles[0].Value
	}
	return ""
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
}

type mediaThumbnail struct {
	URL    string `xml:"url,attr"`
	Width  string `xml:"width,attr"`
	Height string `xml:"height,attr"`
}

func parseRSS(data []byte) (*ParsedFeed, error) {
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}
	if rss.XMLName.Local != "rss" {
		return nil, fmt.Errorf("not an RSS feed")
	}

	var lastBuildDate *time.Time
	if rss.Channel.LastBuildDate != "" {
		if t, err := parseTime(rss.Channel.LastBuildDate); err == nil && !isEpoch(t) {
			lastBuildDate = &t
		}
	}

	feed := &ParsedFeed{
		Title:         rss.Channel.Title,
		Description:   rss.Channel.Description,
		URL:           rss.Channel.ChannelLink(),
		LastBuildDate: lastBuildDate,
		Items:         make([]FeedItem, 0, len(rss.Channel.Items)),
	}

	for _, item := range rss.Channel.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == "" {
			guid = item.Title()
		}

		author := item.Author
		if author == "" {
			author = item.Creator
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		imageURL := extractImageURL(item, content)

		var pubTime *time.Time
		if item.PubDate != "" {
			if t, err := parseTime(item.PubDate); err == nil && !isEpoch(t) {
				pubTime = &t
			}
		}
		// Fall back to channel lastBuildDate for missing/epoch pub dates
		if pubTime == nil && lastBuildDate != nil {
			pubTime = lastBuildDate
		}

		feed.Items = append(feed.Items, FeedItem{
			GUID:        guid,
			Title:       stripTags(item.Title()),
			URL:         item.Link,
			Author:      author,
			Content:     content,
			Summary:     item.Description,
			ImageURL:    imageURL,
			PublishedAt: pubTime,
		})
	}

	return feed, nil
}

// Atom structures
type atomFeed struct {
	XMLName  xml.Name    `xml:"feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Links    []atomLink  `xml:"link"`
	Updated  string      `xml:"updated"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type atomEntry struct {
	ID         string       `xml:"id"`
	Title      string       `xml:"title"`
	Links      []atomLink   `xml:"link"`
	Summary    string       `xml:"summary"`
	Content    *atomContent `xml:"content"`
	Author     *atomAuthor  `xml:"author"`
	Published  string       `xml:"published"`
	Updated    string       `xml:"updated"`
	MediaGroup *mediaGroup  `xml:"http://search.yahoo.com/mrss/ group"`
}

type mediaGroup struct {
	Thumbnail   *mediaThumbnail `xml:"http://search.yahoo.com/mrss/ thumbnail"`
	Content     *mediaContent   `xml:"http://search.yahoo.com/mrss/ content"`
	Description string          `xml:"http://search.yahoo.com/mrss/ description"`
}

type atomContent struct {
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

func parseAtom(data []byte) (*ParsedFeed, error) {
	var atom atomFeed
	if err := xml.Unmarshal(data, &atom); err != nil {
		return nil, err
	}
	if atom.XMLName.Local != "feed" {
		return nil, fmt.Errorf("not an Atom feed")
	}

	var feedURL string
	for _, link := range atom.Links {
		if link.Rel == "" || link.Rel == "alternate" {
			feedURL = link.Href
			break
		}
	}

	var lastBuildDate *time.Time
	if atom.Updated != "" {
		if t, err := parseTime(atom.Updated); err == nil && !isEpoch(t) {
			lastBuildDate = &t
		}
	}

	feed := &ParsedFeed{
		Title:         atom.Title,
		Description:   atom.Subtitle,
		URL:           feedURL,
		LastBuildDate: lastBuildDate,
		Items:         make([]FeedItem, 0, len(atom.Entries)),
	}

	for _, entry := range atom.Entries {
		var url string
		for _, link := range entry.Links {
			if link.Rel == "" || link.Rel == "alternate" {
				url = link.Href
				break
			}
		}

		var author string
		if entry.Author != nil {
			author = entry.Author.Name
		}

		var content string
		if entry.Content != nil {
			content = entry.Content.Content
		}

		// If no content/summary, use media:group description and embed
		if content == "" && entry.Summary == "" && entry.MediaGroup != nil {
			var parts []string
			// Add video embed if media:content is a video
			if entry.MediaGroup.Content != nil && entry.MediaGroup.Content.URL != "" && url != "" {
				// Use the article URL for embedding (works better than Flash URLs)
				embedURL := url
				// Convert YouTube watch URLs to embed URLs
				if strings.Contains(embedURL, "youtube.com/watch?v=") {
					embedURL = strings.Replace(embedURL, "/watch?v=", "/embed/", 1)
				} else if strings.Contains(embedURL, "youtube.com/shorts/") {
					embedURL = strings.Replace(embedURL, "/shorts/", "/embed/", 1)
				}
				parts = append(parts, fmt.Sprintf(`<iframe src="%s" allowfullscreen></iframe>`, html.EscapeString(embedURL)))
			}
			if entry.MediaGroup.Description != "" {
				// Wrap plain text description in a paragraph, preserving newlines
				desc := html.EscapeString(entry.MediaGroup.Description)
				desc = strings.ReplaceAll(desc, "\n", "<br>")
				parts = append(parts, "<p>"+desc+"</p>")
			}
			if len(parts) > 0 {
				content = strings.Join(parts, "\n")
			}
		}

		var pubTime *time.Time
		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}
		if dateStr != "" {
			if t, err := parseTime(dateStr); err == nil && !isEpoch(t) {
				pubTime = &t
			}
		}
		// Fall back to feed-level updated date for missing/epoch pub dates
		if pubTime == nil && lastBuildDate != nil {
			pubTime = lastBuildDate
		}

		// Extract image - check media:group thumbnail first (YouTube), then HTML content
		var imageURL string
		if entry.MediaGroup != nil && entry.MediaGroup.Thumbnail != nil && entry.MediaGroup.Thumbnail.URL != "" {
			imageURL = entry.MediaGroup.Thumbnail.URL
		} else if imageURL = extractImageFromHTML(content); imageURL == "" {
			imageURL = extractImageFromHTML(entry.Summary)
		}

		feed.Items = append(feed.Items, FeedItem{
			GUID:        entry.ID,
			Title:       stripTags(entry.Title),
			URL:         url,
			Author:      author,
			Content:     content,
			Summary:     entry.Summary,
			ImageURL:    imageURL,
			PublishedAt: pubTime,
		})
	}

	return feed, nil
}

// extractImageURL finds an image URL from various RSS sources
func extractImageURL(item rssItem, content string) string {
	// 1. Check enclosure with image type
	if item.Enclosure != nil && strings.HasPrefix(item.Enclosure.Type, "image/") {
		return item.Enclosure.URL
	}

	// 2. Check media:content elements
	for _, mc := range item.MediaContent {
		if mc.URL != "" {
			// Accept if type is image or medium is image, or no type specified (common for images)
			if strings.HasPrefix(mc.Type, "image/") || mc.Medium == "image" || (mc.Type == "" && mc.Medium == "") {
				if looksLikeImage(mc.URL) || mc.Type != "" || mc.Medium != "" {
					return mc.URL
				}
			}
		}
	}

	// 3. Check media:thumbnail elements (direct on item)
	for _, mt := range item.MediaThumbnail {
		if mt.URL != "" {
			return mt.URL
		}
	}

	// 4. Check media:group > media:thumbnail
	if item.MediaGroup != nil && item.MediaGroup.Thumbnail != nil && item.MediaGroup.Thumbnail.URL != "" {
		return item.MediaGroup.Thumbnail.URL
	}

	// 5. Extract first image from HTML content
	if imgURL := extractImageFromHTML(content); imgURL != "" {
		return imgURL
	}

	return ""
}

// looksLikeImage checks if URL looks like an image based on extension
func looksLikeImage(url string) bool {
	lower := strings.ToLower(url)
	extensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	for _, ext := range extensions {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	return false
}

// extractImageFromHTML extracts the first img src from HTML content
func extractImageFromHTML(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	// Simple regex to find img src
	// Look for <img ... src="..." ...>
	imgPattern := `<img[^>]+src=["']([^"']+)["']`
	re := regexp.MustCompile(imgPattern)
	matches := re.FindStringSubmatch(htmlContent)
	if len(matches) >= 2 {
		return html.UnescapeString(matches[1])
	}
	return ""
}

// isEpoch returns true if the time is the Unix epoch (1970-01-01 00:00:00 UTC),
// which many feeds use as a placeholder for missing dates.
func isEpoch(t time.Time) bool {
	return t.Unix() == 0
}

// parseTime tries various date formats
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", s)
}

var tagRegexp = regexp.MustCompile(`<[^>]*>`)

// stripTags removes HTML tags and decodes entities.
func stripTags(s string) string {
	return strings.TrimSpace(html.UnescapeString(tagRegexp.ReplaceAllString(s, "")))
}
