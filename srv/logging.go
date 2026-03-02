package srv

import (
	"log/slog"
	"net/http"
	"time"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

// Unwrap supports http.ResponseController and middleware that
// need access to the original ResponseWriter.
func (sw *statusWriter) Unwrap() http.ResponseWriter {
	return sw.ResponseWriter
}

// loggingMiddleware logs every HTTP request with method, path, status, duration,
// and user ID. Static asset requests (CSS, JS, images) are skipped to reduce noise.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip static assets — they're noisy and low-value.
		if isStaticPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(sw, r)

		duration := time.Since(start)
		attrs := []slog.Attr{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Duration("duration", duration),
		}

		// Add user ID if available from auth context.
		if user := GetUser(r.Context()); user != nil {
			attrs = append(attrs, slog.Int64("user_id", user.ID))
		}

		// Add query string for API endpoints (useful for debugging).
		if r.URL.RawQuery != "" {
			attrs = append(attrs, slog.String("query", r.URL.RawQuery))
		}

		msg := "request"
		switch {
		case sw.status >= 500:
			slog.LogAttrs(r.Context(), slog.LevelError, msg, attrs...)
		case sw.status >= 400:
			slog.LogAttrs(r.Context(), slog.LevelWarn, msg, attrs...)
		default:
			slog.LogAttrs(r.Context(), slog.LevelInfo, msg, attrs...)
		}
	})
}

// isStaticPath returns true for paths that serve static assets.
func isStaticPath(path string) bool {
	switch {
	case len(path) >= 8 && path[:8] == "/static/":
		return true
	case path == "/sw.js":
		return true
	case path == "/api/favicon":
		return true
	default:
		return false
	}
}
