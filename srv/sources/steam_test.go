package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// rewriteTransport rewrites requests to the Steam API to a test server.
type rewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
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

func withMockSteamClient(t *testing.T, ts *httptest.Server) {
	t.Helper()
	orig := steamClient
	steamClient = &http.Client{
		Timeout:   4 * time.Second,
		Transport: &rewriteTransport{base: http.DefaultTransport, target: ts.URL},
	}
	t.Cleanup(func() { steamClient = orig })
}

func TestSteamSource_Match(t *testing.T) {
	s := SteamSource{}
	tests := []struct {
		url, feedType string
		want          bool
	}{
		{"https://store.steampowered.com/news/app/440", "rss", true},
		{"https://store.steampowered.com/feeds/news/app/440", "rss", true},
		{"https://example.com/feed", "rss", false},
		{"https://store.steampowered.com/news/app/440", "huggingface", false},
	}
	for _, tt := range tests {
		if got := s.Match(tt.url, tt.feedType); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.url, tt.feedType, got, tt.want)
		}
	}
}

func TestSteamSource_NormalizeURL(t *testing.T) {
	s := SteamSource{}
	tests := []struct {
		input, want string
	}{
		{"https://store.steampowered.com/news/app/4115450", "https://store.steampowered.com/feeds/news/app/4115450"},
		{"https://store.steampowered.com/news/app/4115450/", "https://store.steampowered.com/feeds/news/app/4115450"},
		{"https://store.steampowered.com/feeds/news/app/440", "https://store.steampowered.com/feeds/news/app/440"},
		{"https://example.com/feed", "https://example.com/feed"},
		{"", ""},
	}
	for _, tt := range tests {
		got, err := s.NormalizeURL(context.Background(), tt.input)
		if err != nil {
			t.Errorf("NormalizeURL(%q) error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSteamSource_ResolveName_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appIDs := r.URL.Query().Get("appids")
		resp := map[string]any{
			appIDs: map[string]any{
				"success": true,
				"data":    map[string]any{"name": "Team Fortress 2"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	withMockSteamClient(t, ts)

	s := SteamSource{}
	name := s.ResolveName(context.Background(), "https://store.steampowered.com/feeds/news/app/440", "")
	if name != "Team Fortress 2" {
		t.Errorf("ResolveName = %q, want %q", name, "Team Fortress 2")
	}
}

func TestSteamSource_ResolveName_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appIDs := r.URL.Query().Get("appids")
		resp := map[string]any{appIDs: map[string]any{"success": false}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	withMockSteamClient(t, ts)

	s := SteamSource{}
	name := s.ResolveName(context.Background(), "https://store.steampowered.com/feeds/news/app/9999999", "")
	if name != "" {
		t.Errorf("ResolveName = %q, want empty", name)
	}
}

func TestSteamSource_ResolveName_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json at all"))
	}))
	defer ts.Close()
	withMockSteamClient(t, ts)

	s := SteamSource{}
	name := s.ResolveName(context.Background(), "https://store.steampowered.com/feeds/news/app/440", "")
	if name != "" {
		t.Errorf("ResolveName = %q, want empty", name)
	}
}

func TestSteamSource_ResolveName_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer ts.Close()
	withMockSteamClient(t, ts)

	s := SteamSource{}
	name := s.ResolveName(context.Background(), "https://store.steampowered.com/feeds/news/app/440", "")
	if name != "" {
		t.Errorf("ResolveName = %q, want empty", name)
	}
}

func TestSteamSource_FeedType(t *testing.T) {
	if ft := (SteamSource{}).FeedType(); ft != "" {
		t.Errorf("FeedType() = %q, want empty", ft)
	}
}
