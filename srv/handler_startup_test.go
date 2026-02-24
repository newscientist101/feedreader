package srv

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// --------------- fetchSteamAppName ---------------

func TestFetchSteamAppName_Success(t *testing.T) {
	t.Parallel()
	// Mock the Steam API response for appID "440" (Team Fortress 2)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appIDs := r.URL.Query().Get("appids")
		if appIDs == "" {
			w.WriteHeader(400)
			return
		}
		resp := map[string]any{
			appIDs: map[string]any{
				"success": true,
				"data": map[string]any{
					"name": "Team Fortress 2",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	// fetchSteamAppName uses http.Get with a hardcoded URL, so we need to
	// override the default transport to redirect requests to our test server.
	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		base:   originalTransport,
		target: ts.URL,
	}
	defer func() { http.DefaultTransport = originalTransport }()

	name := fetchSteamAppName("440")
	if name != "Team Fortress 2" {
		t.Errorf("fetchSteamAppName(\"440\") = %q, want \"Team Fortress 2\"", name)
	}
}

func TestFetchSteamAppName_NotFound(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appIDs := r.URL.Query().Get("appids")
		resp := map[string]any{
			appIDs: map[string]any{
				"success": false,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		base:   originalTransport,
		target: ts.URL,
	}
	defer func() { http.DefaultTransport = originalTransport }()

	name := fetchSteamAppName("9999999")
	if name != "" {
		t.Errorf("fetchSteamAppName(\"9999999\") = %q, want empty", name)
	}
}

func TestFetchSteamAppName_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "not json at all")
	}))
	defer ts.Close()

	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		base:   originalTransport,
		target: ts.URL,
	}
	defer func() { http.DefaultTransport = originalTransport }()

	name := fetchSteamAppName("440")
	if name != "" {
		t.Errorf("fetchSteamAppName(\"440\") = %q, want empty for invalid json", name)
	}
}

func TestFetchSteamAppName_ServerError(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()

	originalTransport := http.DefaultTransport
	http.DefaultTransport = &rewriteTransport{
		base:   originalTransport,
		target: ts.URL,
	}
	defer func() { http.DefaultTransport = originalTransport }()

	// Even with 500, http.Get succeeds (just returns error body).
	// The JSON decode will fail or "success" will be missing.
	name := fetchSteamAppName("440")
	if name != "" {
		t.Errorf("fetchSteamAppName(\"440\") = %q, want empty for server error", name)
	}
}

// rewriteTransport rewrites requests to the Steam API to our test server.
type rewriteTransport struct {
	base   http.RoundTripper
	target string // test server URL
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite Steam API requests to our test server
	if req.URL.Host == "store.steampowered.com" {
		newURL := t.target + req.URL.Path + "?" + req.URL.RawQuery
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		if err != nil {
			return nil, err
		}
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}

// --------------- New ---------------

func TestNew(t *testing.T) {
	// New creates a real DB file, so use a temp dir.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	srv, err := New(dbPath, "testhost")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer srv.DB.Close()

	if srv.Hostname != "testhost" {
		t.Errorf("Hostname = %q, want \"testhost\"", srv.Hostname)
	}
	if srv.DB == nil {
		t.Error("DB is nil")
	}
	if srv.Fetcher == nil {
		t.Error("Fetcher is nil")
	}
	if srv.ScraperRunner == nil {
		t.Error("ScraperRunner is nil")
	}
	if srv.StaticHashes == nil {
		t.Error("StaticHashes is nil")
	}
	if srv.TemplatesDir == "" {
		t.Error("TemplatesDir is empty")
	}
	if srv.StaticDir == "" {
		t.Error("StaticDir is empty")
	}

	// Verify DB is functional by running a simple query.
	var n int
	if err := srv.DB.QueryRow("SELECT 1").Scan(&n); err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("SELECT 1 = %d", n)
	}

	// Verify migrations ran — the migrations table should exist.
	var tableName string
	err = srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("migrations table not found: %v", err)
	}
}

