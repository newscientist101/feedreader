package feeds

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"strings"
	"time"
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
	Title       string
	Description string
	URL         string
	Items       []FeedItem
}

// Parse attempts to parse the feed content as RSS or Atom
func Parse(r io.Reader) (*ParsedFeed, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read feed: %w", err)
	}

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

// RSS structures
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description string        `xml:"description"`
	Content     string        `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	GUID        string        `xml:"guid"`
	PubDate     string        `xml:"pubDate"`
	Author      string        `xml:"author"`
	Creator     string        `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Enclosure   *rssEnclosure `xml:"enclosure"`
	MediaContent *mediaContent `xml:"http://search.yahoo.com/mrss/ content"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaContent struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

func parseRSS(data []byte) (*ParsedFeed, error) {
	var rss rssFeed
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, err
	}
	if rss.XMLName.Local != "rss" {
		return nil, fmt.Errorf("not an RSS feed")
	}

	feed := &ParsedFeed{
		Title:       rss.Channel.Title,
		Description: rss.Channel.Description,
		URL:         rss.Channel.Link,
		Items:       make([]FeedItem, 0, len(rss.Channel.Items)),
	}

	for _, item := range rss.Channel.Items {
		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}
		if guid == "" {
			guid = item.Title
		}

		author := item.Author
		if author == "" {
			author = item.Creator
		}

		content := item.Content
		if content == "" {
			content = item.Description
		}

		var imageURL string
		if item.Enclosure != nil && strings.HasPrefix(item.Enclosure.Type, "image/") {
			imageURL = item.Enclosure.URL
		}
		if imageURL == "" && item.MediaContent != nil && strings.HasPrefix(item.MediaContent.Type, "image/") {
			imageURL = item.MediaContent.URL
		}

		var pubTime *time.Time
		if item.PubDate != "" {
			if t, err := parseTime(item.PubDate); err == nil {
				pubTime = &t
			}
		}

		feed.Items = append(feed.Items, FeedItem{
			GUID:        guid,
			Title:       html.UnescapeString(item.Title),
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
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type atomEntry struct {
	ID        string       `xml:"id"`
	Title     string       `xml:"title"`
	Links     []atomLink   `xml:"link"`
	Summary   string       `xml:"summary"`
	Content   *atomContent `xml:"content"`
	Author    *atomAuthor  `xml:"author"`
	Published string       `xml:"published"`
	Updated   string       `xml:"updated"`
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

	feed := &ParsedFeed{
		Title:       atom.Title,
		Description: atom.Subtitle,
		URL:         feedURL,
		Items:       make([]FeedItem, 0, len(atom.Entries)),
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

		var pubTime *time.Time
		dateStr := entry.Published
		if dateStr == "" {
			dateStr = entry.Updated
		}
		if dateStr != "" {
			if t, err := parseTime(dateStr); err == nil {
				pubTime = &t
			}
		}

		feed.Items = append(feed.Items, FeedItem{
			GUID:        entry.ID,
			Title:       html.UnescapeString(entry.Title),
			URL:         url,
			Author:      author,
			Content:     content,
			Summary:     entry.Summary,
			PublishedAt: pubTime,
		})
	}

	return feed, nil
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
