package srv

import (
	"context"
	"fmt"
	"testing"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
)

// --------------- Newsletter handlers ---------------

func TestHandlerGetNewsletterAddress_NoToken(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiGetNewsletterAddress, "GET", "/api/newsletter/address", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	if m["address"] != nil {
		t.Fatalf("expected null address, got %v", m["address"])
	}
}

func TestHandlerGetNewsletterAddress_WithToken(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)

	// Set a token directly in the DB
	q := dbgen.New(s.DB)
	if err := q.SetNewsletterToken(context.Background(), dbgen.SetNewsletterTokenParams{
		UserID: user.ID,
		Value:  "testtoken123",
	}); err != nil {
		t.Fatal(err)
	}

	w := serveAPI(t, s.apiGetNewsletterAddress, "GET", "/api/newsletter/address", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	addr, ok := m["address"].(string)
	if !ok || addr == "" {
		t.Fatalf("expected non-empty address, got %v", m["address"])
	}
	expected := "nl-testtoken123@test"
	if addr != expected {
		t.Fatalf("expected %q, got %q", expected, addr)
	}
}

func TestHandlerGenerateNewsletterAddress_New(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// First call should generate a new address
	w := serveAPI(t, s.apiGenerateNewsletterAddress, "POST", "/api/newsletter/address", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	addr1, ok := m["address"].(string)
	if !ok || addr1 == "" {
		t.Fatalf("expected non-empty address, got %v", m["address"])
	}

	// Second call should return the same address (token already exists)
	w = serveAPI(t, s.apiGenerateNewsletterAddress, "POST", "/api/newsletter/address", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}
	m = jsonBody(t, w)
	addr2, ok := m["address"].(string)
	if !ok || addr2 == "" {
		t.Fatalf("expected non-empty address, got %v", m["address"])
	}
	if addr1 != addr2 {
		t.Fatalf("expected same address on second call, got %q vs %q", addr1, addr2)
	}
}

// --------------- Retention cleanup error path ---------------

func TestHandlerRetentionCleanup_DBError(t *testing.T) {
	t.Parallel()

	// Create a server with a DB that will be closed, causing query errors
	schema := getSchema(t)
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		t.Fatal(err)
	}

	s := &Server{
		DB:       sqlDB,
		Hostname: "test",
	}
	s.RetentionManager = &RetentionManager{server: s, retentionDays: 30}

	// Create user before closing DB
	ctx, _ := testUser(t, s)

	// Close the DB to force errors
	sqlDB.Close()

	w := serveAPI(t, s.apiRetentionCleanup, "POST", "/api/retention/cleanup", "", ctx)
	if w.Code != 500 {
		t.Fatalf("expected 500, got %d: %s", w.Code, w.Body.String())
	}
}

// --------------- Mark unread error paths ---------------

func TestHandlerMarkUnread_InvalidID(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveMux(t, "POST /api/articles/{id}/unread", s.apiMarkUnread,
		"POST", "/api/articles/abc/unread", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerMarkUnread_NotFound(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// Article ID 99999 doesn't exist; MarkArticleUnread doesn't error for
	// non-existent rows (UPDATE ... WHERE id=? affects 0 rows), so the
	// handler returns 200. This test still exercises the DB execution path
	// with a non-existent article.
	w := serveMux(t, "POST /api/articles/{id}/unread", s.apiMarkUnread,
		"POST", "/api/articles/99999/unread", "", ctx)
	// Handler returns 200 for a no-op update (no rows matched)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --------------- Toggle queue error paths ---------------

func TestHandlerToggleQueue_InvalidID(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", "/api/articles/nope/queue", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- Remove from queue error paths ---------------

func TestHandlerRemoveFromQueue_InvalidID(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveMux(t, "DELETE /api/articles/{id}/queue", s.apiRemoveFromQueue,
		"DELETE", "/api/articles/xyz/queue", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerRemoveFromQueue_NotInQueue(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	// Try to remove an article that was never added to the queue
	w := serveMux(t, "DELETE /api/articles/{id}/queue", s.apiRemoveFromQueue,
		"DELETE", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	// RemoveFromQueue does a DELETE WHERE which succeeds with 0 rows
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
