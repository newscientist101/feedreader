package srv

import (
	"net/http"
	"strings"
)

const (
	// defaultBodyLimit is the maximum request body size for most endpoints (1 MB).
	defaultBodyLimit int64 = 1 << 20
	// uploadBodyLimit is the maximum request body size for file upload endpoints (10 MB).
	uploadBodyLimit int64 = 10 << 20
)

// bodyLimitPaths maps path prefixes to larger body limits for upload endpoints.
var bodyLimitPaths = map[string]int64{
	"/api/opml/import":   uploadBodyLimit,
	newsletterIngestPath: newsletterIngestMaxBody,
}

// bodyLimitMiddleware wraps request bodies with http.MaxBytesReader to
// prevent excessively large requests from consuming memory.
func bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only limit requests that may have a body.
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
			limit := defaultBodyLimit
			for prefix, l := range bodyLimitPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					limit = l
					break
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}
