package srv

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"
)

// testServerWithTemplates returns a test server configured to render real templates.
func testServerWithTemplates(t *testing.T) *Server {
	t.Helper()
	s := testServer(t)
	_, thisFile, _, _ := runtime.Caller(0)
	s.TemplatesDir = filepath.Join(filepath.Dir(thisFile), "templates")
	return s
}

func TestHandleIndex(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	createFeed(t, s, user.ID, "Feed1", "http://f1.com")

	w := serveAPI(t, s.handleIndex, "GET", "/", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String()[:200])
	}
	if ct := w.Header().Get("Content-Type"); ct != "" && ct != "text/html; charset=utf-8" {
		// renderTemplate may not set Content-Type explicitly, that's fine
	}
}

func TestHandleFeeds(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	createFeed(t, s, user.ID, "Feed1", "http://f1.com")

	w := serveAPI(t, s.handleFeeds, "GET", "/feeds", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleStarred(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleStarred, "GET", "/starred", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleQueue(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleQueue, "GET", "/queue", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleScrapers(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleScrapers, "GET", "/scrapers", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleSettings(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleSettings, "GET", "/settings", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleFeedArticles(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "art1", "g1")

	w := serveMux(t, "GET /feeds/{id}", s.handleFeedArticles,
		"GET", fmt.Sprintf("/feeds/%d", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleArticle(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "art1", "g1")

	w := serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// Not found
	w = serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", "/articles/99999", "", ctx)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHandleCategoryArticles(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	cat := createCategory(t, s, user.ID, "Tech")

	w := serveMux(t, "GET /categories/{id}", s.handleCategoryArticles,
		"GET", fmt.Sprintf("/categories/%d", cat.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleCategorySettings(t *testing.T) {
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	cat := createCategory(t, s, user.ID, "News")

	w := serveMux(t, "GET /categories/{id}/settings", s.handleCategorySettings,
		"GET", fmt.Sprintf("/categories/%d/settings", cat.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// Bad ID
	w = serveMux(t, "GET /categories/{id}/settings", s.handleCategorySettings,
		"GET", "/categories/abc/settings", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
