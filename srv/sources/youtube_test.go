package sources

import (
	"context"
	"encoding/xml"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type ytRewriteTransport struct {
	base   http.RoundTripper
	target string
}

func (t *ytRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == "www.youtube.com" {
		newURL := t.target + req.URL.Path + "?" + req.URL.RawQuery
		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, newURL, req.Body)
		if err != nil {
			return nil, err
		}
		maps.Copy(newReq.Header, req.Header)
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}

func withMockYouTubeClient(t *testing.T, ts *httptest.Server) {
	t.Helper()
	orig := youtubeClient
	youtubeClient = &http.Client{
		Timeout:   4 * time.Second,
		Transport: &ytRewriteTransport{base: http.DefaultTransport, target: ts.URL},
	}
	t.Cleanup(func() { youtubeClient = orig })
}

func TestYouTubeSource_Match(t *testing.T) {
	s := YouTubeSource{}
	tests := []struct {
		url, feedType string
		want          bool
	}{
		{"https://www.youtube.com/playlist?list=PLxyz", "youtube", true},
		{"https://www.youtube.com/@handle", "youtube", true},
		{"https://www.youtube.com/channel/UCxyz", "youtube", true},
		{"https://www.youtube.com/feeds/videos.xml?channel_id=UCxyz", "youtube", true},
		{"https://www.youtube.com/playlist?list=PLxyz", "rss", false},
		{"https://example.com/feed", "youtube", false},
	}
	for _, tt := range tests {
		if got := s.Match(tt.url, tt.feedType); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.url, tt.feedType, got, tt.want)
		}
	}
}

func TestYouTubeSource_NormalizeURL_Playlist(t *testing.T) {
	s := YouTubeSource{}
	got, err := s.NormalizeURL(context.Background(),
		"https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.youtube.com/feeds/videos.xml?playlist_id=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"
	if got != want {
		t.Errorf("NormalizeURL = %q, want %q", got, want)
	}
}

func TestYouTubeSource_NormalizeURL_Channel(t *testing.T) {
	s := YouTubeSource{}
	got, err := s.NormalizeURL(context.Background(),
		"https://www.youtube.com/channel/UCddiUEpeqJcYeBxX1IVBKvQ")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.youtube.com/feeds/videos.xml?channel_id=UCddiUEpeqJcYeBxX1IVBKvQ"
	if got != want {
		t.Errorf("NormalizeURL = %q, want %q", got, want)
	}
}

func TestYouTubeSource_NormalizeURL_FeedAlreadyValid(t *testing.T) {
	s := YouTubeSource{}
	url := "https://www.youtube.com/feeds/videos.xml?channel_id=UCxyz"
	got, err := s.NormalizeURL(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if got != url {
		t.Errorf("NormalizeURL changed valid feed URL: %q -> %q", url, got)
	}
}

func TestYouTubeSource_NormalizeURL_Handle(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><link rel="canonical" href="https://www.youtube.com/channel/UCddiUEpeqJcYeBxX1IVBKvQ"></head></html>`)
	}))
	defer ts.Close()
	withMockYouTubeClient(t, ts)

	s := YouTubeSource{}
	got, err := s.NormalizeURL(context.Background(), "https://www.youtube.com/@TestChannel")
	if err != nil {
		t.Fatal(err)
	}
	want := "https://www.youtube.com/feeds/videos.xml?channel_id=UCddiUEpeqJcYeBxX1IVBKvQ"
	if got != want {
		t.Errorf("NormalizeURL = %q, want %q", got, want)
	}
}

func TestYouTubeSource_NormalizeURL_HandleNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer ts.Close()
	withMockYouTubeClient(t, ts)

	s := YouTubeSource{}
	_, err := s.NormalizeURL(context.Background(), "https://www.youtube.com/@NonexistentChannel")
	if err == nil {
		t.Error("expected error for 404")
	}
}

func TestYouTubeSource_ResolveName(t *testing.T) {
	type atomFeed struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		data, _ := xml.Marshal(atomFeed{Title: "PBS Space Time"})
		_, _ = w.Write(data)
	}))
	defer ts.Close()
	withMockYouTubeClient(t, ts)

	s := YouTubeSource{}
	name := s.ResolveName(context.Background(),
		"https://www.youtube.com/feeds/videos.xml?channel_id=UCxyz", "")
	if name != "PBS Space Time" {
		t.Errorf("ResolveName = %q, want %q", name, "PBS Space Time")
	}
}

func TestYouTubeSource_ResolveName_NonFeedURL(t *testing.T) {
	s := YouTubeSource{}
	name := s.ResolveName(context.Background(), "https://www.youtube.com/@TestChannel", "")
	if name != "" {
		t.Errorf("ResolveName for non-feed URL = %q, want empty", name)
	}
}

func TestYouTubeSource_FeedType(t *testing.T) {
	if ft := (YouTubeSource{}).FeedType(); ft != "rss" {
		t.Errorf("FeedType() = %q, want %q", ft, "rss")
	}
}

func TestParseYouTubeURL(t *testing.T) {
	tests := []struct {
		url      string
		wantKind string
		wantID   string
		wantOK   bool
	}{
		{"https://www.youtube.com/playlist?list=PLxyz", "playlist", "PLxyz", true},
		{"https://www.youtube.com/channel/UCxyz", "channel", "UCxyz", true},
		{"https://www.youtube.com/@handle", "handle", "handle", true},
		{"https://www.youtube.com/c/CustomName", "custom", "CustomName", true},
		{"https://www.youtube.com/user/username", "user", "username", true},
		{"https://www.youtube.com/feeds/videos.xml?channel_id=UCxyz", "feed", "UCxyz", true},
		{"https://www.youtube.com/feeds/videos.xml?playlist_id=PLxyz", "feed", "PLxyz", true},
		{"https://example.com/feed", "", "", false},
	}
	for _, tt := range tests {
		kind, id, ok := ParseYouTubeURL(tt.url)
		if ok != tt.wantOK || kind != tt.wantKind || id != tt.wantID {
			t.Errorf("ParseYouTubeURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.url, kind, id, ok, tt.wantKind, tt.wantID, tt.wantOK)
		}
	}
}
