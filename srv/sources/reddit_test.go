package sources

import (
	"context"
	"testing"
)

func TestRedditSource_Match(t *testing.T) {
	s := RedditSource{}
	tests := []struct {
		url, feedType string
		want          bool
	}{
		{"https://www.reddit.com/r/golang/.rss", "rss", true},
		{"https://www.reddit.com/r/golang/.rss", "huggingface", false},
		{"https://example.com/feed", "rss", false},
	}
	for _, tt := range tests {
		if got := s.Match(tt.url, tt.feedType); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.url, tt.feedType, got, tt.want)
		}
	}
}

func TestRedditSource_ResolveName(t *testing.T) {
	s := RedditSource{}
	tests := []struct {
		url, want string
	}{
		{"https://www.reddit.com/r/golang/.rss", "r/golang"},
		{"https://www.reddit.com/r/AskReddit/.rss", "r/AskReddit"},
		{"https://example.com/feed", ""},
	}
	for _, tt := range tests {
		got := s.ResolveName(context.Background(), tt.url, "")
		if got != tt.want {
			t.Errorf("ResolveName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestRedditSource_NormalizeURL(t *testing.T) {
	url := "https://www.reddit.com/r/golang/.rss"
	got, err := (RedditSource{}).NormalizeURL(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	if got != url {
		t.Errorf("NormalizeURL changed URL: %q -> %q", url, got)
	}
}
