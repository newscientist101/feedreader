package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/newscientist101/feedreader/config"
)

// simulateInit runs initCmd with canned stdin input and returns
// the stdout output and the generated config.
func simulateInit(t *testing.T, input string) (string, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	stdin := strings.NewReader(input)
	var stdout bytes.Buffer

	err := initCmd([]string{"--config", configPath}, stdin, &stdout)
	if err != nil {
		t.Fatalf("initCmd failed: %v\nOutput: %s", err, stdout.String())
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("failed to load generated config: %v", err)
	}

	return stdout.String(), cfg
}

func TestInitDefaultsTailscale(t *testing.T) {
	// Accept default listen, default DB, choose tailscale (2), no newsletter
	input := "\n\n2\nn\n"
	output, cfg := simulateInit(t, input)

	if cfg.Listen != ":8000" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, ":8000")
	}
	if cfg.DB != "db.sqlite3" {
		t.Errorf("DB = %q, want %q", cfg.DB, "db.sqlite3")
	}
	if cfg.Auth.Provider != "tailscale" {
		t.Errorf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "tailscale")
	}
	if !strings.Contains(output, "Config written to") {
		t.Errorf("output missing success message: %s", output)
	}
}

func TestInitProxyDefaultHeaders(t *testing.T) {
	// Accept defaults for listen/DB, choose proxy (1), accept default headers, no newsletter
	input := "\n\n1\n\n\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "proxy" {
		t.Errorf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "proxy")
	}
	// Default headers should not create a proxy config section
	if cfg.Auth.Proxy != nil {
		t.Error("expected nil Proxy config for default headers")
	}
}

func TestInitProxyCustomHeaders(t *testing.T) {
	// Choose proxy with custom headers
	input := "\n\n1\nX-My-User\nX-My-Email\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "proxy" {
		t.Fatalf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "proxy")
	}
	if cfg.Auth.Proxy == nil {
		t.Fatal("expected non-nil Proxy config")
	}
	if cfg.Auth.Proxy.UserIDHeader != "X-My-User" {
		t.Errorf("UserIDHeader = %q, want %q", cfg.Auth.Proxy.UserIDHeader, "X-My-User")
	}
	if cfg.Auth.Proxy.EmailHeader != "X-My-Email" {
		t.Errorf("EmailHeader = %q, want %q", cfg.Auth.Proxy.EmailHeader, "X-My-Email")
	}
}

func TestInitCloudflare(t *testing.T) {
	// Choose cloudflare with team domain and audience
	input := "\n\n3\nmyteam\nabc123\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "cloudflare" {
		t.Fatalf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "cloudflare")
	}
	if cfg.Auth.Cloudflare == nil {
		t.Fatal("expected non-nil Cloudflare config")
	}
	if cfg.Auth.Cloudflare.TeamDomain != "myteam" {
		t.Errorf("TeamDomain = %q, want %q", cfg.Auth.Cloudflare.TeamDomain, "myteam")
	}
	if cfg.Auth.Cloudflare.Audience != "abc123" {
		t.Errorf("Audience = %q, want %q", cfg.Auth.Cloudflare.Audience, "abc123")
	}
}

func TestInitAuthelia(t *testing.T) {
	// Choose authelia (4), no newsletter
	input := "\n\n4\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "authelia" {
		t.Errorf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "authelia")
	}
}

func TestInitOAuth2Proxy(t *testing.T) {
	// Choose oauth2_proxy (5), no newsletter
	input := "\n\n5\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "oauth2_proxy" {
		t.Errorf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "oauth2_proxy")
	}
}

func TestInitWithNewsletter(t *testing.T) {
	// Tailscale + newsletter with webhook and SMTP
	input := "\n\n2\ny\nmail.example.com\ny\nsecret123\ny\n2525\n"
	_, cfg := simulateInit(t, input)

	if cfg.EmailDomain != "mail.example.com" {
		t.Errorf("EmailDomain = %q, want %q", cfg.EmailDomain, "mail.example.com")
	}
	if cfg.Newsletter.WebhookSecret != "secret123" {
		t.Errorf("WebhookSecret = %q, want %q", cfg.Newsletter.WebhookSecret, "secret123")
	}
	if cfg.Newsletter.SMTPPort != 2525 {
		t.Errorf("SMTPPort = %d, want %d", cfg.Newsletter.SMTPPort, 2525)
	}
}

func TestInitCustomListenAndDB(t *testing.T) {
	// Custom listen address and DB path
	input := ":9000\n/data/reader.db\n2\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Listen != ":9000" {
		t.Errorf("Listen = %q, want %q", cfg.Listen, ":9000")
	}
	if cfg.DB != "/data/reader.db" {
		t.Errorf("DB = %q, want %q", cfg.DB, "/data/reader.db")
	}
}

func TestInitFileExists(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	// Create existing file
	if err := os.WriteFile(configPath, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdin := strings.NewReader("\n\n1\nn\n")
	var stdout bytes.Buffer

	err := initCmd([]string{"--config", configPath}, stdin, &stdout)
	if err == nil {
		t.Fatal("expected error when config file exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want 'already exists' message", err)
	}
}

func TestInitProviderByName(t *testing.T) {
	// Use provider name instead of number
	input := "\n\ntailscale\nn\n"
	_, cfg := simulateInit(t, input)

	if cfg.Auth.Provider != "tailscale" {
		t.Errorf("Auth.Provider = %q, want %q", cfg.Auth.Provider, "tailscale")
	}
}

func TestResolveProviderChoice(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "proxy"},
		{"2", "tailscale"},
		{"3", "cloudflare"},
		{"4", "authelia"},
		{"5", "oauth2_proxy"},
		{"6", "exedev"},
		{"proxy", "proxy"},
		{"TAILSCALE", "tailscale"},
		{"Cloudflare", "cloudflare"},
		{"unknown", "proxy"}, // defaults to proxy
	}

	for _, tt := range tests {
		got := resolveProviderChoice(tt.input)
		if got != tt.want {
			t.Errorf("resolveProviderChoice(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
