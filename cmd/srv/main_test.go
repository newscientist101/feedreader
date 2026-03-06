package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/newscientist101/feedreader/config"
	"github.com/newscientist101/feedreader/srv"
)

func TestFlagDefault(t *testing.T) {
	want := ":8000"
	got := *flagListenAddr
	if got != want {
		t.Errorf("default listen address = %q, want %q", got, want)
	}

	f := flag.Lookup("listen")
	if f == nil {
		t.Fatal(`flag "listen" not registered`)
	}
	if f.DefValue != want {
		t.Errorf("flag DefValue = %q, want %q", f.DefValue, want)
	}
}

func TestFlagConfig_Registered(t *testing.T) {
	f := flag.Lookup("config")
	if f == nil {
		t.Fatal(`flag "config" not registered`)
	}
	if f.DefValue != "" {
		t.Errorf("flag DefValue = %q, want empty", f.DefValue)
	}
}

func TestInitServer_InvalidDBPath(t *testing.T) {
	badPath := filepath.Join(t.TempDir(), "no", "such", "dir", "db.sqlite3")
	_, err := initServer(badPath, "")
	if err == nil {
		t.Fatal("expected error for invalid DB path, got nil")
	}
}

func TestInitServer_ValidDBPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite3")
	server, err := initServer(dbPath, "")
	if err != nil {
		t.Fatalf("initServer: %v", err)
	}
	if server == nil {
		t.Fatal("returned nil server")
	}
	if server.DB != nil {
		server.DB.Close()
	}
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected DB file to be created")
	}
}

func TestInitServer_SetsFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite3")
	server, err := initServer(dbPath, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.DB.Close() })

	if server.DB == nil {
		t.Error("DB is nil")
	}
	if server.Hostname == "" {
		t.Error("Hostname is empty")
	}
	if server.ScraperRunner == nil {
		t.Error("ScraperRunner is nil")
	}
	if server.Fetcher == nil {
		t.Error("Fetcher is nil")
	}
}

func TestInitServer_ExplicitEmailDomain(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite3")
	server, err := initServer(dbPath, "custom.example.com")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.DB.Close() })

	if server.Hostname != "custom.example.com" {
		t.Errorf("Hostname = %q, want %q", server.Hostname, "custom.example.com")
	}
}

// --- resolveConfigPath tests ---

func TestResolveConfigPath_FlagTakesPrecedence(t *testing.T) {
	// Even if CONFIG_FILE is set, the flag value wins.
	t.Setenv(configFileEnvVar, "/env/config.toml")
	got := resolveConfigPath("/flag/config.toml")
	if got != "/flag/config.toml" {
		t.Errorf("got %q, want /flag/config.toml", got)
	}
}

func TestResolveConfigPath_EnvFallback(t *testing.T) {
	t.Setenv(configFileEnvVar, "/env/config.toml")
	got := resolveConfigPath("")
	if got != "/env/config.toml" {
		t.Errorf("got %q, want /env/config.toml", got)
	}
}

func TestResolveConfigPath_DefaultFileExists(t *testing.T) {
	tmp := t.TempDir()
	oldDir, _ := os.Getwd()
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("chdir back: %v", err)
		}
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	// Create config.toml in the temp dir.
	if err := os.WriteFile("config.toml", []byte("listen = \":9000\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(configFileEnvVar, "") // clear env
	got := resolveConfigPath("")
	if got != "config.toml" {
		t.Errorf("got %q, want config.toml", got)
	}
}

func TestResolveConfigPath_NoConfig(t *testing.T) {
	tmp := t.TempDir()
	oldDir, _ := os.Getwd()
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Logf("chdir back: %v", err)
		}
	})
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}

	t.Setenv(configFileEnvVar, "") // clear env
	got := resolveConfigPath("")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

// --- loadConfig tests ---

