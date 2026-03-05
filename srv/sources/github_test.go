package sources

import (
	"context"
	"testing"
)

func TestGitHubSource_Match(t *testing.T) {
	s := GitHubSource{}
	tests := []struct {
		url, feedType string
		want          bool
	}{
		{"https://github.com/golang/go/releases.atom", "github", true},
		{"https://github.com/golang/go", "github", true},
		{"https://github.com/golang/go/releases.atom", "rss", false},
		{"https://example.com/feed", "github", false},
	}
	for _, tt := range tests {
		if got := s.Match(tt.url, tt.feedType); got != tt.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tt.url, tt.feedType, got, tt.want)
		}
	}
}

func TestGitHubSource_NormalizeURL(t *testing.T) {
	s := GitHubSource{}
	tests := []struct {
		url, want string
	}{
		// Already a feed URL.
		{"https://github.com/golang/go/releases.atom", "https://github.com/golang/go/releases.atom"},
		// Repo URL.
		{"https://github.com/golang/go", "https://github.com/golang/go/releases.atom"},
		// Repo URL with trailing slash.
		{"https://github.com/golang/go/", "https://github.com/golang/go/releases.atom"},
		// Repo URL with extra path.
		{"https://github.com/golang/go/releases", "https://github.com/golang/go/releases.atom"},
		// .git suffix.
		{"https://github.com/golang/go.git", "https://github.com/golang/go/releases.atom"},
	}
	for _, tt := range tests {
		got, err := s.NormalizeURL(context.Background(), tt.url)
		if err != nil {
			t.Errorf("NormalizeURL(%q) error: %v", tt.url, err)
			continue
		}
		if got != tt.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestGitHubSource_ResolveName(t *testing.T) {
	s := GitHubSource{}
	tests := []struct {
		url, want string
	}{
		{"https://github.com/golang/go/releases.atom", "golang/go releases"},
		{"https://github.com/rust-lang/rust/releases.atom", "rust-lang/rust releases"},
		{"https://example.com/feed", ""},
	}
	for _, tt := range tests {
		got := s.ResolveName(context.Background(), tt.url, "")
		if got != tt.want {
			t.Errorf("ResolveName(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestGitHubSource_FeedType(t *testing.T) {
	if ft := (GitHubSource{}).FeedType(); ft != "rss" {
		t.Errorf("FeedType() = %q, want %q", ft, "rss")
	}
}
