package srv

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter provides per-user rate limiting for API write endpoints.
type rateLimiter struct {
	mu       sync.Mutex
	writers  map[int64]*rate.Limiter // general write endpoints
	lastSeen map[int64]time.Time     // for cleanup
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		writers:  make(map[int64]*rate.Limiter),
		lastSeen: make(map[int64]time.Time),
	}
	go rl.cleanup()
	return rl
}

// cleanup removes stale limiters every 10 minutes.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for uid, last := range rl.lastSeen {
			if now.Sub(last) > 30*time.Minute {
				delete(rl.writers, uid)
				delete(rl.lastSeen, uid)
			}
		}
		rl.mu.Unlock()
	}
}

// getWriteLimiter returns the per-user limiter for general write endpoints.
// 5 requests/sec with burst of 20.
func (rl *rateLimiter) getWriteLimiter(userID int64) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.lastSeen[userID] = time.Now()
	if lim, ok := rl.writers[userID]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(5), 20)
	rl.writers[userID] = lim
	return lim
}

// rateLimitMiddleware applies per-user rate limiting to write API endpoints.
// It must be placed after auth middleware so that GetUser() is available.
func rateLimitMiddleware(rl *rateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only rate-limit state-changing API requests
			if !strings.HasPrefix(r.URL.Path, "/api/") || !isStateChanging(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			user := GetUser(r.Context())
			if user == nil {
				// No user context — let downstream auth handle it
				next.ServeHTTP(w, r)
				return
			}

			// Check general write limiter
			if !rl.getWriteLimiter(user.ID).Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"Rate limit exceeded. Please wait before retrying."}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
