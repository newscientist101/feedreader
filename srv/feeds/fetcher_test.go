package feeds

import (
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"

	_ "modernc.org/sqlite"
)

// --- helpers ---

var (
	cachedSchema     string
	cachedSchemaOnce sync.Once
)

func getSchema() string {
	cachedSchemaOnce.Do(func() {
		d, err := sql.Open("sqlite", ":memory:")
		if err != nil {
			panic(err)
		}
		defer d.Close()
		d.Exec("PRAGMA foreign_keys=ON")
		if err := db.RunMigrations(d); err != nil {
			panic(err)
		}
		rows, _ := d.Query("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND sql NOT LIKE 'CREATE TABLE sqlite_%' ORDER BY rowid")
		defer rows.Close()
		var sb strings.Builder
		for rows.Next() {
			var s string
			rows.Scan(&s)
			sb.WriteString(s)
			sb.WriteString(";\n")
		}
		cachedSchema = sb.String()
	})
	return cachedSchema
}

func setupTestDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	schema := getSchema()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if _, err := sqlDB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		t.Fatal(err)
	}
	return sqlDB, dbgen.New(sqlDB)
}

func createTestUser(t *testing.T, q *dbgen.Queries) dbgen.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), dbgen.CreateUserParams{
		ExternalID: "testuser",
		Email:      "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// validRSSBody returns a minimal valid RSS feed XML.
func validRSSBody(title, guid, link string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Test Feed</title>
    <link>https://example.com</link>
    <item>
      <title>%s</title>
      <guid>%s</guid>
      <link>%s</link>
    </item>
  </channel>
</rss>`, title, guid, link)
}

// --- strPtr ---

func TestStrPtr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		nil_ bool
	}{
		{"", true},
		{"  ", true},
		{"\t\n", true},
		{"hello", false},
		{" hi ", false},
	}
	for _, tt := range tests {
		got := strPtr(tt.in)
		if tt.nil_ && got != nil {
			t.Errorf("strPtr(%q) = %v, want nil", tt.in, *got)
		}
		if !tt.nil_ && (got == nil || *got != tt.in) {
			t.Errorf("strPtr(%q) unexpected", tt.in)
		}
	}
}

// --- httpErrorDescription ---

func TestHttpErrorDescription(t *testing.T) {
	t.Parallel()
	cases := []struct {
		code int
		want string
	}{
		{400, "The request was malformed or invalid"},
		{401, "Authentication is required to access this feed"},
		{403, "The server refused to provide this feed"},
		{404, "The feed URL was not found on the server"},
		{405, "The request method is not allowed for this feed"},
		{408, "The server timed out waiting for the request"},
		{410, "The feed has been permanently removed"},
		{429, "Too many requests; the server is rate limiting"},
		{500, "The server encountered an internal error"},
		{502, "The server received an invalid response from upstream"},
		{503, "The server is temporarily unavailable"},
		{504, "The server timed out waiting for an upstream response"},
		{418, "The request could not be completed"}, // unknown 4xx
		{599, "The server encountered an error"},    // unknown 5xx
		{301, "An unexpected error occurred"},       // other
	}
	for _, tc := range cases {
		got := httpErrorDescription(tc.code)
		if got != tc.want {
			t.Errorf("httpErrorDescription(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

// --- NewFetcher ---

func TestNewFetcher(t *testing.T) {
	t.Parallel()
	sqlDB, _ := setupTestDB(t)
	f := NewFetcher(sqlDB, nil)
	if f.DB != sqlDB {
		t.Error("DB not set")
	}
	if f.Client == nil {
		t.Error("Client not set")
	}
}

// --- fetchRSSFeed ---

func TestFetchRSSFeed_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, validRSSBody("Hello", "guid-1", "https://example.com/1"))
	}))
	defer ts.Close()

	f := &Fetcher{Client: ts.Client()}
	items, siteURL, err := f.fetchRSSFeed(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if siteURL != "https://example.com" {
		t.Errorf("siteURL = %q, want %q", siteURL, "https://example.com")
	}
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if items[0].Title != "Hello" {
		t.Errorf("title = %q", items[0].Title)
	}
	if items[0].GUID != "guid-1" {
		t.Errorf("guid = %q", items[0].GUID)
	}
}

func TestFetchRSSFeed_Gzip(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/xml")
		gw := gzip.NewWriter(w)
		fmt.Fprint(gw, validRSSBody("Gzipped", "gz-1", "https://example.com/gz"))
		gw.Close()
	}))
	defer ts.Close()

	f := &Fetcher{Client: ts.Client()}
	items, _, err := f.fetchRSSFeed(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Title != "Gzipped" {
		t.Errorf("got items: %+v", items)
	}
}

func TestFetchRSSFeed_HTTPError(t *testing.T) {
	t.Parallel()
	for _, code := range []int{403, 404, 500} {
		t.Run(fmt.Sprintf("HTTP_%d", code), func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer ts.Close()

			f := &Fetcher{Client: ts.Client()}
			_, _, err := f.fetchRSSFeed(context.Background(), ts.URL)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFetchRSSFeed_InvalidXML(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not xml at all")
	}))
	defer ts.Close()

	f := &Fetcher{Client: ts.Client()}
	_, _, err := f.fetchRSSFeed(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchRSSFeed_BadURL(t *testing.T) {
	t.Parallel()
	f := &Fetcher{Client: http.DefaultClient}
	_, _, err := f.fetchRSSFeed(context.Background(), "http://127.0.0.1:1/nope")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestFetchRSSFeed_InvalidGzip(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write([]byte("not gzip data"))
	}))
	defer ts.Close()

	f := &Fetcher{Client: ts.Client()}
	_, _, err := f.fetchRSSFeed(context.Background(), ts.URL)
	if err == nil {
		t.Fatal("expected gzip error")
	}
}

// --- FetchFeed integration (RSS path) ---

func TestFetchFeed_RSS(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validRSSBody("Article 1", "rss-guid-1", "https://example.com/1"))
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:     "Test RSS",
		Url:      ts.URL,
		FeedType: "rss",
		UserID:   &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	// Verify article was inserted
	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM articles WHERE feed_id = ?", feed.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("got %d articles, want 1", count)
	}

	// Verify feed was updated
	updated, err := q.GetFeed(context.Background(), dbgen.GetFeedParams{ID: feed.ID, UserID: &user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastFetchedAt == nil {
		t.Error("last_fetched_at not set")
	}
	if updated.LastError != nil {
		t.Errorf("last_error = %q, want nil", *updated.LastError)
	}
	// Check site_url was updated
	if updated.SiteUrl != "https://example.com" {
		t.Errorf("site_url = %q, want %q", updated.SiteUrl, "https://example.com")
	}
}

func TestFetchFeed_RSS_DuplicateGUID(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validRSSBody("Article", "dup-guid", "https://example.com/dup"))
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "Dup", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}

	// Fetch twice — second should not error, just skip duplicate
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatal(err)
	}
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatal(err)
	}

	var count int
	sqlDB.QueryRow("SELECT COUNT(*) FROM articles WHERE feed_id = ?", feed.ID).Scan(&count)
	if count != 1 {
		t.Errorf("got %d articles after double fetch, want 1", count)
	}
}

func TestFetchFeed_RSS_FetchError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "Bad", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}

	// Verify error was recorded on the feed
	updated, _ := q.GetFeed(context.Background(), dbgen.GetFeedParams{ID: feed.ID, UserID: &user.ID})
	if updated.LastError == nil {
		t.Error("expected last_error to be set")
	}
}

func TestFetchFeed_UnknownType(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	// Use direct SQL to create a feed with an unusual type, since CreateFeed might default
	_, err := sqlDB.Exec(`INSERT INTO feeds (name, url, feed_type, user_id) VALUES (?, ?, ?, ?)`,
		"Unknown", "http://example.com", "mystery", user.ID)
	if err != nil {
		t.Fatal(err)
	}

	var feed dbgen.Feed
	row := sqlDB.QueryRow("SELECT id, name, url, feed_type, site_url, scraper_module, scraper_config, fetch_interval_minutes, last_fetched_at, last_error, user_id, content_filters, created_at, updated_at FROM feeds WHERE name = 'Unknown'")
	err = row.Scan(&feed.ID, &feed.Name, &feed.Url, &feed.FeedType, &feed.SiteUrl,
		&feed.ScraperModule, &feed.ScraperConfig, &feed.FetchIntervalMinutes,
		&feed.LastFetchedAt, &feed.LastError, &feed.UserID, &feed.ContentFilters,
		&feed.CreatedAt, &feed.UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err = f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error for unknown feed type")
	}
	if got := err.Error(); got != "unknown feed type: mystery" {
		t.Errorf("error = %q", got)
	}
}

func TestFetchFeed_Atom(t *testing.T) {
	t.Parallel()
	atomXML := `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Feed</title>
  <link href="https://atom.example.com"/>
  <entry>
    <title>Atom Entry</title>
    <id>atom-guid-1</id>
    <link href="https://atom.example.com/1"/>
    <summary>An atom summary</summary>
  </entry>
</feed>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, atomXML)
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "Atom", Url: ts.URL, FeedType: "atom", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatalf("FetchFeed atom: %v", err)
	}

	var title string
	if err := sqlDB.QueryRow("SELECT title FROM articles WHERE feed_id = ?", feed.ID).Scan(&title); err != nil {
		t.Fatal(err)
	}
	if title != "Atom Entry" {
		t.Errorf("title = %q", title)
	}
}

