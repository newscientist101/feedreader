package srv

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/scrapers"
)

// cachedSchema holds the fully-migrated schema SQL so we only run
// migrations once per test binary instead of once per test.
var (
	cachedSchema     string
	cachedSchemaOnce sync.Once
)

// getSchema runs migrations once and captures the resulting schema.
func getSchema(t *testing.T) string {
	t.Helper()
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
		rows, err := d.Query("SELECT sql FROM sqlite_master WHERE sql IS NOT NULL AND sql NOT LIKE 'CREATE TABLE sqlite_%' ORDER BY rowid")
		if err != nil {
			panic(err)
		}
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

// testServer creates a Server backed by an in-memory SQLite DB with
// the fully-migrated schema applied. Uses a cached schema dump so
// migrations only run once per test binary.
func testServer(t *testing.T) *Server {
	t.Helper()
	schema := getSchema(t)
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
	s := &Server{
		DB:               sqlDB,
		Hostname:         "test",
		ScraperRunner:    scrapers.NewRunner(),
		StaticHashes:     map[string]string{},
		ShelleyGenerator: NewShelleyScraperGenerator(),
	}
	s.RetentionManager = &RetentionManager{server: s, retentionDays: 30}
	return s
}

// testUser creates a user in the DB and returns a context with that user set.
func testUser(t *testing.T, s *Server) (context.Context, *User) {
	t.Helper()
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(context.Background(), dbgen.GetOrCreateUserParams{
		ExternalID: "test-user",
		Email:      "test@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	user := &User{ID: dbUser.ID, ExternalID: dbUser.ExternalID, Email: dbUser.Email}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	return ctx, user
}

// serveAPI builds a request, injects the context, calls the handler, returns the recorder.
func serveAPI(t *testing.T, handler http.HandlerFunc, method, target, body string, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, http.NoBody)
	}
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

// jsonBody decodes the response into a map.
func jsonBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode json: %v\nbody: %s", err, w.Body.String())
	}
	return m
}

// createFeed inserts a feed directly.
func createFeed(t *testing.T, s *Server, userID int64, name, feedURL string) dbgen.Feed {
	t.Helper()
	q := dbgen.New(s.DB)
	interval := int64(60)
	feed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: name, Url: feedURL, FeedType: "rss",
		FetchIntervalMinutes: &interval, UserID: &userID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return feed
}

// createArticle inserts an article for a feed.
func createArticle(t *testing.T, s *Server, feedID int64, title, guid string) dbgen.Article {
	t.Helper()
	q := dbgen.New(s.DB)
	url := "https://example.com/" + guid
	art, err := q.CreateArticle(context.Background(), dbgen.CreateArticleParams{
		FeedID: feedID, Title: title, Guid: guid, Url: &url,
	})
	if err != nil {
		t.Fatal(err)
	}
	return art
}

// serveMux routes a request through a one-route mux so PathValue works.
func serveMux(t *testing.T, pattern string, handler http.HandlerFunc, method, target, body string, ctx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, handler)
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, target, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, target, http.NoBody)
	}
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

// suppress "unused" warnings for helpers used in later test files
var _ = bytes.NewBuffer
