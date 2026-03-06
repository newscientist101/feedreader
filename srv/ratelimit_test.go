package srv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitMiddleware_PassesGET(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/counts", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET should pass, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_PassesNonAPI(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	req := httptest.NewRequest(http.MethodPost, "/login", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("non-API POST should pass, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_NoUser(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	// POST to /api without user context should pass through
	req := httptest.NewRequest(http.MethodPost, "/api/feeds", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("no user should pass through, got %d", rr.Code)
	}
}

func TestRateLimitMiddleware_AllowsNormalTraffic(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	user := &User{ID: 1, ExternalID: "test", Email: "test@test.com"}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	// First few requests should succeed (burst=20)
	for i := range 15 {
		req := httptest.NewRequest(http.MethodPost, "/api/feeds", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("request %d should succeed, got %d", i, rr.Code)
		}
	}
}

func TestRateLimitMiddleware_BlocksExcessiveTraffic(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	user := &User{ID: 2, ExternalID: "test2", Email: "test2@test.com"}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	// Exhaust burst (20) + some extra
	blocked := false
	for range 30 {
		req := httptest.NewRequest(http.MethodPost, "/api/feeds", http.NoBody)
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code == http.StatusTooManyRequests {
			blocked = true
			// Verify response headers and body
			if rr.Header().Get("Retry-After") == "" {
				t.Error("missing Retry-After header")
			}
			if rr.Header().Get("Content-Type") != "application/json" {
				t.Error("expected JSON content type")
			}
			break
		}
	}
	if !blocked {
		t.Error("expected rate limit to kick in after burst")
	}
}

func TestRateLimitMiddleware_DifferentUsersIndependent(t *testing.T) {
	rl := newRateLimiter()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := rateLimitMiddleware(rl)(inner)

	user1 := &User{ID: 10, ExternalID: "u1", Email: "u1@test.com"}
	user2 := &User{ID: 11, ExternalID: "u2", Email: "u2@test.com"}
	ctx1 := context.WithValue(context.Background(), userContextKey, user1)
	ctx2 := context.WithValue(context.Background(), userContextKey, user2)

	// Exhaust user1's burst
	for range 25 {
		req := httptest.NewRequest(http.MethodPost, "/api/feeds", http.NoBody)
		req = req.WithContext(ctx1)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
	}

	// User2 should still be fine
	req := httptest.NewRequest(http.MethodPost, "/api/feeds", http.NoBody)
	req = req.WithContext(ctx2)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("user2 should not be affected by user1's limit, got %d", rr.Code)
	}
}
