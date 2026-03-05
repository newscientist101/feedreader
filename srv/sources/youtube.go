package sources

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/newscientist101/feedreader/srv/safenet"
)

var (
	// Match YouTube URLs for normalization.
	youtubePlaylistRe = regexp.MustCompile(`(?:youtube\.com|youtu\.be)/playlist\?list=([A-Za-z0-9_-]+)`)
	youtubeChannelRe  = regexp.MustCompile(`youtube\.com/channel/(UC[A-Za-z0-9_-]+)`)
	youtubeHandleRe   = regexp.MustCompile(`youtube\.com/@([A-Za-z0-9._-]+)`)
	youtubeCustomRe   = regexp.MustCompile(`youtube\.com/c/([A-Za-z0-9._-]+)`)
	youtubeUserRe     = regexp.MustCompile(`youtube\.com/user/([A-Za-z0-9._-]+)`)

	// Match already-converted feed URLs.
	youtubeFeedRe = regexp.MustCompile(`youtube\.com/feeds/videos\.xml`)

	// isYouTubeURL checks if a URL is any YouTube URL.
	youtubeHostRe = regexp.MustCompile(`(?:^https?://)?(?:www\.)?(?:youtube\.com|youtu\.be)`)
)

// youtubeClient is a safe HTTP client for YouTube requests.
var youtubeClient = safenet.NewSafeClient(15*time.Second, nil)

// YouTubeSource handles YouTube feed URL normalization and auto-naming.
type YouTubeSource struct{}

func (YouTubeSource) Match(rawURL, feedType string) bool {
	if feedType != "youtube" {
		return false
	}
	return youtubeHostRe.MatchString(rawURL) || youtubeFeedRe.MatchString(rawURL)
}

func (YouTubeSource) NormalizeURL(ctx context.Context, rawURL string) (string, error) {
	// Already a feed URL — no normalization needed.
	if youtubeFeedRe.MatchString(rawURL) {
		return rawURL, nil
	}

	// Playlist URL.
	if m := youtubePlaylistRe.FindStringSubmatch(rawURL); m != nil {
		return "https://www.youtube.com/feeds/videos.xml?playlist_id=" + m[1], nil
	}

	// Channel URL (with channel ID).
	if m := youtubeChannelRe.FindStringSubmatch(rawURL); m != nil {
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + m[1], nil
	}

	// @handle, /c/, or /user/ URL — resolve channel ID from page.
	var slug string
	if m := youtubeHandleRe.FindStringSubmatch(rawURL); m != nil {
		slug = "@" + m[1]
	} else if m := youtubeCustomRe.FindStringSubmatch(rawURL); m != nil {
		slug = "c/" + m[1]
	} else if m := youtubeUserRe.FindStringSubmatch(rawURL); m != nil {
		slug = "user/" + m[1]
	}

	if slug != "" {
		channelID, err := resolveYouTubeChannelID(ctx, slug)
		if err != nil {
			return "", fmt.Errorf("resolve YouTube channel %q: %w", slug, err)
		}
		return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID, nil
	}

	return rawURL, nil
}

func (YouTubeSource) ResolveName(ctx context.Context, feedURL, _ string) string {
	if !youtubeFeedRe.MatchString(feedURL) {
		return ""
	}
	return fetchYouTubeFeedTitle(ctx, feedURL)
}

// FeedType returns "rss" — YouTube Atom feeds are handled by the standard
// RSS/Atom parser, so we override the "youtube" UI type to "rss" for storage.
func (YouTubeSource) FeedType() string { return "rss" }

// resolveYouTubeChannelID fetches a YouTube page and extracts the channel ID
// from the canonical URL or page metadata.
func resolveYouTubeChannelID(ctx context.Context, slug string) (string, error) {
	pageURL := "https://www.youtube.com/" + slug
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedReader/1.0)")

	resp, err := youtubeClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read up to 512KB — the channel ID meta tag is in the <head>.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", err
	}

	// Look for <link rel="canonical" href="https://www.youtube.com/channel/UCxxx">
	// or <meta itemprop="channelId" content="UCxxx">
	html := string(body)

	// Try canonical link.
	if m := regexp.MustCompile(`<link[^>]+rel="canonical"[^>]+href="https://www\.youtube\.com/channel/(UC[A-Za-z0-9_-]+)"`).FindStringSubmatch(html); m != nil {
		return m[1], nil
	}
	// Try alternate attribute order.
	if m := regexp.MustCompile(`<link[^>]+href="https://www\.youtube\.com/channel/(UC[A-Za-z0-9_-]+)"[^>]+rel="canonical"`).FindStringSubmatch(html); m != nil {
		return m[1], nil
	}

	// Try meta tag.
	if m := regexp.MustCompile(`<meta[^>]+itemprop="channelId"[^>]+content="(UC[A-Za-z0-9_-]+)"`).FindStringSubmatch(html); m != nil {
		return m[1], nil
	}
	if m := regexp.MustCompile(`<meta[^>]+content="(UC[A-Za-z0-9_-]+)"[^>]+itemprop="channelId"`).FindStringSubmatch(html); m != nil {
		return m[1], nil
	}

	// Try browseid in the page's initial data JSON.
	if m := regexp.MustCompile(`"browseId"\s*:\s*"(UC[A-Za-z0-9_-]+)"`).FindStringSubmatch(html); m != nil {
		return m[1], nil
	}

	return "", fmt.Errorf("channel ID not found in page for %s", slug)
}

// fetchYouTubeFeedTitle fetches a YouTube Atom feed and returns its <title>.
func fetchYouTubeFeedTitle(ctx context.Context, feedURL string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, http.NoBody)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FeedReader/1.0)")

	resp, err := youtubeClient.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Parse enough of the Atom feed to get the title.
	var feed struct {
		Title string `xml:"title"`
	}
	decoder := xml.NewDecoder(io.LimitReader(resp.Body, 64*1024))
	if err := decoder.Decode(&feed); err != nil {
		return ""
	}
	return strings.TrimSpace(feed.Title)
}

// SetYouTubeClient replaces the HTTP client used for YouTube requests (for testing).
func SetYouTubeClient(c *http.Client) {
	youtubeClient = c
}

// ParseYouTubeURL extracts the type and ID from a YouTube URL.
// Returns (kind, id, ok) where kind is "playlist", "channel", "handle",
// "custom", "user", or "feed".
func ParseYouTubeURL(rawURL string) (kind, id string, ok bool) {
	if m := youtubePlaylistRe.FindStringSubmatch(rawURL); m != nil {
		return "playlist", m[1], true
	}
	if m := youtubeChannelRe.FindStringSubmatch(rawURL); m != nil {
		return "channel", m[1], true
	}
	if m := youtubeHandleRe.FindStringSubmatch(rawURL); m != nil {
		return "handle", m[1], true
	}
	if m := youtubeCustomRe.FindStringSubmatch(rawURL); m != nil {
		return "custom", m[1], true
	}
	if m := youtubeUserRe.FindStringSubmatch(rawURL); m != nil {
		return "user", m[1], true
	}
	if youtubeFeedRe.MatchString(rawURL) {
		if u, err := url.Parse(rawURL); err == nil {
			if pid := u.Query().Get("playlist_id"); pid != "" {
				return "feed", pid, true
			}
			if cid := u.Query().Get("channel_id"); cid != "" {
				return "feed", cid, true
			}
		}
		return "feed", "", true
	}
	return "", "", false
}
