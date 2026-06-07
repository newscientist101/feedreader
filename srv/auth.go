package srv

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// Context key for user
type contextKey string

const userContextKey contextKey = "user"

// User represents an authenticated user
type User struct {
	ID         int64
	ExternalID string
	Email      string
}

// GetUser extracts the user from context
func GetUser(ctx context.Context) *User {
	if user, ok := ctx.Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}

// Identity holds the external identity extracted from an HTTP request
// by an AuthProvider. The app uses ExternalID to look up or create a
// database user.
type Identity struct {
	ExternalID string
	Email      string
}

// AuthProvider extracts an identity from an incoming HTTP request.
// Implementations read provider-specific headers, cookies, or tokens.
// A nil *Identity with nil error means "not authenticated".
type AuthProvider interface {
	Authenticate(r *http.Request) (*Identity, error)
}

// DevProvider is the development-mode auth provider. It always returns
// a fixed identity so the app can run without a reverse proxy.
type DevProvider struct{}

// Authenticate always returns the dev user identity.
func (DevProvider) Authenticate(_ *http.Request) (*Identity, error) {
	slog.Debug("using development user")
	return &Identity{ExternalID: "dev-user", Email: "dev@localhost"}, nil
}

// isDevelopment checks if running in development mode.
func isDevelopment() bool {
	return os.Getenv("DEV") != ""
}

type cachedUser struct {
	user User
	// lastSeenUnixNano is the last time we wrote last_seen_at to the DB,
	// stored as Unix nanoseconds and accessed atomically. The cache entry is
	// shared across concurrent requests for the same user and read under only
	// an RLock, so this field must be updated without a write lock — hence the
	// atomic, which also avoids a duplicate DB write when two requests race.
	lastSeenUnixNano atomic.Int64
}

// maxAuthCacheEntries bounds the in-memory auth cache. Identity is keyed by
// the external ID supplied by the upstream auth provider; a misbehaving or
// hostile proxy could otherwise present unboundedly many distinct IDs and
// grow the map without limit. Real deployments have at most a handful of
// users, so this cap is never reached in practice — it only prevents
// pathological growth.
const maxAuthCacheEntries = 4096

// userCache is a concurrency-safe, size-bounded cache of authenticated users
// keyed by external ID. When full, inserting a new key evicts one arbitrary
// existing entry (Go map iteration order); evicted users simply take the slow
// DB path on their next request, so eviction is always safe.
type userCache struct {
	mu      sync.RWMutex
	entries map[string]*cachedUser
	max     int
}

func newUserCache(maxSize int) *userCache {
	return &userCache{entries: make(map[string]*cachedUser), max: maxSize}
}

func (c *userCache) get(key string) *cachedUser {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[key]
}

func (c *userCache) set(key string, u *cachedUser) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Evict an arbitrary entry if at capacity and inserting a new key.
	if _, exists := c.entries[key]; !exists && len(c.entries) >= c.max {
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}
	c.entries[key] = u
}

func (c *userCache) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// AuthMiddleware extracts user identity via the configured AuthProvider
// and ensures authentication for non-static routes.
func (s *Server) AuthMiddleware(next http.Handler) http.Handler {
	cache := newUserCache(maxAuthCacheEntries)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow static files, service worker, and newsletter webhook without auth.
		// The webhook has its own Bearer-token authentication.
		if (len(r.URL.Path) >= 7 && r.URL.Path[:7] == "/static") || r.URL.Path == "/sw.js" || r.URL.Path == newsletterIngestPath {
			next.ServeHTTP(w, r)
			return
		}

		identity, err := s.AuthProvider.Authenticate(r)
		if err != nil {
			slog.Error("auth: provider error", "error", err)
			http.Error(w, "Authentication error", http.StatusInternalServerError)
			return
		}

		// Fall back to dev provider if primary returned no identity and DEV=1
		if identity == nil && isDevelopment() {
			identity, _ = DevProvider{}.Authenticate(r)
		}

		if identity == nil {
			if len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			// Return a plain 401 page — no provider-specific redirect.
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(unauthenticatedPage))
			return
		}

		// Fast path: serve from in-memory cache (no DB hit)
		cached := cache.get(identity.ExternalID)
		if cached != nil {
			ctx := context.WithValue(r.Context(), userContextKey, &cached.user)
			// Update last_seen_at in DB at most once per minute. CompareAndSwap
			// ensures exactly one racing request wins and performs the write.
			prev := cached.lastSeenUnixNano.Load()
			now := time.Now()
			if now.Sub(time.Unix(0, prev)) > time.Minute &&
				cached.lastSeenUnixNano.CompareAndSwap(prev, now.UnixNano()) {
				go func() {
					q := dbgen.New(s.DB)
					_ = q.UpdateUserLastSeen(context.Background(), dbgen.UpdateUserLastSeenParams{
						Email: identity.Email,
						ID:    cached.user.ID,
					})
				}()
			}
			if r.Method == "POST" {
				slog.Info("POST request", "path", r.URL.Path, "remote", r.RemoteAddr, "request_id", r.Header.Get("X-Request-Id"), "content_type", r.Header.Get("Content-Type"))
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Slow path: get or create user in DB
		ctx := r.Context()
		q := dbgen.New(s.DB)

		dbUser, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
			ExternalID: identity.ExternalID,
			Email:      identity.Email,
		})
		if err != nil {
			slog.Error("auth: GetOrCreateUser failed", "error", err, "external_id", identity.ExternalID)
			http.Error(w, "Failed to authenticate user", http.StatusInternalServerError)
			return
		}

		user := &User{
			ID:         dbUser.ID,
			ExternalID: dbUser.ExternalID,
			Email:      dbUser.Email,
		}

		// Cache for subsequent requests
		entry := &cachedUser{user: *user}
		entry.lastSeenUnixNano.Store(time.Now().UnixNano())
		cache.set(identity.ExternalID, entry)

		// Add user to context
		ctx = context.WithValue(ctx, userContextKey, user)
		if r.Method == "POST" {
			slog.Info("POST request", "path", r.URL.Path, "remote", r.RemoteAddr, "request_id", r.Header.Get("X-Request-Id"), "content_type", r.Header.Get("Content-Type"))
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// unauthenticatedPage is a minimal HTML page shown when no identity is found.
// This is a portable 401 response shown when no auth provider identifies the user.
const unauthenticatedPage = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Not Authenticated</title>
<style>body{font-family:system-ui,sans-serif;display:flex;justify-content:center;align-items:center;min-height:100vh;margin:0;background:#f5f5f5;color:#333}div{text-align:center}h1{font-size:1.5rem;margin-bottom:.5rem}p{color:#666}</style>
</head>
<body><div><h1>Not Authenticated</h1><p>Please sign in through your reverse proxy to access this application.</p></div></body>
</html>`
