package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/newscientist101/feedreader/config"
	"github.com/newscientist101/feedreader/srv"
)

var flagListenAddr = flag.String("listen", ":8000", "address to listen on")
var flagDBPath = flag.String("db", "db.sqlite3", "path to SQLite database file")
var flagEmailDomain = flag.String("email-domain", "", "email domain suffix (default: auto-detect from hostname)")
var flagConfig = flag.String("config", "", "path to TOML config file (env: CONFIG_FILE)")

// configFileEnvVar is the environment variable for overriding the config file path.
const configFileEnvVar = "CONFIG_FILE"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	// Check for subcommands before parsing flags.
	// "init" must be the first argument.
	if len(os.Args) > 1 && os.Args[1] == "init" {
		return initCmd(os.Args[2:], os.Stdin, os.Stdout)
	}

	flag.Parse()

	// Resolve config file path: flag > env > default (config.toml if it exists).
	cfg, err := loadConfig(resolveConfigPath(*flagConfig))
	if err != nil {
		return err
	}

	// Flags override config values when explicitly set.
	listenAddr, dbPath, emailDomain := mergeConfig(cfg)

	server, err := initServer(dbPath, emailDomain)
	if err != nil {
		return err
	}

	// Apply auth provider and newsletter config.
	if cfg != nil {
		provider, providerErr := buildAuthProvider(cfg)
		if providerErr != nil {
			return fmt.Errorf("configure auth: %w", providerErr)
		}
		if provider != nil {
			server.AuthProvider = provider
		}
		server.WebhookSecret = cfg.Newsletter.WebhookSecret
	}

	return server.Serve(listenAddr)
}

// resolveConfigPath returns the config file path from flag, env, or default.
// Returns "" if no config file is specified or found.
func resolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envPath := os.Getenv(configFileEnvVar); envPath != "" {
		return envPath
	}
	// Check if config.toml exists in the current directory.
	if _, err := os.Stat("config.toml"); err == nil {
		return "config.toml"
	}
	return ""
}

// loadConfig loads a config file if the path is non-empty.
// Returns nil config (no error) when no path is given.
func loadConfig(path string) (*config.Config, error) {
	if path == "" {
		return nil, nil
	}
	slog.Info("loading config", "path", path)
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}
	return cfg, nil
}

// mergeConfig returns effective values, preferring explicit flags over config.
func mergeConfig(cfg *config.Config) (listenAddr, dbPath, emailDomain string) {
	listenAddr = *flagListenAddr
	dbPath = *flagDBPath
	emailDomain = *flagEmailDomain

	if cfg == nil {
		return listenAddr, dbPath, emailDomain
	}

	// Config values fill in only when the flag is at its default.
	if !flagExplicitlySet("listen") && cfg.Listen != "" {
		listenAddr = cfg.Listen
	}
	if !flagExplicitlySet("db") && cfg.DB != "" {
		dbPath = cfg.DB
	}
	if !flagExplicitlySet("email-domain") && cfg.EmailDomain != "" {
		emailDomain = cfg.EmailDomain
	}

	return listenAddr, dbPath, emailDomain
}

// flagExplicitlySet reports whether a flag was set on the command line.
func flagExplicitlySet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// buildAuthProvider creates an AuthProvider from config.
// Returns nil provider (no error) when the config has no auth section.
func buildAuthProvider(cfg *config.Config) (srv.AuthProvider, error) {
	switch cfg.Auth.Provider {
	case "":
		return nil, nil
	case "exedev":
		return srv.ExeDevProvider{}, nil
	case "proxy":
		p := &srv.ProxyHeaderProvider{}
		if cfg.Auth.Proxy != nil {
			p.UserIDHeader = cfg.Auth.Proxy.UserIDHeader
			p.EmailHeader = cfg.Auth.Proxy.EmailHeader
		}
		return p, nil
	case "tailscale":
		return srv.TailscaleProvider{}, nil
	case "cloudflare":
		p := &srv.CloudflareAccessProvider{}
		if cfg.Auth.Cloudflare != nil {
			p.TeamDomain = cfg.Auth.Cloudflare.TeamDomain
			p.Audience = cfg.Auth.Cloudflare.Audience
		}
		if p.TeamDomain == "" {
			return nil, fmt.Errorf("cloudflare auth requires team_domain")
		}
		return p, nil
	case "authelia":
		return srv.AutheliaProvider{}, nil
	case "oauth2_proxy":
		return srv.OAuth2ProxyProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown auth provider: %q", cfg.Auth.Provider)
	}
}

// initServer creates and configures the server. Separated from run()
// so tests can exercise initialization without blocking on Serve().
func initServer(dbPath, emailDomain string) (*srv.Server, error) {
	if emailDomain == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "localhost"
		}
		emailDomain = hostname
	}
	server, err := srv.New(dbPath, emailDomain)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	return server, nil
}
