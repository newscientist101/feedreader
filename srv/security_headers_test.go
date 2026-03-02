package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := securityHeaders(inner)

	req := httptest.NewRequest("GET", "/", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	tests := []struct {
		header, want string
	}{
		{"X-Content-Type-Options", "nosniff"},
		{"X-Frame-Options", "DENY"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Content-Security-Policy", ""},
	}

	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got == "" {
			t.Errorf("missing header %s", tt.header)
			continue
		}
		if tt.want != "" && got != tt.want {
			t.Errorf("%s = %q, want %q", tt.header, got, tt.want)
		}
	}

	// Verify CSP contains key directives.
	csp := rec.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src", "script-src", "style-src", "object-src 'none'"} {
		if !containsSubstring(csp, directive) {
			t.Errorf("CSP missing directive %q: %s", directive, csp)
		}
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
