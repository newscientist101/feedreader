package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteToAndLoad(t *testing.T) {
	cfg := &Config{
		Listen: ":9000",
		DB:     "test.db",
		Auth: AuthConfig{
			Provider: "tailscale",
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := cfg.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Listen != ":9000" {
		t.Errorf("Listen = %q, want %q", loaded.Listen, ":9000")
	}
	if loaded.DB != "test.db" {
		t.Errorf("DB = %q, want %q", loaded.DB, "test.db")
	}
	if loaded.Auth.Provider != "tailscale" {
		t.Errorf("Auth.Provider = %q, want %q", loaded.Auth.Provider, "tailscale")
	}
}

func TestWriteToProxyConfig(t *testing.T) {
	cfg := &Config{
		Listen: ":8000",
		DB:     "db.sqlite3",
		Auth: AuthConfig{
			Provider: "proxy",
			Proxy: &ProxyAuthConfig{
				UserIDHeader: "X-Custom-User",
				EmailHeader:  "X-Custom-Email",
			},
		},
	}

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("X-Custom-User")) {
		t.Errorf("output missing proxy header config: %s", got)
	}
}

func TestWriteToCloudflareConfig(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			Provider: "cloudflare",
			Cloudflare: &CloudflareAuthConfig{
				TeamDomain: "myteam",
				Audience:   "abc123",
			},
		},
	}

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	got := buf.String()
	if !bytes.Contains([]byte(got), []byte("myteam")) {
		t.Errorf("output missing cloudflare team_domain: %s", got)
	}
}

func TestWriteToOmitsEmptyOptionals(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{
			Provider: "tailscale",
		},
	}

	var buf bytes.Buffer
	if _, err := cfg.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	got := buf.String()
	// Should not contain proxy or cloudflare sections
	if bytes.Contains([]byte(got), []byte("[auth.proxy]")) {
		t.Errorf("output should not contain proxy section for tailscale: %s", got)
	}
	if bytes.Contains([]byte(got), []byte("[auth.cloudflare]")) {
		t.Errorf("output should not contain cloudflare section for tailscale: %s", got)
	}
}

func TestLoadNonexistent(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("[[[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestNewsletterConfig(t *testing.T) {
	cfg := &Config{
		Auth: AuthConfig{Provider: "tailscale"},
		Newsletter: NewsletterConfig{
			WebhookSecret: "secret123",
			SMTP: SMTPConfig{
				Enabled: true,
				Listen:  ":2525",
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	if err := cfg.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Newsletter.WebhookSecret != "secret123" {
		t.Errorf("WebhookSecret = %q, want %q", loaded.Newsletter.WebhookSecret, "secret123")
	}
	if !loaded.Newsletter.SMTP.Enabled {
		t.Errorf("SMTP.Enabled = false, want true")
	}
	if loaded.Newsletter.SMTP.Listen != ":2525" {
		t.Errorf("SMTP.Listen = %q, want %q", loaded.Newsletter.SMTP.Listen, ":2525")
	}
}
