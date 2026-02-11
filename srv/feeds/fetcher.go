package feeds

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/scrapers"
)

// Fetcher handles fetching and updating feeds
type Fetcher struct {
	DB            *sql.DB
	Client        *http.Client
	ScraperRunner *scrapers.Runner
	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
}

// NewFetcher creates a new feed fetcher
func NewFetcher(db *sql.DB, scraperRunner *scrapers.Runner) *Fetcher {
	return &Fetcher{
		DB: db,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
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

	for _, feed := range feeds {
		if err := f.FetchFeed(ctx, feed); err != nil {
			slog.Warn("fetch feed", "feed_id", feed.ID, "url", feed.Url, "error", err)
		}
	}
}

// FetchFeed fetches a single feed
func (f *Fetcher) FetchFeed(ctx context.Context, feed dbgen.Feed) error {
	q := dbgen.New(f.DB)
	now := time.Now()

	var items []FeedItem
	var fetchErr error

	switch feed.FeedType {
	case "rss", "atom":
		items, fetchErr = f.fetchRSSFeed(ctx, feed.Url)
	case "scraper":
		if feed.ScraperModule != nil && *feed.ScraperModule != "" {
			items, fetchErr = f.fetchWithScraper(ctx, feed)
		} else {
			fetchErr = fmt.Errorf("scraper module not configured")
		}
	default:
		fetchErr = fmt.Errorf("unknown feed type: %s", feed.FeedType)
	}

	var errStr *string
	if fetchErr != nil {
		s := fetchErr.Error()
		errStr = &s
	}

	// Update feed status
	if err := q.UpdateFeedLastFetched(ctx, dbgen.UpdateFeedLastFetchedParams{
		LastFetchedAt: &now,
		LastError:     errStr,
		ID:            feed.ID,
	}); err != nil {
		slog.Warn("update feed status", "error", err)
	}

	if fetchErr != nil {
		return fetchErr
	}

	// Store items
	for _, item := range items {
		if item.GUID == "" {
			continue
		}
		_, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
			FeedID:      feed.ID,
			Guid:        item.GUID,
			Title:       item.Title,
			Url:         strPtr(item.URL),
			Author:      strPtr(item.Author),
			Content:     strPtr(item.Content),
			Summary:     strPtr(item.Summary),
			ImageUrl:    strPtr(item.ImageURL),
			PublishedAt: item.PublishedAt,
		})
		if err != nil {
			slog.Debug("create article", "error", err, "guid", item.GUID)
		}
	}

	slog.Info("fetched feed", "feed_id", feed.ID, "name", feed.Name, "items", len(items))
	return nil
}

func (f *Fetcher) fetchRSSFeed(ctx context.Context, url string) ([]FeedItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "FeedReader/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	feed, err := Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	return feed.Items, nil
}

func (f *Fetcher) fetchWithScraper(ctx context.Context, feed dbgen.Feed) ([]FeedItem, error) {
	if f.ScraperRunner == nil {
		return nil, fmt.Errorf("scraper runner not available")
	}

	q := dbgen.New(f.DB)
	module, err := q.GetScraperModuleByName(ctx, *feed.ScraperModule)
	if err != nil {
		return nil, fmt.Errorf("get scraper module: %w", err)
	}

	// Fetch the page content
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feed.Url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
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
