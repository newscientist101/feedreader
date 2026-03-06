package srv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/newscientist101/feedreader/config"
)

func TestSetupProvider_AlwaysNil(t *testing.T) {
	t.Parallel()
	p := SetupProvider{}
	r := httptest.NewRequest("GET", "/", http.NoBody)

	id, err := p.Authenticate(r)
	if err != nil {
		t.Fatal(err)
	}
	if id != nil {
		t.Errorf("expected nil identity, got %+v", id)
	}
}

func TestSetupHandler_ServesPage(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "FeedReader Setup") {
		t.Error("page should contain 'FeedReader Setup'")
	}
	if strings.Contains(body, "Warning") {
		t.Error("local request should not show warning")
	}
}

func TestSetupHandler_NonLocalWarning(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.RemoteAddr = "203.0.113.1:12345"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Warning") {
		t.Error("non-local request should show warning")
	}
	if !strings.Contains(body, "203.0.113.1:12345") {
		t.Error("warning should include remote address")
	}
}

func TestSetupHandler_AnyPathServesPage(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	for _, path := range []string{"/", "/feeds", "/api/counts", "/setup"} {
		r := httptest.NewRequest("GET", path, http.NoBody)
		r.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: expected 200, got %d", path, w.Code)
		}
	}
}

func TestSetupHandler_SaveProxy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	var saved atomic.Bool

	h := &SetupHandler{
		ConfigPath:    configPath,
		OnConfigSaved: func() { saved.Store(true) },
	}

	body := `{"provider":"proxy","listen":":9000","db":"my.db"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["ok"] != true {
		t.Errorf("expected ok=true, got %v", resp["ok"])
	}

	// Verify config file was written.
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if cfg.Auth.Provider != "proxy" {
		t.Errorf("provider = %q", cfg.Auth.Provider)
	}
	if cfg.Listen != ":9000" {
		t.Errorf("listen = %q", cfg.Listen)
	}
	if cfg.DB != "my.db" {
		t.Errorf("db = %q", cfg.DB)
	}
	// No proxy section since default headers were used.
	if cfg.Auth.Proxy != nil {
		t.Error("proxy config should be nil for default headers")
	}
}

func TestSetupHandler_SaveProxyCustomHeaders(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	h := &SetupHandler{ConfigPath: configPath}

	body := `{"provider":"proxy","user_id_header":"X-Auth-User","email_header":"X-Auth-Email"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if cfg.Auth.Proxy == nil {
		t.Fatal("proxy config should not be nil")
	}
	if cfg.Auth.Proxy.UserIDHeader != "X-Auth-User" {
		t.Errorf("user_id_header = %q", cfg.Auth.Proxy.UserIDHeader)
	}
	if cfg.Auth.Proxy.EmailHeader != "X-Auth-Email" {
		t.Errorf("email_header = %q", cfg.Auth.Proxy.EmailHeader)
	}
}

func TestSetupHandler_SaveCloudflare(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	h := &SetupHandler{ConfigPath: configPath}

	body := `{"provider":"cloudflare","team_domain":"myteam","audience":"aud-tag"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if cfg.Auth.Provider != "cloudflare" {
		t.Errorf("provider = %q", cfg.Auth.Provider)
	}
	if cfg.Auth.Cloudflare == nil {
		t.Fatal("cloudflare config should not be nil")
	}
	if cfg.Auth.Cloudflare.TeamDomain != "myteam" {
		t.Errorf("team_domain = %q", cfg.Auth.Cloudflare.TeamDomain)
	}
	if cfg.Auth.Cloudflare.Audience != "aud-tag" {
		t.Errorf("audience = %q", cfg.Auth.Cloudflare.Audience)
	}
}

func TestSetupHandler_SaveTailscale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	h := &SetupHandler{ConfigPath: configPath}

	body := `{"provider":"tailscale"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	if cfg.Auth.Provider != "tailscale" {
		t.Errorf("provider = %q", cfg.Auth.Provider)
	}
	if cfg.Listen != config.DefaultListen {
		t.Errorf("listen = %q, want default", cfg.Listen)
	}
	if cfg.DB != config.DefaultDB {
		t.Errorf("db = %q, want default", cfg.DB)
	}
}

func TestSetupHandler_SaveNoProvider(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	body := `{"listen":":8000"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetupHandler_SaveInvalidProvider(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	body := `{"provider":"bogus"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetupHandler_SaveInvalidJSON(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader("not json"))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSetupHandler_SaveCreatesSubdir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "sub", "dir", "config.toml")

	h := &SetupHandler{ConfigPath: configPath}

	body := `{"provider":"authelia"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file should exist: %v", err)
	}
}

func TestSetupHandler_CallbackCalled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	callbackCh := make(chan struct{}, 1)

	h := &SetupHandler{
		ConfigPath:    configPath,
		OnConfigSaved: func() { callbackCh <- struct{}{} },
	}

	body := `{"provider":"tailscale"}`
	r := httptest.NewRequest("POST", setupAPIPath, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Callback runs in a goroutine, so wait briefly.
	select {
	case <-callbackCh:
		// good
	case <-make(chan struct{}):
		// Use a timeout via the test's own deadline
	}
}

func TestSetupHandler_NoCacheHeader(t *testing.T) {
	t.Parallel()
	h := &SetupHandler{ConfigPath: filepath.Join(t.TempDir(), "config.toml")}

	r := httptest.NewRequest("GET", "/", http.NoBody)
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

// --- isLocalRequest tests ---

func TestIsLocalRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		addr  string
		local bool
	}{
		{"127.0.0.1:1234", true},
		{"::1:1234", false},  // malformed (net.SplitHostPort will try)
		{"[::1]:1234", true}, // IPv6 loopback
		{"192.168.1.1:80", false},
		{"10.0.0.1:80", false},
		{"203.0.113.1:443", false},
		{"not-an-ip:80", false},
	}

	for _, tc := range tests {
		r := httptest.NewRequest("GET", "/", http.NoBody)
		r.RemoteAddr = tc.addr
		got := isLocalRequest(r)
		if got != tc.local {
			t.Errorf("isLocalRequest(%q) = %v, want %v", tc.addr, got, tc.local)
		}
	}
}