func TestNew_BadPath(t *testing.T) {
	// An invalid path should return an error.
	_, err := New("/nonexistent/dir/that/does/not/exist/db.sqlite", "host")
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
}

// --------------- hashStaticFiles ---------------

func TestHashStaticFiles(t *testing.T) {
	// Create a temp dir with known files to test hashing.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmpDir, "icons")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "icon.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	hashes := hashStaticFiles(tmpDir)

	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d: %v", len(hashes), hashes)
	}

	// Check that style.css has a hash.
	if h, ok := hashes["style.css"]; !ok {
		t.Error("missing hash for style.css")
	} else if len(h) != 8 {
		t.Errorf("hash for style.css has length %d, want 8", len(h))
	}

	// Check the nested file.
	key := filepath.Join("icons", "icon.svg")
	if h, ok := hashes[key]; !ok {
		t.Errorf("missing hash for %s", key)
	} else if len(h) != 8 {
		t.Errorf("hash for %s has length %d, want 8", key, len(h))
	}
}

func TestHashStaticFiles_NonexistentDir(t *testing.T) {
	hashes := hashStaticFiles("/nonexistent/dir/xyz")
	if len(hashes) != 0 {
		t.Errorf("expected empty map for missing dir, got %v", hashes)
	}
}

func TestHashStaticFiles_Deterministic(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.js"), []byte("console.log('hi')"), 0o644)

	h1 := hashStaticFiles(tmpDir)
	h2 := hashStaticFiles(tmpDir)

	if h1["a.js"] != h2["a.js"] {
		t.Errorf("hashes not deterministic: %q vs %q", h1["a.js"], h2["a.js"])
	}
}

// --------------- setUpDatabase ---------------

func TestSetUpDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "setup_test.db")

	srv := &Server{}
	if err := srv.setUpDatabase(dbPath); err != nil {
		t.Fatalf("setUpDatabase() error: %v", err)
	}
	defer srv.DB.Close()

	// DB should be set.
	if srv.DB == nil {
		t.Fatal("DB is nil after setUpDatabase")
	}

	// Migrations table should exist.
	var tableName string
	err := srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("migrations table not found: %v", err)
	}

	// Core tables should exist (feeds, articles, users, etc.).
	for _, table := range []string{"feeds", "articles", "users", "categories", "scraper_modules"} {
		var name string
		err := srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}

	// WAL mode should be enabled.
	var journalMode string
	srv.DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\"", journalMode)
	}

	// Foreign keys should be enabled.
	var fk int
	srv.DB.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestSetUpDatabase_BadPath(t *testing.T) {
	srv := &Server{}
	err := srv.setUpDatabase("/nonexistent/dir/that/does/not/exist/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
}

// --------------- apiGenerateScraper ---------------

func TestHandlerGenerateScraper_Unavailable(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Point ShelleyGenerator at a mock server that returns 503 for IsAvailable.
	mockShelley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer mockShelley.Close()

	s.ShelleyGenerator = &ShelleyScraperGenerator{
		shelleyURL: mockShelley.URL,
		httpClient: mockShelley.Client(),
	}

	w := serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"url":"http://example.com","description":"test"}`, ctx)
	if w.Code != 503 {
		t.Fatalf("expected 503 when Shelley unavailable, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerGenerateScraper_SuccessPath(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Create a fake Shelley DB with the expected response.
	tmpDir := t.TempDir()
	fakeDBPath := filepath.Join(tmpDir, "shelley.db")

	fakeDB, err := sql.Open("sqlite", fakeDBPath)
	if err != nil {
		t.Fatalf("failed to create fake Shelley DB: %v", err)
	}
	_, err = fakeDB.Exec(`
		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			type TEXT NOT NULL,
			sequence_id INTEGER NOT NULL,
			llm_data TEXT
		)
	`)
	if err != nil {
		fakeDB.Close()
		t.Fatalf("failed to create messages table: %v", err)
	}

	convID := "test-conv-123"
	scraperJSON := `{"name": "Test Scraper", "config": {"type": "html", "itemSelector": "div.item", "titleSelector": "h2"}}`
	llmData := fmt.Sprintf(`{"Content":[{"Type":2,"Text":%s}]}`, mustJSON(scraperJSON))
	_, err = fakeDB.Exec(
		"INSERT INTO messages (conversation_id, type, sequence_id, llm_data) VALUES (?, 'agent', 1, ?)",
		convID, llmData,
	)
	if err != nil {
		fakeDB.Close()
		t.Fatalf("failed to insert fake message: %v", err)
	}
	fakeDB.Close()

	// Track polling call count to transition from working -> done.
	pollCount := 0

	// Create mock Shelley HTTP server.
	mockShelley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/conversations":
			pollCount++
			// First poll: still working. Second+: done.
			working := pollCount <= 1
			resp := []map[string]any{{
				"conversation_id": convID,
				"working":         working,
			}}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.Method == "POST" && r.URL.Path == "/api/conversations/new":
			resp := map[string]string{
				"conversation_id": convID,
				"status":          "processing",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		default:
			w.WriteHeader(404)
		}
	}))
	defer mockShelley.Close()

	// Replace the ShelleyGenerator with one pointing to our mock.
	s.ShelleyGenerator = &ShelleyScraperGenerator{
		shelleyURL: mockShelley.URL,
		httpClient: mockShelley.Client(),
		dbPath:     fakeDBPath,
	}

	w := serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"url":"http://example.com","description":"extract articles"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	body := jsonBody(t, w)
	if body["name"] != "Test Scraper" {
		t.Errorf("name = %v, want \"Test Scraper\"", body["name"])
	}
	if body["config"] == nil || body["config"] == "" {
		t.Error("expected non-empty config")
	}
}

func TestHandlerGenerateScraper_MissingFields(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Make Shelley "available" with a mock server that responds to IsAvailable.
	mockShelley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer mockShelley.Close()

	s.ShelleyGenerator = &ShelleyScraperGenerator{
		shelleyURL: mockShelley.URL,
		httpClient: mockShelley.Client(),
	}

	// Missing URL
	w := serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"description":"test"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing URL, got %d: %s", w.Code, w.Body.String())
	}

	// Missing description
	w = serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"url":"http://example.com"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for missing description, got %d: %s", w.Code, w.Body.String())
	}

	// Bad JSON
	w = serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`not json`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for bad json, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandlerGenerateScraper_GenerateError(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Mock server: IsAvailable returns true, but Generate fails.
	mockShelley := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/api/conversations":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, "[]")
		case r.Method == "POST" && r.URL.Path == "/api/conversations/new":
			w.WriteHeader(500)
			fmt.Fprint(w, "internal error")
		default:
			w.WriteHeader(404)
		}
	}))
	defer mockShelley.Close()

	s.ShelleyGenerator = &ShelleyScraperGenerator{
		shelleyURL: mockShelley.URL,
		httpClient: mockShelley.Client(),
	}

	w := serveAPI(t, s.apiGenerateScraper, "POST", "/api/ai/generate-scraper",
		`{"url":"http://example.com","description":"extract articles"}`, ctx)
	if w.Code != 500 {
		t.Fatalf("expected 500 for generate error, got %d: %s", w.Code, w.Body.String())
	}
}

// mustJSON marshals v to a JSON string. Panics on error.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// --------------- Serve (skipped) ---------------
// Serve is not unit-tested because it calls http.ListenAndServe which
// blocks until the server shuts down, starts background goroutines
// (Fetcher, RetentionManager, EmailWatcher), and binds a real TCP port.
// Integration testing of Serve would require orchestrating a clean
// shutdown which is out of scope for unit tests. The Handler() method
// it uses is indirectly tested through all the API handler tests.

// --------------- Notes ---------------
// hashStaticFiles: Testable directly since it uses os.ReadFile on a real
// directory (not embed.FS). Tests above use temp directories.