func TestLoadConfig_EmptyPath(t *testing.T) {
	cfg, err := loadConfig("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for empty path")
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.toml")
	if err := os.WriteFile(path, []byte("listen = \":9000\"\ndb = \"/data/test.db\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Listen != ":9000" {
		t.Errorf("Listen = %q, want :9000", cfg.Listen)
	}
	if cfg.DB != "/data/test.db" {
		t.Errorf("DB = %q, want /data/test.db", cfg.DB)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := loadConfig("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadConfig_InvalidTOML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.toml")
	if err := os.WriteFile(path, []byte("not valid toml [[[\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

// --- mergeConfig tests ---

func TestMergeConfig_NilConfig(t *testing.T) {
	listen, db, email := mergeConfig(nil)
	if listen != *flagListenAddr {
		t.Errorf("listen = %q, want %q", listen, *flagListenAddr)
	}
	if db != *flagDBPath {
		t.Errorf("db = %q, want %q", db, *flagDBPath)
	}
	if email != *flagEmailDomain {
		t.Errorf("email = %q, want %q", email, *flagEmailDomain)
	}
}

func TestMergeConfig_ConfigFillsDefaults(t *testing.T) {
	// When flags are at their defaults, config values should be used.
	cfg := &config.Config{
		Listen:      ":9000",
		DB:          "/data/my.db",
		EmailDomain: "example.com",
	}
	listen, db, email := mergeConfig(cfg)
	if listen != ":9000" {
		t.Errorf("listen = %q, want :9000", listen)
	}
	if db != "/data/my.db" {
		t.Errorf("db = %q, want /data/my.db", db)
	}
	if email != "example.com" {
		t.Errorf("email = %q, want example.com", email)
	}
}

func TestMergeConfig_EmptyConfigKeepsDefaults(t *testing.T) {
	cfg := &config.Config{} // all zero values
	listen, db, email := mergeConfig(cfg)
	// Should keep flag defaults when config values are empty.
	if listen != *flagListenAddr {
		t.Errorf("listen = %q, want %q", listen, *flagListenAddr)
	}
	if db != *flagDBPath {
		t.Errorf("db = %q, want %q", db, *flagDBPath)
	}
	if email != *flagEmailDomain {
		t.Errorf("email = %q, want %q", email, *flagEmailDomain)
	}
}

// --- buildAuthProvider tests ---

func TestBuildAuthProvider_EmptyProvider(t *testing.T) {
	cfg := &config.Config{}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p != nil {
		t.Error("expected nil provider for empty config")
	}
}

func TestBuildAuthProvider_ExeDev(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "exedev"}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(srv.ExeDevProvider); !ok {
		t.Errorf("expected ExeDevProvider, got %T", p)
	}
}

func TestBuildAuthProvider_Proxy_Defaults(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "proxy"}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proxy, ok := p.(*srv.ProxyHeaderProvider)
	if !ok {
		t.Fatalf("expected *ProxyHeaderProvider, got %T", p)
	}
	// Should use defaults when no proxy config is given.
	if proxy.UserIDHeader != "" {
		t.Errorf("UserIDHeader = %q, want empty (defaults internally)", proxy.UserIDHeader)
	}
}

func TestBuildAuthProvider_Proxy_CustomHeaders(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		Provider: "proxy",
		Proxy: &config.ProxyAuthConfig{
			UserIDHeader: "X-User",
			EmailHeader:  "X-Email",
		},
	}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	proxy := p.(*srv.ProxyHeaderProvider)
	if proxy.UserIDHeader != "X-User" {
		t.Errorf("UserIDHeader = %q, want X-User", proxy.UserIDHeader)
	}
	if proxy.EmailHeader != "X-Email" {
		t.Errorf("EmailHeader = %q, want X-Email", proxy.EmailHeader)
	}
}

func TestBuildAuthProvider_Tailscale(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "tailscale"}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(srv.TailscaleProvider); !ok {
		t.Errorf("expected TailscaleProvider, got %T", p)
	}
}

func TestBuildAuthProvider_Cloudflare(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		Provider: "cloudflare",
		Cloudflare: &config.CloudflareAuthConfig{
			TeamDomain: "myteam",
			Audience:   "aud123",
		},
	}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cf, ok := p.(*srv.CloudflareAccessProvider)
	if !ok {
		t.Fatalf("expected *CloudflareAccessProvider, got %T", p)
	}
	if cf.TeamDomain != "myteam" {
		t.Errorf("TeamDomain = %q, want myteam", cf.TeamDomain)
	}
	if cf.Audience != "aud123" {
		t.Errorf("Audience = %q, want aud123", cf.Audience)
	}
}

func TestBuildAuthProvider_Cloudflare_MissingTeamDomain(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "cloudflare"}}
	_, err := buildAuthProvider(cfg)
	if err == nil {
		t.Fatal("expected error for missing team_domain")
	}
}

func TestBuildAuthProvider_Authelia(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "authelia"}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(srv.AutheliaProvider); !ok {
		t.Errorf("expected AutheliaProvider, got %T", p)
	}
}

func TestBuildAuthProvider_OAuth2Proxy(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "oauth2_proxy"}}
	p, err := buildAuthProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := p.(srv.OAuth2ProxyProvider); !ok {
		t.Errorf("expected OAuth2ProxyProvider, got %T", p)
	}
}

func TestBuildAuthProvider_Unknown(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Provider: "bogus"}}
	_, err := buildAuthProvider(cfg)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
