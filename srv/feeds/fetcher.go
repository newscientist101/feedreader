package feeds

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"time"

	"github.com/newscientist101/feedreader/srv/youtube"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/huggingface"
	"github.com/newscientist101/feedreader/srv/safenet"
	"github.com/newscientist101/feedreader/srv/scrapers"
)

// BrowserUserAgent is a Chrome-like User-Agent string used for feed fetching
// and scraping to avoid bot detection on sites behind Cloudflare etc.
const BrowserUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Fetcher handles fetching and updating feeds
type Fetcher struct {
	DB            *sql.DB
	Client        *http.Client
	ScraperRunner *scrapers.Runner
	OnFeedFetched func(ctx context.Context, feedID int64) // called after articles are inserted
	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
}

// NewFetcher creates a new feed fetcher
func NewFetcher(db *sql.DB, scraperRunner *scrapers.Runner) *Fetcher {
	// Use a custom transport with TLS config to avoid Cloudflare bot detection
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
	}
	return &Fetcher{
		DB:            db,
		Client:        safenet.NewSafeClient(30*time.Second, transport),
		ScraperRunner: scraperRunner,
	}
}

// Start begins the background fetch loop
func (f *Fetcher) Start(interval time.Duration) {
	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return
	}
	f.running = true
	f.stopCh = make(chan struct{})
	f.mu.Unlock()

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Initial fetch
		f.FetchAll(context.Background())

		for {
			select {
			case <-f.stopCh:
				return
			case <-ticker.C:
				f.FetchAll(context.Background())
			}
		}
	}()
}

// Stop stops the background fetch loop
func (f *Fetcher) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.running {
		close(f.stopCh)
		f.running = false
	}
}

// FetchAll fetches all feeds that need updating
func (f *Fetcher) FetchAll(ctx context.Context) {
	q := dbgen.New(f.DB)
	feeds, err := q.ListFeedsToFetch(ctx)
	if err != nil {
		slog.Error("list feeds to fetch", "error", err)
		return
	}

	for i := range feeds {
		if err := f.FetchFeed(ctx, &feeds[i]); err != nil {
			slog.Warn("fetch feed", "feed_id", feeds[i].ID, "url", feeds[i].Url, "error", err)
		}
	}
}

// FetchFeed fetches a single feed
func (f *Fetcher) FetchFeed(ctx context.Context, feed *dbgen.Feed) error {
	slog.Debug("starting feed fetch", "feed_id", feed.ID, "url", feed.Url, "type", feed.FeedType)
	q := dbgen.New(f.DB)
	now := time.Now()

	var items []FeedItem
	var siteURL string
	var fetchErr error

	switch feed.FeedType {
	case "rss", "atom":
		items, siteURL, fetchErr = f.fetchRSSFeed(ctx, feed.Url)
	case "scraper":
		if feed.ScraperModule != nil && *feed.ScraperModule != "" {
			items, fetchErr = f.fetchWithScraper(ctx, feed)
		} else {
			fetchErr = fmt.Errorf("scraper module not configured")
		}
	case "huggingface":
		items, fetchErr = f.fetchHuggingFace(ctx, feed)
	case "youtube-playlist":
		items, fetchErr = f.fetchYouTubePlaylist(ctx, feed)
	default:
		fetchErr = fmt.Errorf("unknown feed type: %s", feed.FeedType)
	}

	// Update feed status: increment error counter on failure, reset on success.
	if fetchErr != nil {
		errMsg := fetchErr.Error()
		if err := q.IncrementFeedErrors(ctx, dbgen.IncrementFeedErrorsParams{
			LastError:     &errMsg,
			LastFetchedAt: &now,
			ID:            feed.ID,
		}); err != nil {
			slog.Warn("increment feed errors", "error", err)
		}
	} else {
		if err := q.ResetFeedErrors(ctx, dbgen.ResetFeedErrorsParams{
			LastFetchedAt: &now,
			ID:            feed.ID,
		}); err != nil {
			slog.Warn("reset feed errors", "error", err)
		}
	}

	if fetchErr != nil {
		return fetchErr
	}

	// Persist the site URL discovered from the feed
	if siteURL != "" && siteURL != feed.SiteUrl {
		if err := q.UpdateFeedSiteURL(ctx, dbgen.UpdateFeedSiteURLParams{SiteUrl: siteURL, ID: feed.ID}); err != nil {
			slog.Warn("update feed site_url", "error", err)
		}
	}

	// Store items
	inserted := 0
	for i, item := range items {
		if item.GUID == "" {
			slog.Warn("skipping item with empty GUID", "feed_id", feed.ID, "title", item.Title)
			continue
		}
		if i == 0 {
			slog.Info("first item", "guid", item.GUID, "title", item.Title, "url", item.URL)
		}
		// Skip GUIDs that were already seen and hard-deleted by retention cleanup.
		// This prevents re-insertion of articles that were intentionally removed.
		seen, err := q.IsGuidSeen(ctx, dbgen.IsGuidSeenParams{FeedID: feed.ID, Guid: item.GUID})
		if err != nil {
			slog.Warn("seen_guids check failed", "error", err, "guid", item.GUID, "feed_id", feed.ID)
		} else if seen > 0 {
			continue
		}
		// Normalize time to UTC for consistent storage
		var pubAt *time.Time
		if item.PublishedAt != nil {
			utc := item.PublishedAt.UTC()
			pubAt = &utc
		}
		_, err = q.CreateArticle(ctx, dbgen.CreateArticleParams{
			FeedID:      feed.ID,
			Guid:        item.GUID,
			Title:       item.Title,
			Url:         strPtr(item.URL),
			Author:      strPtr(item.Author),
			Content:     strPtr(item.Content),
			Summary:     strPtr(item.Summary),
			ImageUrl:    strPtr(item.ImageURL),
			PublishedAt: pubAt,
		})
		if err != nil {
			slog.Warn("create article failed", "error", err, "guid", item.GUID, "feed_id", feed.ID)
		} else {
			inserted++
		}
	}

	slog.Info("fetched feed", "feed_id", feed.ID, "name", feed.Name, "items", len(items), "inserted", inserted)

	if inserted > 0 && f.OnFeedFetched != nil {
		f.OnFeedFetched(ctx, feed.ID)
	}

	return nil
}

