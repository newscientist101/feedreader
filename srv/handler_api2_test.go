package srv

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/feeds"
)

// testFetcher creates a Fetcher backed by the server's DB.
func testFetcher(s *Server) *feeds.Fetcher {
	return feeds.NewFetcher(s.DB, s.ScraperRunner)
}

// helper to create a category directly via DB
func createCategory(t *testing.T, s *Server, userID int64, name string) dbgen.Category {
	t.Helper()
	q := dbgen.New(s.DB)
	cat, err := q.CreateCategory(context.Background(), dbgen.CreateCategoryParams{
		Name: name, UserID: &userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

// helper to create a scraper module directly via DB
func createScraperModule(t *testing.T, s *Server, userID int64, name, script string) dbgen.ScraperModule {
	t.Helper()
	q := dbgen.New(s.DB)
	m, err := q.CreateScraperModule(context.Background(), dbgen.CreateScraperModuleParams{
		Name:       name,
		Script:     script,
		ScriptType: "json",
		UserID:     &userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// --------------- Scraper CRUD ---------------

func TestHandlerScraperCRUD(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Create
	w := serveAPI(t, s.apiCreateScraper, "POST", "/api/scrapers",
		`{"name":"My Scraper","script":"{}","scriptType":"json"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("create got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	scraperID := int64(m["id"].(float64))
	if m["name"] != "My Scraper" {
		t.Fatalf("unexpected: %v", m)
	}

	// Create with missing fields
	w = serveAPI(t, s.apiCreateScraper, "POST", "/api/scrapers",
		`{"name":"","script":""}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for empty fields, got %d", w.Code)
	}

	// Create with bad JSON
	w = serveAPI(t, s.apiCreateScraper, "POST", "/api/scrapers",
		`not json`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for bad json, got %d", w.Code)
	}

	// Create with same name (upsert — the query uses ON CONFLICT DO UPDATE)
	w = serveAPI(t, s.apiCreateScraper, "POST", "/api/scrapers",
		`{"name":"My Scraper","script":"{\"updated\": true}"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("upsert should succeed, got %d: %s", w.Code, w.Body.String())
	}

	// Get
	w = serveMux(t, "GET /api/scrapers/{id}", s.apiGetScraper,
		"GET", fmt.Sprintf("/api/scrapers/%d", scraperID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("get got %d", w.Code)
	}

	// Get not found
	w = serveMux(t, "GET /api/scrapers/{id}", s.apiGetScraper,
		"GET", "/api/scrapers/99999", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// Get bad id
	w = serveMux(t, "GET /api/scrapers/{id}", s.apiGetScraper,
		"GET", "/api/scrapers/abc", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Update
	w = serveMux(t, "PUT /api/scrapers/{id}", s.apiUpdateScraper,
		"PUT", fmt.Sprintf("/api/scrapers/%d", scraperID),
		`{"name":"Updated","script":"{}","description":"desc"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("update got %d: %s", w.Code, w.Body.String())
	}

	// Update bad id
	w = serveMux(t, "PUT /api/scrapers/{id}", s.apiUpdateScraper,
		"PUT", "/api/scrapers/bad", `{"name":"X","script":"{}"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Update bad json
	w = serveMux(t, "PUT /api/scrapers/{id}", s.apiUpdateScraper,
		"PUT", fmt.Sprintf("/api/scrapers/%d", scraperID), `not json`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for bad json, got %d", w.Code)
	}

	// Delete
	w = serveMux(t, "DELETE /api/scrapers/{id}", s.apiDeleteScraper,
		"DELETE", fmt.Sprintf("/api/scrapers/%d", scraperID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("delete got %d", w.Code)
	}

	// Delete bad id
	w = serveMux(t, "DELETE /api/scrapers/{id}", s.apiDeleteScraper,
		"DELETE", "/api/scrapers/bad", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- Feed Create / Update / Refresh / Status ---------------

func TestHandlerCreateFeed(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)
	// Give the server a no-op fetcher so the goroutine doesn't panic
	s.Fetcher = testFetcher(s)

	// Basic create
	w := serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		`{"name":"Test Feed","url":"http://example.com/rss"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("create got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if m["name"] != "Test Feed" {
		t.Fatalf("unexpected name: %v", m["name"])
	}
	if m["feed_type"] != "rss" {
		t.Fatalf("expected rss type, got: %v", m["feed_type"])
	}

	// Create with empty URL
	w = serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		`{"name":"No URL"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing URL, got %d", w.Code)
	}

	// Create with bad JSON
	w = serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		`not json`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Create with category
	cat := createCategory(t, s, 1, "Tech")
	w = serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		fmt.Sprintf(`{"name":"Cat Feed","url":"http://example.com/rss2","categoryId":%d}`, cat.ID), ctx)
	if w.Code != 200 {
		t.Fatalf("create with category got %d: %s", w.Code, w.Body.String())
	}

	// Create with scraper type
	w = serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		`{"url":"http://example.com/page","feedType":"scraper","scraperModule":"mod1","scraperConfig":"{}"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("create scraper feed got %d", w.Code)
	}

	// Name should default to URL when empty
	w = serveAPI(t, s.apiCreateFeed, "POST", "/api/feeds",
		`{"url":"http://auto-name.example.com/feed"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m = jsonBody(t, w)
	if m["name"] != "http://auto-name.example.com/feed" {
		t.Errorf("expected URL as name, got %v", m["name"])
	}
}

func TestHandlerUpdateFeed(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "Original", "http://orig.com")

	// Update name
	w := serveMux(t, "PUT /api/feeds/{id}", s.apiUpdateFeed,
		"PUT", fmt.Sprintf("/api/feeds/%d", feed.ID),
		`{"name":"Updated Name"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("update got %d: %s", w.Code, w.Body.String())
	}

	// Verify update
	q := dbgen.New(s.DB)
	updated, _ := q.GetFeed(context.Background(), dbgen.GetFeedParams{ID: feed.ID, UserID: &user.ID})
	if updated.Name != "Updated Name" {
		t.Errorf("name = %q, want 'Updated Name'", updated.Name)
	}
	// URL should be preserved
	if updated.Url != "http://orig.com" {
		t.Errorf("url changed unexpectedly to %q", updated.Url)
	}

	// Bad ID
	w = serveMux(t, "PUT /api/feeds/{id}", s.apiUpdateFeed,
		"PUT", "/api/feeds/abc", `{"name":"X"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "PUT /api/feeds/{id}", s.apiUpdateFeed,
		"PUT", "/api/feeds/99999", `{"name":"X"}`, ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// Bad JSON
	w = serveMux(t, "PUT /api/feeds/{id}", s.apiUpdateFeed,
		"PUT", fmt.Sprintf("/api/feeds/%d", feed.ID), `not json`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerRefreshFeed(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	s.Fetcher = testFetcher(s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	w := serveMux(t, "POST /api/feeds/{id}/refresh", s.apiRefreshFeed,
		"POST", fmt.Sprintf("/api/feeds/%d/refresh", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if m["status"] != "refreshing" {
		t.Fatalf("unexpected: %v", m)
	}

	// Bad ID
	w = serveMux(t, "POST /api/feeds/{id}/refresh", s.apiRefreshFeed,
		"POST", "/api/feeds/abc/refresh", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "POST /api/feeds/{id}/refresh", s.apiRefreshFeed,
		"POST", "/api/feeds/99999/refresh", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandlerGetFeedStatus(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	w := serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", fmt.Sprintf("/api/feeds/%d/status", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m := jsonBody(t, w)
	if m["name"] != "f" {
		t.Fatalf("unexpected: %v", m)
	}

	// Bad ID
	w = serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", "/api/feeds/abc/status", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "GET /api/feeds/{id}/status", s.apiGetFeedStatus,
		"GET", "/api/feeds/99999/status", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --------------- Category Articles / Feed Articles ---------------

func TestHandlerGetCategoryArticles(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	cat := createCategory(t, s, user.ID, "News")
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "art1", "g1")

	// Assign feed to category
	q := dbgen.New(s.DB)
	q.AddFeedToCategory(context.Background(), dbgen.AddFeedToCategoryParams{
		FeedID: feed.ID, CategoryID: cat.ID,
	})

	w := serveMux(t, "GET /api/categories/{id}/articles", s.apiGetCategoryArticles,
		"GET", fmt.Sprintf("/api/categories/%d/articles", cat.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// Bad ID
	w = serveMux(t, "GET /api/categories/{id}/articles", s.apiGetCategoryArticles,
		"GET", "/api/categories/abc/articles", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "GET /api/categories/{id}/articles", s.apiGetCategoryArticles,
		"GET", "/api/categories/99999/articles", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandlerGetFeedArticles(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "art1", "g1")

	w := serveMux(t, "GET /api/feeds/{id}/articles", s.apiGetFeedArticles,
		"GET", fmt.Sprintf("/api/feeds/%d/articles", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// Bad ID
	w = serveMux(t, "GET /api/feeds/{id}/articles", s.apiGetFeedArticles,
		"GET", "/api/feeds/abc/articles", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "GET /api/feeds/{id}/articles", s.apiGetFeedArticles,
		"GET", "/api/feeds/99999/articles", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --------------- Reorder / Set Parent ---------------

func TestHandlerReorderCategories(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	cat1 := createCategory(t, s, user.ID, "A")
	cat2 := createCategory(t, s, user.ID, "B")

	body := fmt.Sprintf(`{"order":[%d,%d]}`, cat2.ID, cat1.ID)
	w := serveAPI(t, s.apiReorderCategories, "POST", "/api/categories/reorder", body, ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// Bad JSON
	w = serveAPI(t, s.apiReorderCategories, "POST", "/api/categories/reorder", "not json", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerSetCategoryParent(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	parent := createCategory(t, s, user.ID, "Parent")
	child := createCategory(t, s, user.ID, "Child")

	// Set parent
	w := serveMux(t, "PUT /api/categories/{id}/parent", s.apiSetCategoryParent,
		"PUT", fmt.Sprintf("/api/categories/%d/parent", child.ID),
		fmt.Sprintf(`{"parent_id":%d,"sort_order":0}`, parent.ID), ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// Set to top level (null parent)
	w = serveMux(t, "PUT /api/categories/{id}/parent", s.apiSetCategoryParent,
		"PUT", fmt.Sprintf("/api/categories/%d/parent", child.ID),
		`{"parent_id":null,"sort_order":0}`, ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// Self-parent should fail
	w = serveMux(t, "PUT /api/categories/{id}/parent", s.apiSetCategoryParent,
		"PUT", fmt.Sprintf("/api/categories/%d/parent", child.ID),
		fmt.Sprintf(`{"parent_id":%d}`, child.ID), ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for self-parent, got %d", w.Code)
	}

	// Bad ID
	w = serveMux(t, "PUT /api/categories/{id}/parent", s.apiSetCategoryParent,
		"PUT", "/api/categories/abc/parent", `{"parent_id":null}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Bad JSON
	w = serveMux(t, "PUT /api/categories/{id}/parent", s.apiSetCategoryParent,
		"PUT", fmt.Sprintf("/api/categories/%d/parent", child.ID), "not json", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- OPML ---------------

func TestHandlerExportOPML(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	createFeed(t, s, user.ID, "Feed1", "http://f1.com/rss")
	createFeed(t, s, user.ID, "Feed2", "http://f2.com/rss")

	w := serveAPI(t, s.apiExportOPML, "GET", "/api/opml/export", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/xml" {
		t.Errorf("content-type = %q", ct)
	}
	if !strings.Contains(w.Body.String(), "Feed1") {
		t.Error("expected Feed1 in OPML output")
	}
	if !strings.Contains(w.Body.String(), "Feed2") {
		t.Error("expected Feed2 in OPML output")
	}
}

func TestHandlerImportOPML_XML(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)
	s.Fetcher = testFetcher(s)

	opmlData := `<?xml version="1.0"?>
<opml version="2.0">
  <body>
    <outline text="Tech" title="Tech">
      <outline text="Go Blog" title="Go Blog" xmlUrl="http://go.dev/feed" type="rss"/>
    </outline>
    <outline text="Uncategorized" title="Uncategorized" xmlUrl="http://example.com/rss" type="rss"/>
  </body>
</opml>`

	r := httptest.NewRequest("POST", "/api/opml/import", strings.NewReader(opmlData))
	r.Header.Set("Content-Type", "application/xml")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	s.apiImportOPML(w, r)

	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if m["imported"].(float64) < 1 {
		t.Fatalf("expected at least 1 import, got %v", m)
	}

	// Import same data again — should be skipped
	r = httptest.NewRequest("POST", "/api/opml/import", strings.NewReader(opmlData))
	r.Header.Set("Content-Type", "application/xml")
	r = r.WithContext(ctx)
	w = httptest.NewRecorder()
	s.apiImportOPML(w, r)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m = jsonBody(t, w)
	if m["skipped"].(float64) < 1 {
		t.Errorf("expected skipped > 0, got %v", m)
	}
}

func TestHandlerImportOPML_Multipart(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)
	s.Fetcher = testFetcher(s)

	opmlData := `<?xml version="1.0"?>
<opml version="2.0">
  <body>
    <outline text="Feed" title="Feed" xmlUrl="http://mp.example.com/rss" type="rss"/>
  </body>
</opml>`

	var buf bytes.Buffer
	mpw := multipart.NewWriter(&buf)
	fw, _ := mpw.CreateFormFile("file", "feeds.opml")
	fw.Write([]byte(opmlData))
	mpw.Close()

	r := httptest.NewRequest("POST", "/api/opml/import", &buf)
	r.Header.Set("Content-Type", mpw.FormDataContentType())
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	s.apiImportOPML(w, r)

	if w.Code != 200 {
		t.Fatalf("multipart import got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerImportOPML_BadData(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	r := httptest.NewRequest("POST", "/api/opml/import", strings.NewReader("not opml"))
	r.Header.Set("Content-Type", "application/xml")
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	s.apiImportOPML(w, r)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- Retention ---------------

func TestHandlerRetentionStats(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiRetentionStats, "GET", "/api/retention/stats", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if _, ok := m["retention_days"]; !ok {
		t.Fatalf("missing retention_days: %v", m)
	}
}

func TestHandlerRetentionCleanup(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiRetentionCleanup, "POST", "/api/retention/cleanup", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if _, ok := m["deleted"]; !ok {
		t.Fatalf("missing 'deleted': %v", m)
	}
}

// --------------- Generate Scraper ---------------

func TestHandlerGenerateScraper_Validation(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Missing URL
	w := serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"description":"test"}`, ctx)
	// If Shelley is available, should get 400 for missing URL.
	// If unavailable, should get 503.
	if w.Code != 400 && w.Code != 503 {
		t.Fatalf("expected 400 or 503, got %d: %s", w.Code, w.Body.String())
	}

	// Bad JSON
	w = serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`not json`, ctx)
	if w.Code != 400 && w.Code != 503 {
		t.Fatalf("expected 400 or 503, got %d", w.Code)
	}
}

// --------------- Gzip Middleware ---------------

func TestGzipMiddleware(t *testing.T) {
	t.Parallel()
	handler := gzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello world"))
	}))

	// With gzip
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("expected gzip content-encoding")
	}

	// Without gzip
	r = httptest.NewRequest("GET", "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("expected no gzip when not requested")
	}
	if w.Body.String() != "hello world" {
		t.Errorf("body = %q", w.Body.String())
	}
}
