package srv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"srv.exe.dev/srv/feeds"
)

// integrationServer creates a full httptest.Server running the real mux,
// auth middleware, and gzip — the same stack as production.
// Requests must include X-Exedev-Userid and X-Exedev-Email headers.
func integrationServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	s := testServer(t)
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	s.TemplatesDir = filepath.Join(baseDir, "templates")
	s.StaticDir = filepath.Join(baseDir, "static")
	s.Fetcher = feeds.NewFetcher(s.DB, s.ScraperRunner)

	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return ts, s
}

// authGet issues a GET with auth headers.
func authGet(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("GET", ts.URL+path, nil)
	req.Header.Set("X-Exedev-Userid", "integ-user")
	req.Header.Set("X-Exedev-Email", "integ@test.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// authPost issues a POST with auth headers and JSON body.
func authPost(t *testing.T, ts *httptest.Server, path, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+path, strings.NewReader(body))
	req.Header.Set("X-Exedev-Userid", "integ-user")
	req.Header.Set("X-Exedev-Email", "integ@test.com")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// authDo issues an arbitrary request with auth headers.
func authDo(t *testing.T, ts *httptest.Server, method, path, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, _ := http.NewRequest(method, ts.URL+path, bodyReader)
	req.Header.Set("X-Exedev-Userid", "integ-user")
	req.Header.Set("X-Exedev-Email", "integ@test.com")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

// readJSON reads and decodes a JSON response body into a map.
func readJSON(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatalf("decode json: %v", err)
	}
	return m
}

// readJSONArray reads a JSON array response.
func readJSONArray(t *testing.T, resp *http.Response) []any {
	t.Helper()
	defer resp.Body.Close()
	var arr []any
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode json array: %v", err)
	}
	return arr
}

// ---------- Auth integration ----------

func TestIntegration_UnauthenticatedAPIReturns401(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	resp, err := http.Get(ts.URL + "/api/counts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestIntegration_UnauthenticatedPageRedirects(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// Disable redirect following
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(ts.URL + "/feeds")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", resp.StatusCode)
	}
}

func TestIntegration_StaticFilesNoAuth(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	resp, err := http.Get(ts.URL + "/static/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------- Full workflow: create feed → read articles → star → queue ----------

func TestIntegration_FeedWorkflow(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// 1. Create a category
	resp := authPost(t, ts, "/api/categories", `{"name":"Tech"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("create category: %d", resp.StatusCode)
	}
	cat := readJSON(t, resp)
	catID := int64(cat["id"].(float64))

	// 2. Create a feed in that category
	resp = authPost(t, ts, "/api/feeds",
		fmt.Sprintf(`{"name":"Go Blog","url":"http://go.dev/feed","categoryId":%d}`, catID))
	if resp.StatusCode != 200 {
		t.Fatalf("create feed: %d", resp.StatusCode)
	}
	feed := readJSON(t, resp)
	feedID := int64(feed["id"].(float64))

	// 3. Verify counts endpoint works
	resp = authGet(t, ts, "/api/counts")
	if resp.StatusCode != 200 {
		t.Fatalf("counts: %d", resp.StatusCode)
	}
	counts := readJSON(t, resp)
	if counts["feeds"] == nil {
		t.Fatal("missing feeds in counts")
	}

	// 4. Verify feed status
	resp = authGet(t, ts, fmt.Sprintf("/api/feeds/%d/status", feedID))
	if resp.StatusCode != 200 {
		t.Fatalf("feed status: %d", resp.StatusCode)
	}

	// 5. Verify feed detail page renders
	resp = authGet(t, ts, fmt.Sprintf("/feed/%d", feedID))
	if resp.StatusCode != 200 {
		t.Fatalf("feed page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. Get feed articles via API
	resp = authGet(t, ts, fmt.Sprintf("/api/feeds/%d/articles", feedID))
	if resp.StatusCode != 200 {
		t.Fatalf("feed articles: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Get category articles
	resp = authGet(t, ts, fmt.Sprintf("/api/categories/%d/articles", catID))
	if resp.StatusCode != 200 {
		t.Fatalf("category articles: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 8. Update the feed
	resp = authDo(t, ts, "PUT", fmt.Sprintf("/api/feeds/%d", feedID),
		`{"name":"Go Blog Updated"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("update feed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 9. Delete the feed
	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/feeds/%d", feedID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete feed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 10. Verify it's gone
	resp = authGet(t, ts, fmt.Sprintf("/api/feeds/%d", feedID))
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------- Full workflow: articles (read, star, queue) ----------

func TestIntegration_ArticleWorkflow(t *testing.T) {
	t.Parallel()
	ts, s := integrationServer(t)

	// Create feed + article directly in DB (bypasses fetch)
	// First, make an authed request to ensure the user exists
	resp := authGet(t, ts, "/api/counts")
	resp.Body.Close()

	// Now insert test data — the user "integ-user" was created by the auth middleware
	var userID int64
	s.DB.QueryRow("SELECT id FROM users WHERE external_id = 'integ-user'").Scan(&userID)
	if userID == 0 {
		t.Fatal("user not created")
	}

	_, err := s.DB.Exec(`INSERT INTO feeds (name, url, feed_type, user_id) VALUES (?, ?, ?, ?)`,
		"Test Feed", "http://test.example.com/rss", "rss", userID)
	if err != nil {
		t.Fatal(err)
	}
	var feedID int64
	s.DB.QueryRow("SELECT id FROM feeds WHERE user_id = ?", userID).Scan(&feedID)

	_, err = s.DB.Exec(`INSERT INTO articles (feed_id, guid, title, url) VALUES (?, ?, ?, ?)`,
		feedID, "test-guid-1", "Test Article", "http://test.example.com/1")
	if err != nil {
		t.Fatal(err)
	}
	var artID int64
	s.DB.QueryRow("SELECT id FROM articles WHERE guid = 'test-guid-1'").Scan(&artID)

	// 1. Article page renders
	resp = authGet(t, ts, fmt.Sprintf("/article/%d", artID))
	if resp.StatusCode != 200 {
		t.Fatalf("article page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Mark as read
	resp = authPost(t, ts, fmt.Sprintf("/api/articles/%d/read", artID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("mark read: %d", resp.StatusCode)
	}
	m := readJSON(t, resp)
	if m["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", m)
	}

	// 3. Mark as unread
	resp = authPost(t, ts, fmt.Sprintf("/api/articles/%d/unread", artID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("mark unread: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Star
	resp = authPost(t, ts, fmt.Sprintf("/api/articles/%d/star", artID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("star: %d", resp.StatusCode)
	}
	m = readJSON(t, resp)
	if m["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", m)
	}

	// 5. Add to queue
	resp = authPost(t, ts, fmt.Sprintf("/api/articles/%d/queue", artID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("queue: %d", resp.StatusCode)
	}
	m = readJSON(t, resp)
	if m["queued"] != true {
		t.Fatalf("expected queued=true")
	}

	// 6. Verify queue list
	resp = authGet(t, ts, "/api/queue")
	if resp.StatusCode != 200 {
		t.Fatalf("queue list: %d", resp.StatusCode)
	}
	arr := readJSONArray(t, resp)
	if len(arr) != 1 {
		t.Fatalf("expected 1 queued article, got %d", len(arr))
	}

	// 7. Queue page renders
	resp = authGet(t, ts, "/queue")
	if resp.StatusCode != 200 {
		t.Fatalf("queue page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 8. Remove from queue
	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/articles/%d/queue", artID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("remove from queue: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 9. Search
	resp = authGet(t, ts, "/api/search?q=Test")
	if resp.StatusCode != 200 {
		t.Fatalf("search: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 10. Mark all read
	resp = authPost(t, ts, "/api/articles/read-all", "")
	if resp.StatusCode != 200 {
		t.Fatalf("mark all read: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------- Scraper + OPML + Settings workflow ----------

func TestIntegration_ScraperAndSettings(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// 1. Create scraper
	resp := authPost(t, ts, "/api/scrapers",
		`{"name":"Test Scraper","script":"{}","description":"A test"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("create scraper: %d", resp.StatusCode)
	}
	scraper := readJSON(t, resp)
	scraperID := int64(scraper["id"].(float64))

	// 2. Get scraper
	resp = authGet(t, ts, fmt.Sprintf("/api/scrapers/%d", scraperID))
	if resp.StatusCode != 200 {
		t.Fatalf("get scraper: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 3. Update scraper
	resp = authDo(t, ts, "PUT", fmt.Sprintf("/api/scrapers/%d", scraperID),
		`{"name":"Updated Scraper","script":"{}"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("update scraper: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Delete scraper
	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/scrapers/%d", scraperID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete scraper: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Scrapers page renders
	resp = authGet(t, ts, "/scrapers")
	if resp.StatusCode != 200 {
		t.Fatalf("scrapers page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 6. Update settings
	resp = authDo(t, ts, "PUT", "/api/settings", `{"autoMarkRead":"true"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("update settings: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Read settings back
	resp = authGet(t, ts, "/api/settings")
	if resp.StatusCode != 200 {
		t.Fatalf("get settings: %d", resp.StatusCode)
	}
	settings := readJSON(t, resp)
	if settings["autoMarkRead"] != "true" {
		t.Fatalf("setting not persisted: %v", settings)
	}

	// 8. Settings page renders
	resp = authGet(t, ts, "/settings")
	if resp.StatusCode != 200 {
		t.Fatalf("settings page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 9. Retention stats
	resp = authGet(t, ts, "/api/retention/stats")
	if resp.StatusCode != 200 {
		t.Fatalf("retention stats: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 10. AI status
	resp = authGet(t, ts, "/api/ai/status")
	if resp.StatusCode != 200 {
		t.Fatalf("ai status: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------- OPML round-trip ----------

func TestIntegration_OPMLRoundTrip(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// 1. Create a category and a feed in it
	resp := authPost(t, ts, "/api/categories", `{"name":"News"}`)
	cat := readJSON(t, resp)
	catID := int64(cat["id"].(float64))

	resp = authPost(t, ts, "/api/feeds",
		fmt.Sprintf(`{"name":"BBC","url":"http://bbc.co.uk/rss","categoryId":%d}`, catID))
	if resp.StatusCode != 200 {
		t.Fatalf("create feed: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Export OPML
	resp = authGet(t, ts, "/api/opml/export")
	if resp.StatusCode != 200 {
		t.Fatalf("export: %d", resp.StatusCode)
	}
	opmlBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	opml := string(opmlBytes)
	if !strings.Contains(opml, "BBC") {
		t.Error("OPML missing feed name")
	}
	if !strings.Contains(opml, "bbc.co.uk") {
		t.Error("OPML missing feed URL")
	}
}

// ---------- Category with exclusion rules ----------

func TestIntegration_ExclusionRules(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// Create category
	resp := authPost(t, ts, "/api/categories", `{"name":"Filtered"}`)
	cat := readJSON(t, resp)
	catID := int64(cat["id"].(float64))

	// Add exclusion rule
	resp = authPost(t, ts, fmt.Sprintf("/api/categories/%d/exclusions", catID),
		`{"type":"keyword","pattern":"spam","isRegex":false}`)
	if resp.StatusCode != 200 {
		t.Fatalf("create exclusion: %d", resp.StatusCode)
	}
	excl := readJSON(t, resp)
	exclID := int64(excl["id"].(float64))

	// List exclusions
	resp = authGet(t, ts, fmt.Sprintf("/api/categories/%d/exclusions", catID))
	if resp.StatusCode != 200 {
		t.Fatalf("list exclusions: %d", resp.StatusCode)
	}
	arr := readJSONArray(t, resp)
	if len(arr) != 1 {
		t.Fatalf("expected 1 exclusion, got %d", len(arr))
	}

	// Category settings page renders
	resp = authGet(t, ts, fmt.Sprintf("/category/%d/settings", catID))
	if resp.StatusCode != 200 {
		t.Fatalf("category settings page: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete exclusion
	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/exclusions/%d", exclID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete exclusion: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------- Category hierarchy ----------

func TestIntegration_CategoryHierarchy(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	// Create parent
	resp := authPost(t, ts, "/api/categories", `{"name":"Parent"}`)
	parent := readJSON(t, resp)
	parentID := int64(parent["id"].(float64))

	// Create child
	resp = authPost(t, ts, "/api/categories", `{"name":"Child"}`)
	child := readJSON(t, resp)
	childID := int64(child["id"].(float64))

	// Set parent
	resp = authPost(t, ts, fmt.Sprintf("/api/categories/%d/parent", childID),
		fmt.Sprintf(`{"parent_id":%d,"sort_order":0}`, parentID))
	if resp.StatusCode != 200 {
		t.Fatalf("set parent: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Reorder
	resp = authPost(t, ts, "/api/categories/reorder",
		fmt.Sprintf(`{"order":[%d,%d]}`, childID, parentID))
	if resp.StatusCode != 200 {
		t.Fatalf("reorder: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Rename
	resp = authDo(t, ts, "PUT", fmt.Sprintf("/api/categories/%d", parentID),
		`{"name":"Renamed Parent"}`)
	if resp.StatusCode != 200 {
		t.Fatalf("rename: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Delete child then parent
	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/categories/%d", childID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete child: %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = authDo(t, ts, "DELETE", fmt.Sprintf("/api/categories/%d", parentID), "")
	if resp.StatusCode != 200 {
		t.Fatalf("delete parent: %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// ---------- Pages render for fresh user (no data) ----------

func TestIntegration_EmptyStatePages(t *testing.T) {
	t.Parallel()
	ts, _ := integrationServer(t)

	pages := []string{
		"/",
		"/feeds",
		"/starred",
		"/queue",
		"/scrapers",
		"/settings",
	}
	for _, p := range pages {
		t.Run(p, func(t *testing.T) {
			resp := authGet(t, ts, p)
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("%s: got %d", p, resp.StatusCode)
			}
		})
	}
}
