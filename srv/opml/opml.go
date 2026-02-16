package opml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// OPML represents an OPML document
type OPML struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    Head     `xml:"head"`
	Body    Body     `xml:"body"`
}

// Head contains OPML metadata
type Head struct {
	Title       string `xml:"title,omitempty"`
	DateCreated string `xml:"dateCreated,omitempty"`
	OwnerName   string `xml:"ownerName,omitempty"`
}

// Body contains the outline elements
type Body struct {
	Outlines []Outline `xml:"outline"`
}

// Outline represents a feed or folder
type Outline struct {
	Text     string    `xml:"text,attr"`
	Title    string    `xml:"title,attr,omitempty"`
	Type     string    `xml:"type,attr,omitempty"`
	XMLURL   string    `xml:"xmlUrl,attr,omitempty"`
	HTMLURL  string    `xml:"htmlUrl,attr,omitempty"`
	Outlines []Outline `xml:"outline,omitempty"`
}

// Feed represents an imported feed
type Feed struct {
	Name     string
	URL      string
	SiteURL  string
	Category string
}

// Parse reads an OPML file and returns the feeds
func Parse(r io.Reader) ([]Feed, error) {
	var opml OPML
	if err := xml.NewDecoder(r).Decode(&opml); err != nil {
		return nil, fmt.Errorf("parse OPML: %w", err)
	}

	var feeds []Feed
	for _, outline := range opml.Body.Outlines {
		feeds = append(feeds, extractFeeds(&outline, "")...)
	}
	return feeds, nil
}

func extractFeeds(outline *Outline, category string) []Feed {
	var feeds []Feed

	// If this outline has a feed URL, it's a feed
	if outline.XMLURL != "" {
		name := outline.Title
		if name == "" {
			name = outline.Text
		}
		feeds = append(feeds, Feed{
			Name:     name,
			URL:      outline.XMLURL,
			SiteURL:  outline.HTMLURL,
			Category: category,
		})
	}

	// Process child outlines (folder contents)
	folderName := category
	if outline.XMLURL == "" && len(outline.Outlines) > 0 {
		// This is a folder
		folderName = outline.Text
		if folderName == "" {
			folderName = outline.Title
		}
	}

	for _, child := range outline.Outlines {
		feeds = append(feeds, extractFeeds(&child, folderName)...)
	}

	return feeds
}

// Export creates an OPML document from feeds organized by category
type ExportFeed struct {
	Name     string
	URL      string
	SiteURL  string
	Category string
}

// Export generates an OPML document
func Export(feeds []ExportFeed, title string) ([]byte, error) {
	opml := OPML{
		Version: "2.0",
		Head: Head{
			Title:       title,
			DateCreated: time.Now().Format(time.RFC1123),
		},
	}

	// Group feeds by category
	categories := make(map[string][]ExportFeed)
	var uncategorized []ExportFeed

	for _, feed := range feeds {
		if feed.Category == "" {
			uncategorized = append(uncategorized, feed)
		} else {
			categories[feed.Category] = append(categories[feed.Category], feed)
		}
	}

	// Add uncategorized feeds at top level
	for _, feed := range uncategorized {
		opml.Body.Outlines = append(opml.Body.Outlines, Outline{
			Text:    feed.Name,
			Title:   feed.Name,
			Type:    "rss",
			XMLURL:  feed.URL,
			HTMLURL: feed.SiteURL,
		})
	}

	// Add categorized feeds in folders
	for catName, catFeeds := range categories {
		folder := Outline{
			Text:  catName,
			Title: catName,
		}
		for _, feed := range catFeeds {
			folder.Outlines = append(folder.Outlines, Outline{
				Text:    feed.Name,
				Title:   feed.Name,
				Type:    "rss",
				XMLURL:  feed.URL,
				HTMLURL: feed.SiteURL,
			})
		}
		opml.Body.Outlines = append(opml.Body.Outlines, folder)
	}

	output, err := xml.MarshalIndent(opml, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
}

// Validate checks if the content looks like OPML
func Validate(content string) bool {
	return strings.Contains(content, "<opml") || strings.Contains(content, "<OPML")
}
