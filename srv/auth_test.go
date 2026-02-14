package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetUser_NoUser(t *testing.T) {
	ctx := httptest.NewRequest("GET", "/", nil).Context()
	if u := GetUser(ctx); u != nil {
		t.Errorf("expected nil, got %+v", u)
	}
}

func TestAuthMiddleware_StaticBypass(t *testing.T) {
	s := testServer(t)
	called := false
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/static/app.js", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if !called {
		t.Error("static route should pass through without auth")
	}
}

func TestAuthMiddleware_NoHeaders_APIReturns401(t *testing.T) {
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_NoHeaders_PageRedirects(t *testing.T) {
	s := testServer(t)
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/feeds", nil)
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
	s := testServer(t)
	var gotUser *User
	handler := s.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser = GetUser(r.Context())
		w.WriteHeader(200)
	}))

	r := httptest.NewRequest("GET", "/api/feeds", nil)
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