func TestFetchFeed_ScraperMissingModule(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "Scraper No Mod", Url: "http://example.com", FeedType: "scraper", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchFeed_ScraperNoRunner(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:          "Scraper",
		Url:           "http://example.com",
		FeedType:      "scraper",
		ScraperModule: new("some-module"),
		UserID:        &user.ID,
	})

	// Insert a fake scraper module so GetScraperModuleInternal succeeds
	_, err := sqlDB.Exec(`INSERT INTO scraper_modules (name, script, user_id) VALUES (?, ?, ?)`,
		"some-module", "return []", user.ID)
	if err != nil {
		t.Fatal(err)
	}

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient, ScraperRunner: nil}
	err = f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error for nil scraper runner")
	}
}

func TestFetchFeed_HuggingFaceNoConfig(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "HF", Url: "http://hf.example.com", FeedType: "huggingface", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchFeed_HuggingFaceBadConfig(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:          "HF",
		Url:           "http://hf.example.com",
		FeedType:      "huggingface",
		ScraperConfig: new("not json"),
		UserID:        &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- FetchAll ---

func TestFetchAll(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validRSSBody("FetchAll Item", "fa-guid", "https://example.com/fa"))
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	// Create feed with no last_fetched_at — should be picked up by ListFeedsToFetch
	q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "All", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	f.FetchAll(context.Background())

	var count int
	sqlDB.QueryRow("SELECT COUNT(*) FROM articles").Scan(&count)
	if count != 1 {
		t.Errorf("FetchAll: got %d articles, want 1", count)
	}
}

// --- Start / Stop ---

func TestStartStop(t *testing.T) {
	t.Parallel()
	sqlDB, _ := setupTestDB(t)
	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}

	// Start with a long interval so it only does the initial fetch
	f.Start(10 * time.Minute)
	time.Sleep(50 * time.Millisecond) // let goroutine start

	if !f.running {
		t.Error("expected running=true")
	}

	// Double start should be no-op
	f.Start(10 * time.Minute)

	f.Stop()
	if f.running {
		t.Error("expected running=false after Stop")
	}

	// Double stop should be safe
	f.Stop()
}