func (f *Fetcher) fetchRSSFeed(ctx context.Context, url string) ([]FeedItem, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, "", err
	}
	// Use browser-like headers to avoid bot detection (e.g., Cloudflare)
	req.Header.Set("User-Agent", BrowserUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d %s: %s", resp.StatusCode, http.StatusText(resp.StatusCode), httpErrorDescription(resp.StatusCode))
	}

	// Handle gzip-encoded responses
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, "", fmt.Errorf("failed to decompress gzip response: %w", err)
		}
		defer func() { _ = gzReader.Close() }()
		reader = gzReader
	}

	feed, err := Parse(reader)
	if err != nil {
		return nil, "", err
	}

	return feed.Items, feed.URL, nil
}

func (f *Fetcher) fetchWithScraper(ctx context.Context, feed *dbgen.Feed) ([]FeedItem, error) {
	if f.ScraperRunner == nil {
		return nil, fmt.Errorf("scraper runner not available")
	}

	q := dbgen.New(f.DB)
	module, err := q.GetScraperModuleInternal(ctx, *feed.ScraperModule)
	if err != nil {
		return nil, fmt.Errorf("get scraper module: %w", err)
	}

	// Fetch the page content
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.Url, http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", BrowserUserAgent)

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	htmlContent := buf.String()

	var config string
	if feed.ScraperConfig != nil {
		config = *feed.ScraperConfig
	}

	scraperItems, err := f.ScraperRunner.Run(ctx, module.Script, htmlContent, feed.Url, config)
	if err != nil {
		return nil, err
	}

	// Convert scraper items to feed items
	items := make([]FeedItem, len(scraperItems))
	for i, si := range scraperItems {
		items[i] = FeedItem{
			GUID:        si.GUID,
			Title:       si.Title,
			URL:         si.URL,
			Author:      si.Author,
			Content:     si.Content,
			Summary:     si.Summary,
			ImageURL:    si.ImageURL,
			PublishedAt: si.PublishedAt,
		}
	}
	return items, nil
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func (f *Fetcher) fetchHuggingFace(ctx context.Context, feed *dbgen.Feed) ([]FeedItem, error) {
	if feed.ScraperConfig == nil || *feed.ScraperConfig == "" {
		return nil, fmt.Errorf("huggingface config not set")
	}

	var config huggingface.FeedConfig
	if err := json.Unmarshal([]byte(*feed.ScraperConfig), &config); err != nil {
		return nil, fmt.Errorf("parse huggingface config: %w", err)
	}

	client := huggingface.NewClient("") // No auth token for now
	hfItems, err := client.Fetch(ctx, &config)
	if err != nil {
		return nil, fmt.Errorf("fetch from huggingface: %w", err)
	}

	items := make([]FeedItem, len(hfItems))
	for i := range hfItems {
		items[i] = FeedItem{
			GUID:        hfItems[i].GUID,
			Title:       hfItems[i].Title,
			URL:         hfItems[i].URL,
			Author:      hfItems[i].Author,
			Summary:     hfItems[i].Summary,
			ImageURL:    hfItems[i].ImageURL,
			PublishedAt: hfItems[i].PublishedAt,
		}
	}
	return items, nil
}

// YouTubePlaylistConfig is the JSON config stored in scraper_config for youtube-playlist feeds.
type YouTubePlaylistConfig struct {
	PlaylistID     string `json:"playlist_id"`
	LastKnownCount int    `json:"last_known_count"`
}

func (f *Fetcher) fetchYouTubePlaylist(ctx context.Context, feed *dbgen.Feed) ([]FeedItem, error) {
	if feed.ScraperConfig == nil || *feed.ScraperConfig == "" {
		return nil, fmt.Errorf("youtube-playlist config not set")
	}

	var config YouTubePlaylistConfig
	if err := json.Unmarshal([]byte(*feed.ScraperConfig), &config); err != nil {
		return nil, fmt.Errorf("parse youtube-playlist config: %w", err)
	}
	if config.PlaylistID == "" {
		return nil, fmt.Errorf("playlist_id not set in config")
	}

	// Look up user's YouTube API key.
	if feed.UserID == nil {
		return nil, fmt.Errorf("feed has no user_id")
	}
	q := dbgen.New(f.DB)
	apiKey, err := q.GetUserSetting(ctx, dbgen.GetUserSettingParams{
		UserID: *feed.UserID,
		Key:    "youtubeApiKey",
	})
	if err != nil || apiKey == "" {
		// Fall back to RSS if no API key configured.
		slog.Warn("no YouTube API key configured, falling back to RSS",
			"feed_id", feed.ID, "playlist_id", config.PlaylistID)
		items, _, fetchErr := f.fetchRSSFeed(ctx,
			"https://www.youtube.com/feeds/videos.xml?playlist_id="+config.PlaylistID)
		return items, fetchErr
	}

	client := youtube.NewClient(apiKey)
	ytItems, totalResults, err := client.FetchPlaylistItems(ctx, config.PlaylistID, 0)
	if err != nil {
		return nil, fmt.Errorf("youtube API: %w", err)
	}

	// Only return items past the last known count (newly appended items).
	var newYTItems []youtube.Item
	if config.LastKnownCount > 0 && config.LastKnownCount < len(ytItems) {
		newYTItems = ytItems[config.LastKnownCount:]
	} else if config.LastKnownCount == 0 {
		// First fetch: return all items.
		newYTItems = ytItems
	}
	// If totalResults <= lastKnownCount, no new items (items may have been removed).

	// Convert youtube.Item to FeedItem.
	newItems := make([]FeedItem, len(newYTItems))
	for i, yt := range newYTItems {
		newItems[i] = FeedItem{
			GUID:        yt.GUID,
			Title:       yt.Title,
			URL:         yt.URL,
			Author:      yt.Author,
			Content:     yt.Content,
			ImageURL:    yt.ImageURL,
			PublishedAt: yt.PublishedAt,
		}
	}

	// Persist the new count for next fetch.
	if totalResults != config.LastKnownCount {
		config.LastKnownCount = totalResults
		cfgBytes, _ := json.Marshal(config)
		cfgStr := string(cfgBytes)
		if err := q.UpdateFeedScraperConfig(ctx, dbgen.UpdateFeedScraperConfigParams{
			ScraperConfig: &cfgStr,
			ID:            feed.ID,
		}); err != nil {
			slog.Warn("update youtube-playlist config", "error", err, "feed_id", feed.ID)
		}
	}

	slog.Info("youtube-playlist fetch",
		"feed_id", feed.ID,
		"playlist_id", config.PlaylistID,
		"total", totalResults,
		"new", len(newItems))

	return newItems, nil
}

// httpErrorDescription returns a human-readable description of HTTP error codes
func httpErrorDescription(code int) string {
	switch code {
	case 400:
		return "The request was malformed or invalid"
	case 401:
		return "Authentication is required to access this feed"
	case 403:
		return "The server refused to provide this feed"
	case 404:
		return "The feed URL was not found on the server"
	case 405:
		return "The request method is not allowed for this feed"
	case 408:
		return "The server timed out waiting for the request"
	case 410:
		return "The feed has been permanently removed"
	case 429:
		return "Too many requests; the server is rate limiting"
	case 500:
		return "The server encountered an internal error"
	case 502:
		return "The server received an invalid response from upstream"
	case 503:
		return "The server is temporarily unavailable"
	case 504:
		return "The server timed out waiting for an upstream response"
	default:
		if code >= 400 && code < 500 {
			return "The request could not be completed"
		}
		if code >= 500 {
			return "The server encountered an error"
		}
		return "An unexpected error occurred"
	}
}
