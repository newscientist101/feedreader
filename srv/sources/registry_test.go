package sources

import (
	"context"
	"testing"
)

func TestRegistry_Lookup(t *testing.T) {
	r := DefaultRegistry()

	// Steam URL should match SteamSource.
	if s := r.Lookup("https://store.steampowered.com/feeds/news/app/440", "rss"); s == nil {
		t.Error("expected SteamSource for Steam feed URL")
	} else if _, ok := s.(SteamSource); !ok {
		t.Errorf("got %T, want SteamSource", s)
	}

	// Reddit URL should match RedditSource.
	if s := r.Lookup("https://www.reddit.com/r/golang/.rss", "rss"); s == nil {
		t.Error("expected RedditSource for Reddit URL")
	} else if _, ok := s.(RedditSource); !ok {
		t.Errorf("got %T, want RedditSource", s)
	}

	// HuggingFace.
	if s := r.Lookup("https://huggingface.co/whatever", "huggingface"); s == nil {
		t.Error("expected HuggingFaceSource")
	} else if _, ok := s.(HuggingFaceSource); !ok {
		t.Errorf("got %T, want HuggingFaceSource", s)
	}

	// Unknown URL returns nil.
	if s := r.Lookup("https://example.com/feed", "rss"); s != nil {
		t.Errorf("expected nil for unknown URL, got %T", s)
	}
}

func TestRegistry_Resolve_NoMatch(t *testing.T) {
	r := DefaultRegistry()
	url, name, ft := r.Resolve(context.Background(), "https://example.com/feed", "", "rss", "")
	if url != "https://example.com/feed" {
		t.Errorf("URL changed: %q", url)
	}
	if name != "" {
		t.Errorf("name = %q, want empty", name)
	}
	if ft != "rss" {
		t.Errorf("feedType = %q, want rss", ft)
	}
}

func TestRegistry_Resolve_PreservesUserName(t *testing.T) {
	r := DefaultRegistry()
	_, name, _ := r.Resolve(context.Background(),
		"https://www.reddit.com/r/golang/.rss", "My Custom Name", "rss", "")
	if name != "My Custom Name" {
		t.Errorf("name = %q, want %q", name, "My Custom Name")
	}
}
