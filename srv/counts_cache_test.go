package srv

import (
	"testing"
	"time"
)

func TestCountsCache_GetSet(t *testing.T) {
	c := NewCountsCache(time.Minute)

	// Miss on empty cache
	_, ok := c.Get(1)
	if ok {
		t.Fatal("expected cache miss")
	}

	// Set and hit
	counts := articleCounts{Unread: 5, Starred: 3, Queue: 1}
	c.Set(1, counts)
	got, ok := c.Get(1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.Unread != 5 || got.Starred != 3 || got.Queue != 1 {
		t.Fatalf("unexpected counts: %+v", got)
	}

	// Different user is a miss
	_, ok = c.Get(2)
	if ok {
		t.Fatal("expected cache miss for different user")
	}
}

func TestCountsCache_Invalidate(t *testing.T) {
	c := NewCountsCache(time.Minute)
	c.Set(1, articleCounts{Unread: 5})
	c.Set(2, articleCounts{Unread: 10})

	c.Invalidate(1)

	_, ok := c.Get(1)
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}

	// User 2 still cached
	_, ok = c.Get(2)
	if !ok {
		t.Fatal("expected cache hit for user 2")
	}
}

func TestCountsCache_InvalidateAll(t *testing.T) {
	c := NewCountsCache(time.Minute)
	c.Set(1, articleCounts{Unread: 5})
	c.Set(2, articleCounts{Unread: 10})

	c.InvalidateAll()

	_, ok1 := c.Get(1)
	_, ok2 := c.Get(2)
	if ok1 || ok2 {
		t.Fatal("expected all entries invalidated")
	}
}

func TestCountsCache_TTLExpiry(t *testing.T) {
	c := NewCountsCache(10 * time.Millisecond)
	c.Set(1, articleCounts{Unread: 5})

	// Should hit immediately
	_, ok := c.Get(1)
	if !ok {
		t.Fatal("expected cache hit")
	}

	// Wait for TTL
	time.Sleep(15 * time.Millisecond)
	_, ok = c.Get(1)
	if ok {
		t.Fatal("expected cache miss after TTL")
	}
}