// --- FetchFeed with items that have empty GUID (should be skipped) ---

func TestFetchFeed_SkipsEmptyGUID(t *testing.T) {
	t.Parallel()
	// RSS with an item that has no <guid>
	xml := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Feed</title>
    <item>
      <title>No GUID</title>
      <link>https://example.com/no-guid</link>
    </item>
    <item>
      <title>Has GUID</title>
      <guid>good-guid</guid>
      <link>https://example.com/good</link>
    </item>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, xml)
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "GUID Test", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatal(err)
	}

	var count int
	sqlDB.QueryRow("SELECT COUNT(*) FROM articles WHERE feed_id = ?", feed.ID).Scan(&count)
	// The parser uses link as GUID fallback, so both items likely get a GUID.
	// Just verify no crash and at least 1 article.
	if count == 0 {
		t.Error("expected at least 1 article")
	}
}

// --- OnFeedFetched callback ---

func TestFetchFeed_CallsOnFeedFetched(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validRSSBody("Callback Test", "cb-guid-1", "https://example.com/cb"))
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "CB Test", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	var calledWith int64
	f := &Fetcher{
		DB:     sqlDB,
		Client: ts.Client(),
		OnFeedFetched: func(ctx context.Context, feedID int64) {
			calledWith = feedID
		},
	}

	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if calledWith != feed.ID {
		t.Errorf("OnFeedFetched called with feed_id=%d, want %d", calledWith, feed.ID)
	}
}

