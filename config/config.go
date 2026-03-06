// Package config handles TOML configuration loading and generation
// for the feedreader application.
package config

import (
	"fmt"
	"io"
	"os"

	toml "github.com/pelletier/go-toml/v2"
)

// Config is the top-level configuration structure.
type Config struct {
	// Listen is the address to listen on (e.g. ":8000").
	Listen string `toml:"listen,omitempty"`

	// DB is the path to the SQLite database file.
	DB string `toml:"db,omitempty"`

	// EmailDomain is the email domain suffix for newsletter ingestion.
	EmailDomain string `toml:"email_domain,omitempty"`

	// Auth contains authentication provider configuration.
	Auth AuthConfig `toml:"auth"`

	// Newsletter contains newsletter ingestion configuration.
	Newsletter NewsletterConfig `toml:"newsletter,omitempty"`
}

// AuthConfig holds auth provider selection and provider-specific settings.
type AuthConfig struct {
	// Provider is the auth provider name: "exedev", "proxy", "tailscale",
	// "cloudflare", "authelia", "oauth2_proxy".
	Provider string `toml:"provider"`

	// Proxy holds settings for the generic proxy header provider.
	Proxy *ProxyAuthConfig `toml:"proxy,omitempty"`

	// Cloudflare holds settings for Cloudflare Access.
	Cloudflare *CloudflareAuthConfig `toml:"cloudflare,omitempty"`
}

// ProxyAuthConfig holds settings for the generic reverse-proxy header auth.
type ProxyAuthConfig struct {
	// UserIDHeader is the header containing the unique user identifier.
	// Defaults to "Remote-User".
	UserIDHeader string `toml:"user_id_header,omitempty"`

	// EmailHeader is the header containing the user's email.
	// Defaults to "Remote-Email".
	EmailHeader string `toml:"email_header,omitempty"`
}

// CloudflareAuthConfig holds settings for Cloudflare Access.
type CloudflareAuthConfig struct {
	// TeamDomain is the Cloudflare Access team domain
	// (e.g. "myteam" for myteam.cloudflareaccess.com).
	TeamDomain string `toml:"team_domain,omitempty"`

	// Audience is the Application Audience (AUD) tag for JWT validation.
	Audience string `toml:"audience,omitempty"`
}

// NewsletterConfig holds newsletter ingestion settings.
type NewsletterConfig struct {
	// WebhookSecret is the shared secret for HTTP webhook ingestion.
	WebhookSecret string `toml:"webhook_secret,omitempty"`

	// SMTPPort is the port for the built-in SMTP server (0 = disabled).
	SMTPPort int `toml:"smtp_port,omitempty"`
}

// Defaults for configuration values.
const (
	DefaultListen = ":8000"
	DefaultDB     = "db.sqlite3"
)

// AuthProviderNames lists all supported auth provider identifiers.
var AuthProviderNames = []string{
	"proxy",
	"tailscale",
	"cloudflare",
	"authelia",
	"oauth2_proxy",
	"exedev",
}

// WriteTo writes the config as TOML to w.
func (c *Config) WriteTo(w io.Writer) (int64, error) {
	b, err := toml.Marshal(c)
	if err != nil {
		return 0, fmt.Errorf("marshal config: %w", err)
	}
	n, err := w.Write(b)
	return int64(n), err
}

// WriteFile writes the config as TOML to the given path.
// It creates the file with 0644 permissions.
func (c *Config) WriteFile(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create config file: %w", err)
	}

	if _, werr := c.WriteTo(f); werr != nil {
		_ = f.Close() // best-effort close on write error
		return werr
	}
	return f.Close()
}

// Load reads a TOML config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	return &cfg, nil
}
