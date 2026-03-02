package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFMiddleware(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := csrfMiddleware(inner)

	tests := []struct {
		name       string
		method     string
		path       string
		header     string // X-Requested-With value; "" means omit
		wantStatus int
	}{
		{"POST /api no header", http.MethodPost, "/api/feeds", "", 403},
		{"PUT /api no header", http.MethodPut, "/api/feeds/1", "", 403},
		{"DELETE /api no header", http.MethodDelete, "/api/feeds/1", "", 403},
		{"PATCH /api no header", http.MethodPatch, "/api/feeds/1", "", 403},
		{"POST /api with header", http.MethodPost, "/api/feeds", "XMLHttpRequest", 200},
		{"PUT /api with header", http.MethodPut, "/api/feeds/1", "XMLHttpRequest", 200},
		{"DELETE /api with header", http.MethodDelete, "/api/feeds/1", "XMLHttpRequest", 200},
		{"GET /api no header", http.MethodGet, "/api/counts", "", 200},
		{"POST non-api no header", http.MethodPost, "/login", "", 200},
		{"POST /api arbitrary value", http.MethodPost, "/api/feeds", "fetch", 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, http.NoBody)
			if tt.header != "" {
				req.Header.Set("X-Requested-With", tt.header)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
