package srv

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

// --------------- Article actions ---------------

func TestHandlerMarkRead(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	w := serveMux(t, "POST /api/articles/{id}/read", s.apiMarkRead,
		"POST", fmt.Sprintf("/api/articles/%d/read", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	if m := jsonBody(t, w); m["status"] != "ok" {
		t.Fatalf("body: %v", m)
	}

	// bad id
	w = serveMux(t, "POST /api/articles/{id}/read", s.apiMarkRead,
		"POST", "/api/articles/abc/read", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerMarkUnread(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	w := serveMux(t, "POST /api/articles/{id}/unread", s.apiMarkUnread,
		"POST", fmt.Sprintf("/api/articles/%d/unread", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandlerToggleStar(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	w := serveMux(t, "POST /api/articles/{id}/star", s.apiToggleStar,
		"POST", fmt.Sprintf("/api/articles/%d/star", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	w = serveMux(t, "POST /api/articles/{id}/star", s.apiToggleStar,
		"POST", "/api/articles/nope/star", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerToggleQueue(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	// add
	w := serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m := jsonBody(t, w)
	if m["queued"] != true {
		t.Fatalf("expected queued=true, got %v", m)
	}

	// toggle off
	w = serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	m = jsonBody(t, w)
	if m["queued"] != false {
		t.Fatalf("expected queued=false, got %v", m)
	}
}

func TestHandlerRemoveFromQueue(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	// add first, then remove
	serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)

	w := serveMux(t, "DELETE /api/articles/{id}/queue", s.apiRemoveFromQueue,
		"DELETE", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

func TestHandlerListQueue(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	art := createArticle(t, s, feed.ID, "a", "g1")

	// queue one article
	serveMux(t, "POST /api/articles/{id}/queue", s.apiToggleQueue,
		"POST", fmt.Sprintf("/api/articles/%d/queue", art.ID), "", ctx)

	w := serveAPI(t, s.apiListQueue, "GET", "/api/queue", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var arr []any
	if err := json.Unmarshal(w.Body.Bytes(), &arr); err != nil {
		t.Fatal(err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 queued, got %d", len(arr))
	}
}

func TestHandlerMarkAllRead(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "a", "g1")

	for _, age := range []string{"", "day", "week"} {
		target := "/api/articles/read-all"
		if age != "" {
			target += "?age=" + age
		}
		w := serveAPI(t, s.apiMarkAllRead, "POST", target, "", ctx)
		if w.Code != 200 {
			t.Fatalf("age=%q got %d", age, w.Code)
		}
	}
}

func TestHandlerMarkFeedRead(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "a", "g1")

	w := serveMux(t, "POST /api/feeds/{id}/read-all", s.apiMarkFeedRead,
		"POST", fmt.Sprintf("/api/feeds/%d/read-all", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	w = serveMux(t, "POST /api/feeds/{id}/read-all", s.apiMarkFeedRead,
		"POST", "/api/feeds/bad/read-all", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- Feed CRUD ---------------

func TestHandlerDeleteFeed(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	w := serveMux(t, "DELETE /api/feeds/{id}", s.apiDeleteFeed,
		"DELETE", fmt.Sprintf("/api/feeds/%d", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// verify it's gone
	w = serveMux(t, "GET /api/feeds/{id}", s.apiGetFeed,
		"GET", fmt.Sprintf("/api/feeds/%d", feed.ID), "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestHandlerGetFeed(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "myfeed", "http://f")

	w := serveMux(t, "GET /api/feeds/{id}", s.apiGetFeed,
		"GET", fmt.Sprintf("/api/feeds/%d", feed.ID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m := jsonBody(t, w)
	if m["name"] != "myfeed" {
		t.Fatalf("unexpected body: %v", m)
	}

	// not found
	w = serveMux(t, "GET /api/feeds/{id}", s.apiGetFeed,
		"GET", "/api/feeds/99999", "", ctx)
	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// bad id
	w = serveMux(t, "GET /api/feeds/{id}", s.apiGetFeed,
		"GET", "/api/feeds/xyz", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- Search ---------------

func TestHandlerSearch(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "Golang news", "g1")

	// empty q returns []
	w := serveAPI(t, s.apiSearch, "GET", "/api/search?q=", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	var arr []any
	json.Unmarshal(w.Body.Bytes(), &arr)
	if len(arr) != 0 {
		t.Fatalf("expected empty, got %d", len(arr))
	}

	// search with query
	w = serveAPI(t, s.apiSearch, "GET", "/api/search?q=Golang", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
}

// --------------- Counts ---------------

func TestHandlerGetCounts(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")
	createArticle(t, s, feed.ID, "a", "g1")

	w := serveAPI(t, s.apiGetCounts, "GET", "/api/counts", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}
	m := jsonBody(t, w)
	for _, key := range []string{"unread", "starred", "queue", "feeds", "categories"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing key %q in counts", key)
		}
	}
}

// --------------- Settings ---------------

func TestHandlerSettings(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// GET empty settings
	w := serveAPI(t, s.apiGetSettings, "GET", "/api/settings", "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// PUT valid setting
	w = serveAPI(t, s.apiUpdateSettings, "PUT", "/api/settings",
		`{"autoMarkRead":"true"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// verify it was saved
	w = serveAPI(t, s.apiGetSettings, "GET", "/api/settings", "", ctx)
	m := jsonBody(t, w)
	if m["autoMarkRead"] != "true" {
		t.Fatalf("setting not saved: %v", m)
	}

	// PUT unknown key
	w = serveAPI(t, s.apiUpdateSettings, "PUT", "/api/settings",
		`{"bogusKey":"val"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for unknown key, got %d", w.Code)
	}

	// PUT invalid value
	w = serveAPI(t, s.apiUpdateSettings, "PUT", "/api/settings",
		`{"autoMarkRead":"maybe"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for invalid value, got %d", w.Code)
	}
}

func TestHandlerSettingsAllKeys(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	tests := []struct {
		key, value string
		wantCode   int
	}{
		{"hideReadArticles", "hide", 200},
		{"hideEmptyFeeds", "show", 200},
		{"defaultFolderView", "card", 200},
		{"defaultFeedView", "list", 200},
		{"defaultView", "magazine", 200},
		{"defaultView", "badval", 400},
	}
	for _, tt := range tests {
		t.Run(tt.key+"="+tt.value, func(t *testing.T) {
			body := fmt.Sprintf(`{"%s":"%s"}`, tt.key, tt.value)
			w := serveAPI(t, s.apiUpdateSettings, "PUT", "/api/settings", body, ctx)
			if w.Code != tt.wantCode {
				t.Fatalf("got %d, want %d: %s", w.Code, tt.wantCode, w.Body.String())
			}
		})
	}
}

// --------------- Categories ---------------

func TestHandlerCategoryCRUD(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// create
	w := serveAPI(t, s.apiCreateCategory, "POST", "/api/categories",
		`{"name":"Tech"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("create got %d: %s", w.Code, w.Body.String())
	}
	m := jsonBody(t, w)
	catID := m["id"].(float64)
	if m["name"] != "Tech" {
		t.Fatalf("unexpected: %v", m)
	}

	// create with empty name
	w = serveAPI(t, s.apiCreateCategory, "POST", "/api/categories",
		`{"name":""}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for empty name, got %d", w.Code)
	}

	// update
	w = serveMux(t, "PUT /api/categories/{id}", s.apiUpdateCategory,
		"PUT", fmt.Sprintf("/api/categories/%d", int64(catID)), `{"name":"Science"}`, ctx)
	if w.Code != 200 {
		t.Fatalf("update got %d: %s", w.Code, w.Body.String())
	}

	// update bad id
	w = serveMux(t, "PUT /api/categories/{id}", s.apiUpdateCategory,
		"PUT", "/api/categories/abc", `{"name":"X"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// delete
	w = serveMux(t, "DELETE /api/categories/{id}", s.apiDeleteCategory,
		"DELETE", fmt.Sprintf("/api/categories/%d", int64(catID)), "", ctx)
	if w.Code != 200 {
		t.Fatalf("delete got %d", w.Code)
	}

	// delete bad id
	w = serveMux(t, "DELETE /api/categories/{id}", s.apiDeleteCategory,
		"DELETE", "/api/categories/bad", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerSetFeedCategory(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	// create category
	w := serveAPI(t, s.apiCreateCategory, "POST", "/api/categories",
		`{"name":"Cat1"}`, ctx)
	catID := int64(jsonBody(t, w)["id"].(float64))

	// assign feed to category
	w = serveMux(t, "POST /api/feeds/{id}/category", s.apiSetFeedCategory,
		"POST", fmt.Sprintf("/api/feeds/%d/category", feed.ID),
		fmt.Sprintf(`{"categoryId":%d}`, catID), ctx)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	// bad id
	w = serveMux(t, "POST /api/feeds/{id}/category", s.apiSetFeedCategory,
		"POST", "/api/feeds/xyz/category", `{"categoryId":1}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandlerMarkCategoryRead(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	_ = createFeed(t, s, user.ID, "f", "http://f")

	w := serveAPI(t, s.apiCreateCategory, "POST", "/api/categories",
		`{"name":"Cat1"}`, ctx)
	catID := int64(jsonBody(t, w)["id"].(float64))

	for _, age := range []string{"", "day", "week"} {
		target := fmt.Sprintf("/api/categories/%d/read-all", catID)
		if age != "" {
			target += "?age=" + age
		}
		w = serveMux(t, "POST /api/categories/{id}/read-all", s.apiMarkCategoryRead,
			"POST", target, "", ctx)
		if w.Code != 200 {
			t.Fatalf("age=%q got %d", age, w.Code)
		}
	}
}

// --------------- Exclusions ---------------

func TestHandlerExclusions(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	// create category
	w := serveAPI(t, s.apiCreateCategory, "POST", "/api/categories",
		`{"name":"Cat"}`, ctx)
	catID := int64(jsonBody(t, w)["id"].(float64))

	// list empty
	w = serveMux(t, "GET /api/categories/{id}/exclusions", s.apiListExclusions,
		"GET", fmt.Sprintf("/api/categories/%d/exclusions", catID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("got %d", w.Code)
	}

	// create exclusion
	w = serveMux(t, "POST /api/categories/{id}/exclusions", s.apiCreateExclusion,
		"POST", fmt.Sprintf("/api/categories/%d/exclusions", catID),
		`{"type":"keyword","pattern":"spam","isRegex":false}`, ctx)
	if w.Code != 200 {
		t.Fatalf("create exclusion got %d: %s", w.Code, w.Body.String())
	}
	exclID := int64(jsonBody(t, w)["id"].(float64))

	// invalid type
	w = serveMux(t, "POST /api/categories/{id}/exclusions", s.apiCreateExclusion,
		"POST", fmt.Sprintf("/api/categories/%d/exclusions", catID),
		`{"type":"bad","pattern":"x"}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for bad type, got %d", w.Code)
	}

	// empty pattern
	w = serveMux(t, "POST /api/categories/{id}/exclusions", s.apiCreateExclusion,
		"POST", fmt.Sprintf("/api/categories/%d/exclusions", catID),
		`{"type":"author","pattern":""}`, ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400 for empty pattern, got %d", w.Code)
	}

	// delete
	w = serveMux(t, "DELETE /api/exclusions/{id}", s.apiDeleteExclusion,
		"DELETE", fmt.Sprintf("/api/exclusions/%d", exclID), "", ctx)
	if w.Code != 200 {
		t.Fatalf("delete got %d", w.Code)
	}

	// delete bad id
	w = serveMux(t, "DELETE /api/exclusions/{id}", s.apiDeleteExclusion,
		"DELETE", "/api/exclusions/nope", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// list bad category id
	w = serveMux(t, "GET /api/categories/{id}/exclusions", s.apiListExclusions,
		"GET", "/api/categories/bad/exclusions", "", ctx)
	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --------------- AI Status ---------------

func TestHandlerAIStatus(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, _ := testUser(t, s)

	w := serveAPI(t, s.apiAIStatus, "GET", "/api/ai/status", "", ctx)
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	m := jsonBody(t, w)
	if _, ok := m["available"]; !ok {
		t.Fatalf("missing 'available' key in response: %v", m)
	}
	// available is a bool; value depends on whether Shelley is running
	if _, ok := m["available"].(bool); !ok {
		t.Fatalf("expected bool for 'available', got %T", m["available"])
	}
}
