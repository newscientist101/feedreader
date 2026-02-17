package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser_NoUser(t *testing.T) {
	ctx := httptest.NewRequest("GET", "/", http.NoBody).Context()
	if u := GetUser(ctx); u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}

func TestAuthMiddleware_StaticBypass(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	called := false
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/static/app.js", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !called {
		t.Error("static route should pass through without auth")
	}
}

func TestAuthMiddleware_NoHeaders_APIReturns401(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoHeaders_PageRedirects(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/feeds", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/__exe.dev/login?redirect=/feeds" {
		t.Errorf("redirect location = %q", loc)
	}
}

func TestAuthMiddleware_WithHeaders(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "ext-123")
	r.Header.Set("X-Exedev-Email", "user@example.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotUser == nil {
		t.Fatal("user not set in context")
	}
	if gotUser.ExternalID != "ext-123" {
		t.Errorf("external_id = %q", gotUser.ExternalID)
	}
	if gotUser.Email != "user@example.com" {
		t.Errorf("email = %q", gotUser.Email)
	}
}

func TestAuthMiddleware_CachesUser(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var callCount int
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		u := GetUser(r.Context())
		if u == nil {
			t.Error("user should be set")
		}
		w.WriteHeader(200)
	}))

	// First request — populates cache
	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "cache-test-user")
	r.Header.Set("X-Exedev-Email", "cache@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("first request: got %d", w.Code)
	}

	// Second request — should use cache (same user)
	r = httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "cache-test-user")
	r.Header.Set("X-Exedev-Email", "cache@test.com")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("second request: got %d", w.Code)
	}

	if callCount != 2 {
		t.Fatalf("expected handler called 2 times, got %d", callCount)
	}
}

func TestAuthMiddleware_CacheReturnsCorrectUser(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	var users []*User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		users = append(users, GetUser(r.Context()))
		w.WriteHeader(200)
	}))

	// Two different users
	for _, uid := range []string{"user-a", "user-b"} {
		r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
		r.Header.Set("X-Exedev-Userid", uid)
		r.Header.Set("X-Exedev-Email", uid+"@test.com")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s: got %d", uid, w.Code)
		}
	}

	if len(users) != 2 {
		t.Fatalf("expected 2, got %d", len(users))
	}
	if users[0].ExternalID != "user-a" {
		t.Errorf("user[0] = %q", users[0].ExternalID)
	}
	if users[1].ExternalID != "user-b" {
		t.Errorf("user[1] = %q", users[1].ExternalID)
	}
	if users[0].ID == users[1].ID {
		t.Error("different external IDs should have different DB IDs")
	}

	// Second request for user-a — should return same ID from cache
	r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
	r.Header.Set("X-Exedev-Userid", "user-a")
	r.Header.Set("X-Exedev-Email", "user-a@test.com")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if users[2].ID != users[0].ID {
		t.Errorf("cached user-a ID = %d, want %d", users[2].ID, users[0].ID)
	}
}

func TestAuthMiddleware_CachePerServer(t *testing.T) {
	// Each server's AuthMiddleware should have its own cache — ensures
	// parallel tests with different DBs don't interfere.
	t.Parallel()
	s1 := testServer(t)
	s2 := testServer(t)

	var user1, user2 *User
	h1 := s1.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user1 = GetUser(r.Context())
		w.WriteHeader(200)
	}))
	h2 := s2.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user2 = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	// Same external ID, different server DBs
	for _, h := range []http.Handler{h1, h2} {
		r := httptest.NewRequest("GET", "/api/feeds", http.NoBody)
		r.Header.Set("X-Exedev-Userid", "shared-ext-id")
		r.Header.Set("X-Exedev-Email", "shared@test.com")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("got %d", w.Code)
		}
	}

	if user1 == nil || user2 == nil {
		t.Fatal("users should be set")
	}
	// Both should be valid users in their respective DBs
	if user1.ExternalID != "shared-ext-id" || user2.ExternalID != "shared-ext-id" {
		t.Error("external IDs should match")
	}
}
