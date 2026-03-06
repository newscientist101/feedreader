package srv

import (
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/newscientist101/feedreader/config"
)

// SetupProvider is a sentinel auth provider indicating no authentication
// has been configured yet. When this provider is active, the server
// serves the setup UI instead of normal content.
type SetupProvider struct{}

// Authenticate always returns nil — no user is authenticated in setup mode.
func (SetupProvider) Authenticate(_ *http.Request) (*Identity, error) {
	return nil, nil
}

// SetupHandler wraps the normal server handler to intercept all requests
// when the server is in setup mode (AuthProvider is SetupProvider).
// It serves a self-contained setup UI for initial configuration.
type SetupHandler struct {
	// ConfigPath is where the generated config.toml will be written.
	ConfigPath string

	// OnConfigSaved is called after config is successfully written.
	// Typically triggers a server restart or shutdown.
	OnConfigSaved func()
}

const setupAPIPath = "/setup/save"

// ServeHTTP handles all requests in setup mode. Static files pass through;
// everything else gets the setup UI or API.
func (h *SetupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Warn if accessed from a non-local IP.
	if !isLocalRequest(r) {
		slog.Warn("setup UI accessed from non-local address",
			"remote", r.RemoteAddr,
			"path", r.URL.Path)
	}

	switch {
	case r.Method == http.MethodPost && r.URL.Path == setupAPIPath:
		h.handleSave(w, r)
	default:
		h.handlePage(w, r)
	}
}

// setupRequest is the JSON payload for the setup save API.
type setupRequest struct {
	Provider     string `json:"provider"`
	Listen       string `json:"listen"`
	DB           string `json:"db"`
	UserIDHeader string `json:"user_id_header"`
	EmailHeader  string `json:"email_header"`
	TeamDomain   string `json:"team_domain"`
	Audience     string `json:"audience"`
}

func (h *SetupHandler) handlePage(w http.ResponseWriter, r *http.Request) {
	local := isLocalRequest(r)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	page := setupPageHTML
	if !local {
		// Inject a warning banner for non-local access.
		page = strings.Replace(page, "<!--NON_LOCAL_WARNING-->",
			`<div class="warning">⚠️ Warning: You are accessing the setup UI from a non-local address (`+
				html.EscapeString(r.RemoteAddr)+
				`). This page has no authentication. Consider binding to localhost only.</div>`, 1)
	}
	_, _ = w.Write([]byte(page))
}

func (h *SetupHandler) handleSave(w http.ResponseWriter, r *http.Request) {
	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Provider == "" {
		http.Error(w, "Provider is required", http.StatusBadRequest)
		return
	}

	// Validate provider name.
	if !slices.Contains(config.AuthProviderNames, req.Provider) {
		http.Error(w, fmt.Sprintf("Unknown provider: %s", req.Provider), http.StatusBadRequest)
		return
	}

	// Build config.
	cfg := &config.Config{
		Listen: req.Listen,
		DB:     req.DB,
		Auth: config.AuthConfig{
			Provider: req.Provider,
		},
	}
	if cfg.Listen == "" {
		cfg.Listen = config.DefaultListen
	}
	if cfg.DB == "" {
		cfg.DB = config.DefaultDB
	}

	// Provider-specific config.
	switch req.Provider {
	case "proxy":
		if req.UserIDHeader != "" || req.EmailHeader != "" {
			proxy := &config.ProxyAuthConfig{}
			if req.UserIDHeader != "" {
				proxy.UserIDHeader = req.UserIDHeader
			}
			if req.EmailHeader != "" {
				proxy.EmailHeader = req.EmailHeader
			}
			cfg.Auth.Proxy = proxy
		}
	case "cloudflare":
		if req.TeamDomain != "" || req.Audience != "" {
			cfg.Auth.Cloudflare = &config.CloudflareAuthConfig{
				TeamDomain: req.TeamDomain,
				Audience:   req.Audience,
			}
		}
	}

	// Ensure parent directory exists.
	dir := filepath.Dir(h.ConfigPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Error("setup: create config dir", "error", err, "dir", dir)
			http.Error(w, "Failed to create config directory", http.StatusInternalServerError)
			return
		}
	}

	// Write config.
	if err := cfg.WriteFile(h.ConfigPath); err != nil {
		slog.Error("setup: write config", "error", err, "path", h.ConfigPath)
		http.Error(w, "Failed to write config file", http.StatusInternalServerError)
		return
	}

	slog.Info("setup: config saved", "path", h.ConfigPath, "provider", req.Provider)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true,"message":"Configuration saved. The server will restart."}`))

	// Trigger restart/shutdown callback if set.
	if h.OnConfigSaved != nil {
		go h.OnConfigSaved()
	}
}

// isLocalRequest checks if the request comes from a loopback address.
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port.
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
