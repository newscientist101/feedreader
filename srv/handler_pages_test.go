package srv

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/newscientist101/feedreader/db/dbgen"
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
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	createFeed(t, s, user.ID, "Feed1", "http://f1.com")

	w := serveAPI(t, s.handleIndex, "GET", "/", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String()[:200])
	}
	if ct := w.Header().Get("Content-Type"); ct != "" && ct != "text/html; charset=utf-8" {
		t.Logf("unexpected Content-Type: %s", ct)
	}
}

func TestHandleFeeds(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	createFeed(t, s, user.ID, "Feed1", "http://f1.com")

	w := serveAPI(t, s.handleFeeds, "GET", "/feeds", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleStarred(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleStarred, "GET", "/starred", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleQueue(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleQueue, "GET", "/queue", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleHistory(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleHistory, "GET", "/history", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleScrapers(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleScrapers, "GET", "/scrapers", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleSettings(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.handleSettings, "GET", "/settings", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandleFeedArticles(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestHandleArticle_AddsToHistory(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)
	feed := createFeed(t, s, user.ID, "f", "http://f.com/feed")
	art := createArticle(t, s, feed.ID, "history-art", "hg1")

	// Verify no history initially
	count, _ := q.GetHistoryCount(ctx, user.ID)
	if count != 0 {
		t.Fatalf("expected 0 history entries, got %d", count)
	}

	// View the article
	w := serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// Verify it was added to history
	count, _ = q.GetHistoryCount(ctx, user.ID)
	if count != 1 {
		t.Fatalf("expected 1 history entry, got %d", count)
	}

	history, _ := q.ListHistoryArticles(ctx, dbgen.ListHistoryArticlesParams{
		UserID: user.ID, Limit: 10, Offset: 0,
	})
	if len(history) != 1 || history[0].ID != art.ID {
		t.Errorf("history entry mismatch: got %+v", history)
	}

	// View same article again — should update timestamp, not duplicate
	w = serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	count, _ = q.GetHistoryCount(ctx, user.ID)
	if count != 1 {
		t.Fatalf("expected still 1 history entry after re-view, got %d", count)
	}
}

func TestHandleCategoryArticles(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

// TestHandleCategoryArticles_SubcategoryRendersDataID verifies that a
// subcategory page renders article cards with data-id attributes, which
// the auto-mark-read IntersectionObserver needs after client-side navigation.
func TestHandleCategoryArticles_SubcategoryRendersDataID(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	parent := createCategory(t, s, user.ID, "Gaming")
	child := createCategory(t, s, user.ID, "VR")
	if err := q.UpdateCategoryParent(context.Background(), dbgen.UpdateCategoryParentParams{
		ParentID: &parent.ID, SortOrder: new(int64), ID: child.ID, UserID: &user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	feed := createFeed(t, s, user.ID, "VR Feed", "http://vr.example.com")
	art := createArticle(t, s, feed.ID, "VR Article", "vr-g1")
	if err := q.AddFeedToCategory(context.Background(), dbgen.AddFeedToCategoryParams{
		FeedID: feed.ID, CategoryID: child.ID,
	}); err != nil {
		t.Fatal(err)
	}

	w := serveMux(t, "GET /categories/{id}", s.handleCategoryArticles,
		"GET", fmt.Sprintf("/categories/%d", child.ID), "", ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}

	body := w.Body.String()

	// The rendered page must contain an article card with the correct data-id.
	// This is what initAutoMarkRead() queries to attach the IntersectionObserver.
	wantAttr := fmt.Sprintf(`data-id="%d"`, art.ID)
	if !strings.Contains(body, wantAttr) {
		t.Errorf("rendered subcategory page missing %s", wantAttr)
	}

	// Must also contain the articles-list container that the observer targets
	if !strings.Contains(body, `id="articles-list"`) {
		t.Error("rendered page missing articles-list container")
	}
}

// testServerWithTemplatesAndUsenet returns a test server with real templates
// and Usenet enabled.
func testServerWithTemplatesAndUsenet(t *testing.T) *Server {
	t.Helper()
	s := testServerWithUsenet(t)
	_, thisFile, _, _ := runtime.Caller(0)
	s.TemplatesDir = filepath.Join(filepath.Dir(thisFile), "templates")
	return s
}

// TestHandleArticle_NoUsenetThreadForRSS verifies that the thread context
// section does not appear for a regular RSS article.
func TestHandleArticle_NoUsenetThreadForRSS(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplates(t)
	ctx, user := testUser(t, s)

	feed := createFeed(t, s, user.ID, "RSS Feed", "http://rss.example.com")
	art := createArticle(t, s, feed.ID, "Plain RSS Article", "rss-g1")

	w := serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if strings.Contains(body, "usenet-thread-context") {
		t.Error("RSS article page must not contain usenet-thread-context section")
	}
}

// TestHandleArticle_UsenetThreadContext verifies that the thread context
// section appears for a Usenet article that has multiple thread members.
func TestHandleArticle_UsenetThreadContext(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplatesAndUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.lang.go")

	// Root article and one reply share the same root_message_id.
	root := "<root@test>"
	reply := "<reply@test>"
	q := dbgen.New(s.DB)

	// Insert root article.
	rootURL := "nntp://news.eternal-september.org/comp.lang.go/1"
	rootArt, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: feed.ID, Title: "Root question", Guid: root, Url: &rootURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		rootArt.ID, feed.ID, root, root, "comp.lang.go", 1)
	if err != nil {
		t.Fatal(err)
	}

	// Insert reply article (same root_message_id).
	replyURL := "nntp://news.eternal-september.org/comp.lang.go/2"
	replyArt, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: feed.ID, Title: "Re: Root question", Guid: reply, Url: &replyURL,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		replyArt.ID, feed.ID, reply, root, "comp.lang.go", 2)
	if err != nil {
		t.Fatal(err)
	}

	// Request the root article's page.
	w := serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", rootArt.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	if !strings.Contains(body, "usenet-thread-context") {
		t.Error("Usenet article page must contain usenet-thread-context section")
	}
	if !strings.Contains(body, "2 messages") {
		t.Errorf("expected '2 messages' in thread heading; body excerpt: %q",
			body[max(0, strings.Index(body, "usenet-thread")-50):])
	}
	if !strings.Contains(body, "Re: Root question") {
		t.Error("thread context must include the reply article title")
	}
	// The current article should be marked with the 'usenet-thread-current' class.
	if !strings.Contains(body, "usenet-thread-current") {
		t.Error("current article must have usenet-thread-current class")
	}
}

// TestHandleArticle_UsenetSingleArticleNoThread verifies that the thread
// context section is absent when the Usenet article has no thread siblings.
func TestHandleArticle_UsenetSingleArticleNoThread(t *testing.T) {
	t.Parallel()
	s := testServerWithTemplatesAndUsenet(t)
	ctx, user := testUser(t, s)

	feed := createNNTPFeed(t, s, user.ID, "comp.misc")
	art := createNNTPArticleWithMeta(t, s, &feed, "<solo@test>", 1)

	w := serveMux(t, "GET /articles/{id}", s.handleArticle,
		"GET", fmt.Sprintf("/articles/%d", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	body := w.Body.String()
	// Single-article thread should not show thread context.
	if strings.Contains(body, "usenet-thread-context") {
		t.Error("single-article Usenet post must not show thread context")
	}
}
