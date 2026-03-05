package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchPlaylistItems_SinglePage(t *testing.T) {
	resp := playlistItemsResponse{
		PageInfo: playlistPageInfo{TotalResults: 2},
		Items: []playlistItemEntry{
			makeEntry("vid1", "Video One", "Desc 1", "Author A", "2025-01-15T10:00:00Z", 0),
			makeEntry("vid2", "Video Two", "", "Author B", "2025-01-16T12:00:00Z", 1),
		},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("playlistId") != "PLtest" {
			t.Errorf("expected playlistId=PLtest, got %s", r.URL.Query().Get("playlistId"))
		}
		if r.URL.Query().Get("part") != "snippet" {
			t.Errorf("expected part=snippet, got %s", r.URL.Query().Get("part"))
		}
		if r.URL.Query().Get("maxResults") != "50" {
			t.Errorf("expected maxResults=50, got %s", r.URL.Query().Get("maxResults"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, total, err := c.FetchPlaylistItems(context.Background(), "PLtest", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("totalResults = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	// Check first item.
	it := items[0]
	if it.GUID != "yt:video:vid1" {
		t.Errorf("GUID = %q, want %q", it.GUID, "yt:video:vid1")
	}
	if it.Title != "Video One" {
		t.Errorf("Title = %q, want %q", it.Title, "Video One")
	}
	if it.URL != "https://www.youtube.com/watch?v=vid1" {
		t.Errorf("URL = %q", it.URL)
	}
	if it.Author != "Author A" {
		t.Errorf("Author = %q, want %q", it.Author, "Author A")
	}
	if it.ImageURL == "" {
		t.Error("ImageURL is empty")
	}
	if it.PublishedAt == nil {
		t.Error("PublishedAt is nil")
	}

	// Second item has no description — content should only have the iframe.
	it2 := items[1]
	if it2.GUID != "yt:video:vid2" {
		t.Errorf("GUID = %q", it2.GUID)
	}
}

func TestFetchPlaylistItems_Pagination(t *testing.T) {
	page1 := playlistItemsResponse{
		NextPageToken: "page2token",
		PageInfo:      playlistPageInfo{TotalResults: 3},
		Items: []playlistItemEntry{
			makeEntry("v1", "V1", "", "A", "2025-01-01T00:00:00Z", 0),
			makeEntry("v2", "V2", "", "A", "2025-01-02T00:00:00Z", 1),
		},
	}
	page2 := playlistItemsResponse{
		PageInfo: playlistPageInfo{TotalResults: 3},
		Items: []playlistItemEntry{
			makeEntry("v3", "V3", "", "A", "2025-01-03T00:00:00Z", 2),
		},
	}

	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "page2token" {
			_ = json.NewEncoder(w).Encode(page2)
		} else {
			_ = json.NewEncoder(w).Encode(page1)
		}
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, total, err := c.FetchPlaylistItems(context.Background(), "PLtest", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Errorf("totalResults = %d, want 3", total)
	}
	if len(items) != 3 {
		t.Errorf("got %d items, want 3", len(items))
	}
	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
}

func TestFetchPlaylistItems_MaxPages(t *testing.T) {
	resp := playlistItemsResponse{
		NextPageToken: "more",
		PageInfo:      playlistPageInfo{TotalResults: 100},
		Items: []playlistItemEntry{
			makeEntry("v1", "V1", "", "A", "2025-01-01T00:00:00Z", 0),
		},
	}

	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, _, err := c.FetchPlaylistItems(context.Background(), "PLtest", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("got %d items, want 2 (maxPages=2)", len(items))
	}
	if calls != 2 {
		t.Errorf("expected 2 API calls, got %d", calls)
	}
}

func TestFetchPlaylistItems_QuotaExceeded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    403,
				"message": "The request cannot be completed because you have exceeded your quota.",
				"errors": []map[string]any{
					{"reason": "quotaExceeded"},
				},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _, err := c.FetchPlaylistItems(context.Background(), "PLtest", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "youtube API quota exceeded"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", got, want)
	}
}

func TestFetchPlaylistItems_InvalidKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    400,
				"message": "API key not valid.",
				"errors": []map[string]any{
					{"reason": "keyInvalid"},
				},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _, err := c.FetchPlaylistItems(context.Background(), "PLtest", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "youtube API key invalid"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", got, want)
	}
}

func TestFetchPlaylistItems_PlaylistNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code":    404,
				"message": "The playlist identified with the request's playlistId parameter cannot be found.",
				"errors": []map[string]any{
					{"reason": "playlistNotFound"},
				},
			},
		})
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, _, err := c.FetchPlaylistItems(context.Background(), "PLbogus", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "youtube playlist not found"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error = %q, want prefix %q", got, want)
	}
}