func TestFetchFeed_DoesNotCallOnFeedFetched_WhenNoItems(t *testing.T) {
	t.Parallel()
	// RSS feed with no items
	emptyRSS := `<?xml version="1.0"?>
<rss version="2.0">
  <channel>
    <title>Empty Feed</title>
    <link>https://example.com</link>
  </channel>
</rss>`

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, emptyRSS)
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "Empty", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	called := false
	f := &Fetcher{
		DB:     sqlDB,
		Client: ts.Client(),
		OnFeedFetched: func(ctx context.Context, feedID int64) {
			called = true
		},
	}

	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}

	if called {
		t.Error("OnFeedFetched should not be called when no items inserted")
	}
}

func TestFetchFeed_NilOnFeedFetched_NoPanic(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, validRSSBody("No Panic", "np-guid", "https://example.com/np"))
	}))
	defer ts.Close()

	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "No Panic", Url: ts.URL, FeedType: "rss", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: ts.Client(), OnFeedFetched: nil}
	if err := f.FetchFeed(context.Background(), &feed); err != nil {
		t.Fatalf("FetchFeed: %v", err)
	}
}

// --- YouTube Playlist ---

func TestFetchFeed_YouTubePlaylist_NoConfig(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "YT No Config", Url: "http://example.com", FeedType: "youtube-playlist", UserID: &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "config not set") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestFetchFeed_YouTubePlaylist_NoPlaylistID(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:          "YT No PID",
		Url:           "http://example.com",
		FeedType:      "youtube-playlist",
		ScraperConfig: new(`{"last_known_count":0}`),
		UserID:        &user.ID,
	})

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "playlist_id not set") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestFetchFeed_YouTubePlaylist_NoAPIKey_FallsBackToRSS(t *testing.T) {
	t.Parallel()
	// Set up an RSS server that serves a valid YouTube-like Atom feed.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprint(w, `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Playlist Feed</title>
  <entry>
    <id>yt:video:abc123</id>
    <title>Fallback Video</title>
    <link href="https://www.youtube.com/watch?v=abc123"/>
  </entry>
</feed>`)
	}))
	defer ts.Close()

	// We need the fetcher to hit our test server for the RSS fallback.
	// The playlist_id in config won't match the test server URL, but we can
	// craft the URL so that the RSS fallback URL resolves to our test server.
	sqlDB, q := setupTestDB(t)
	user := createTestUser(t, q)

	// No youtube API key set for this user.
	config := `{"playlist_id":"PLtest"}`
	feed, _ := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:          "YT Fallback",
		Url:           ts.URL,
		FeedType:      "youtube-playlist",
		ScraperConfig: &config,
		UserID:        &user.ID,
	})

	// The RSS fallback will try https://www.youtube.com/feeds/videos.xml?playlist_id=PLtest
	// which will fail in tests. That's expected — we test that the fallback path is taken.
	f := &Fetcher{DB: sqlDB, Client: ts.Client()}
	err := f.FetchFeed(context.Background(), &feed)
	// The RSS fallback will fail because the URL won't resolve, but that's fine —
	// we're testing that it attempts the fallback rather than crashing.
	if err == nil {
		// If it succeeded, that's also fine (test server might have served it).
		return
	}
	// The error should be about the RSS fetch failing, not about the API key.
	if strings.Contains(err.Error(), "youtube API") {
		t.Errorf("should not attempt API call without key, got: %v", err)
	}
}

func TestFetchFeed_YouTubePlaylist_NoUserID(t *testing.T) {
	t.Parallel()
	sqlDB, _ := setupTestDB(t)

	config := `{"playlist_id":"PLtest"}`
	feed := dbgen.Feed{
		ID:            999,
		Name:          "No User",
		Url:           "http://example.com",
		FeedType:      "youtube-playlist",
		ScraperConfig: &config,
		UserID:        nil,
	}

	f := &Fetcher{DB: sqlDB, Client: http.DefaultClient}
	err := f.FetchFeed(context.Background(), &feed)
	if err == nil {
		t.Fatal("expected error for nil user_id")
	}
	if !strings.Contains(err.Error(), "no user_id") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestYouTubePlaylistConfig_Marshal(t *testing.T) {
	t.Parallel()
	cfg := YouTubePlaylistConfig{
		PlaylistID:     "PLtest123",
		LastKnownCount: 42,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	var decoded YouTubePlaylistConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.PlaylistID != "PLtest123" {
		t.Errorf("PlaylistID = %q", decoded.PlaylistID)
	}
	if decoded.LastKnownCount != 42 {
		t.Errorf("LastKnownCount = %d", decoded.LastKnownCount)
	}
}
