package srv

import (
	"sync"
	"time"
)

// CountsCache is a per-user write-through cache for article counts.
// Counts are cached after computation and invalidated when articles
// change state (read/unread/starred/queued) or new articles arrive.
type CountsCache struct {
	mu      sync.RWMutex
	entries map[int64]*countsCacheEntry
	ttl     time.Duration
}

type countsCacheEntry struct {
	counts   articleCounts
	cachedAt time.Time
}

// NewCountsCache creates a cache with the given TTL.
// Even without explicit invalidation, entries expire after the TTL.
func NewCountsCache(ttl time.Duration) *CountsCache {
	return &CountsCache{
		entries: make(map[int64]*countsCacheEntry),
		ttl:     ttl,
	}
}

// Get returns cached counts for a user, or ok=false if not cached/expired.
func (c *CountsCache) Get(userID int64) (articleCounts, bool) {
	if c == nil {
		return articleCounts{}, false
	}
	c.mu.RLock()
	e, ok := c.entries[userID]
	c.mu.RUnlock()
	if !ok || time.Since(e.cachedAt) > c.ttl {
		return articleCounts{}, false
	}
	return e.counts, true
}

// Set stores counts for a user.
func (c *CountsCache) Set(userID int64, counts articleCounts) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[userID] = &countsCacheEntry{
		counts:   counts,
		cachedAt: time.Now(),
	}
	c.mu.Unlock()
}

// Invalidate removes cached counts for a user.
func (c *CountsCache) Invalidate(userID int64) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, userID)
	c.mu.Unlock()
}

// InvalidateAll removes all cached counts (e.g., after a feed fetch
// that may affect multiple users).
func (c *CountsCache) InvalidateAll() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries = make(map[int64]*countsCacheEntry)
	c.mu.Unlock()
}
