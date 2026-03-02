package srv

import (
	"net/http"
	"strings"
)

// csrfMiddleware rejects state-changing requests (POST, PUT, DELETE, PATCH)
// to /api/ paths that lack a custom X-Requested-With header. Browsers will
// not include custom headers in cross-origin requests without CORS preflight
// approval, so this effectively prevents CSRF attacks.
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && isStateChanging(r.Method) {
			if r.Header.Get("X-Requested-With") == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"error":"CSRF validation failed"}`))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func isStateChanging(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		return true
	}
	return false
}