func TestConvertItem_Content(t *testing.T) {
	entry := makeEntry("abc123", "Test Video", "Some description", "Channel", "2025-06-01T00:00:00Z", 0)
	fi := convertItem(&entry)

	if fi.Content == "" {
		t.Fatal("content is empty")
	}
	// Should contain iframe embed.
	if want := `src="https://www.youtube.com/embed/abc123"`; !contains(fi.Content, want) {
		t.Errorf("content missing iframe embed: %s", fi.Content)
	}
	// Should contain description.
	if !contains(fi.Content, "Some description") {
		t.Errorf("content missing description: %s", fi.Content)
	}
}

func TestConvertItem_NoDescription(t *testing.T) {
	entry := makeEntry("xyz", "No Desc", "", "Ch", "2025-01-01T00:00:00Z", 0)
	fi := convertItem(&entry)
	// Should have iframe but no <p> tag.
	if contains(fi.Content, "<p>") {
		t.Errorf("expected no <p> tag for empty description, got: %s", fi.Content)
	}
}

func TestBestThumbnail(t *testing.T) {
	tests := []struct {
		name string
		tm   thumbnailMap
		want string
	}{
		{"high", thumbnailMap{High: &thumbnail{URL: "high.jpg"}, Medium: &thumbnail{URL: "med.jpg"}}, "high.jpg"},
		{"medium", thumbnailMap{Medium: &thumbnail{URL: "med.jpg"}, Default: &thumbnail{URL: "def.jpg"}}, "med.jpg"},
		{"default", thumbnailMap{Default: &thumbnail{URL: "def.jpg"}}, "def.jpg"},
		{"none", thumbnailMap{}, ""},
	}
	for _, tt := range tests {
		if got := bestThumbnail(tt.tm); got != tt.want {
			t.Errorf("%s: bestThumbnail = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// --- helpers ---

func newTestClient(ts *httptest.Server) *Client {
	c := NewClient("test-api-key")
	c.SetHTTPClient(ts.Client())
	// Override the base URL by replacing fetchPage with a version that hits the test server.
	// Since we can't easily swap the base URL, we use a custom transport.
	originalTransport := ts.Client().Transport
	c.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		// Rewrite the request URL to point at the test server.
		testURL := ts.URL + req.URL.Path + "?" + req.URL.RawQuery
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, testURL, req.Body)
		if err != nil {
			return nil, err
		}
		newReq.Header = req.Header
		if originalTransport != nil {
			return originalTransport.RoundTrip(newReq)
		}
		return http.DefaultTransport.RoundTrip(newReq)
	})
	return c
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func makeEntry(videoID, title, desc, author, published string, position int) playlistItemEntry {
	return playlistItemEntry{
		Snippet: playlistSnippet{
			PublishedAt:            published,
			Title:                  title,
			Description:            desc,
			VideoOwnerChannelTitle: author,
			Position:               position,
			ResourceID:             playlistResource{VideoID: videoID},
			Thumbnails: thumbnailMap{
				High: &thumbnail{URL: "https://i.ytimg.com/vi/" + videoID + "/hqdefault.jpg"},
			},
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || s != "" && strings.Contains(s, substr))
}
