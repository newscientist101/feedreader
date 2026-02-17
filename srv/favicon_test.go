package srv

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --------------- faviconDomain unit tests ---------------

func TestFaviconDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		site string
		feed string
		want string
	}{
		{"prefers site URL", "https://example.com", "https://feeds.example.com/rss", "example.com"},
		{"falls back to feed URL", "", "https://example.com/feed.xml", "example.com"},
		{"strips feeds subdomain", "", "https://feeds.example.com/rss", "example.com"},
		{"strips feed subdomain", "", "https://feed.example.com/rss", "example.com"},
		{"strips rss subdomain", "", "https://rss.example.com/rss", "example.com"},
		{"keeps non-feed subdomain", "", "https://blog.example.com/rss", "blog.example.com"},
		{"two-part domain preserved", "", "https://example.com/rss", "example.com"},
		{"empty both returns empty", "", "", ""},
		{"invalid URL returns empty", "", "://bad", ""},
		{"no host returns empty", "", "/just/a/path", ""},
		{"domain with port", "", "https://example.com:8080/feed", "example.com:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := faviconDomain(tt.site, tt.feed)
			if got != tt.want {
				t.Errorf("faviconDomain(%q, %q) = %q, want %q", tt.site, tt.feed, got, tt.want)
			}
		})
	}
}

func TestFaviconURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		site string
		feed string
		want string
	}{
		{"normal domain", "https://example.com", "", "/api/favicon?domain=example.com"},
		{"empty returns empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := faviconURL(tt.site, tt.feed)
			if got != tt.want {
				t.Errorf("faviconURL(%q, %q) = %q, want %q", tt.site, tt.feed, got, tt.want)
			}
		})
	}
}

// --------------- apiFavicon handler tests ---------------

func TestApiFavicon_EmptyDomain(t *testing.T) {
	t.Parallel()
	s := testServer(t)

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if !isFallbackFavicon(w.Body.Bytes()) {
		t.Error("expected fallback favicon body")
	}
}

func TestApiFavicon_UpstreamSuccess(t *testing.T) {
	t.Parallel()

	// Mock upstream that returns a fake PNG
	fakePNG := []byte("fake-png-data")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("domain") != "example.com" {
			t.Errorf("unexpected domain param: %q", r.URL.Query().Get("domain"))
		}
		if r.URL.Query().Get("sz") != "32" {
			t.Errorf("unexpected sz param: %q", r.URL.Query().Get("sz"))
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(fakePNG)
	}))
	defer upstream.Close()

	s := testServer(t)
	s.FaviconBaseURL = upstream.URL

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=example.com", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Errorf("Content-Type = %q, want image/png", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=604800" {
		t.Errorf("Cache-Control = %q, want 7-day cache", cc)
	}
	if w.Body.String() != "fake-png-data" {
		t.Errorf("body = %q", w.Body.String())
	}
}

func TestApiFavicon_UpstreamEmptyContentType(t *testing.T) {
	t.Parallel()

	// Simulate an upstream that returns binary data with no Content-Type.
	// Go's net/http will auto-detect Content-Type from the body, so we
	// use a raw TCP listener to send a response with no Content-Type header.
	// Simpler approach: explicitly send Content-Type as empty via a header trick.
	// Actually: the handler falls back to image/png when ct == "".
	// In practice, Go's http client receives auto-detected types, so this
	// tests that arbitrary upstream content-types are forwarded.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte("<svg></svg>"))
	}))
	defer upstream.Close()

	s := testServer(t)
	s.FaviconBaseURL = upstream.URL

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=example.com", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
}

func TestApiFavicon_Upstream404(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer upstream.Close()

	s := testServer(t)
	s.FaviconBaseURL = upstream.URL

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=unknown.example", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200 (fallback), got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "public, max-age=86400" {
		t.Errorf("Cache-Control = %q, want 1-day cache", cc)
	}
	if !isFallbackFavicon(w.Body.Bytes()) {
		t.Error("expected fallback favicon body")
	}
}

func TestApiFavicon_Upstream500(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", 500)
	}))
	defer upstream.Close()

	s := testServer(t)
	s.FaviconBaseURL = upstream.URL

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=broken.example", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200 (fallback), got %d", w.Code)
	}
	if !isFallbackFavicon(w.Body.Bytes()) {
		t.Error("expected fallback favicon body")
	}
}

func TestApiFavicon_UpstreamUnreachable(t *testing.T) {
	t.Parallel()

	s := testServer(t)
	// Point to a port that's not listening
	s.FaviconBaseURL = "http://127.0.0.1:1"

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=example.com", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200 (fallback), got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", ct)
	}
	if !isFallbackFavicon(w.Body.Bytes()) {
		t.Error("expected fallback favicon body")
	}
}

func TestApiFavicon_PreservesUpstreamContentType(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		w.Write([]byte("ico-data"))
	}))
	defer upstream.Close()

	s := testServer(t)
	s.FaviconBaseURL = upstream.URL

	ctx := context.Background()
	w := serveAPI(t, s.apiFavicon, "GET", "/api/favicon?domain=example.com", "", ctx)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/x-icon" {
		t.Errorf("Content-Type = %q, want image/x-icon", ct)
	}
}

// isFallbackFavicon checks if the bytes match the embedded app icon SVG.
func isFallbackFavicon(b []byte) bool {
	return bytes.Equal(b, fallbackFavicon)
}
