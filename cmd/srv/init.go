package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/newscientist101/feedreader/config"
)

// promptWriter wraps an io.Writer and silently discards write errors.
// Interactive CLI output to stdout/stderr is best-effort; failing to
// display a prompt line is not recoverable.
type promptWriter struct {
	w io.Writer
}

func newPromptWriter(w io.Writer) *promptWriter {
	return &promptWriter{w: w}
}

func (p *promptWriter) printf(format string, a ...any) {
	_, _ = fmt.Fprintf(p.w, format, a...)
}

func (p *promptWriter) println(s string) {
	_, _ = fmt.Fprintln(p.w, s)
}

// initCmd runs the interactive config generation wizard.
// It writes a config.toml file to the specified path.
func initCmd(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stdout)
	configPath := fs.String("config", "config.toml", "path to write config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if file already exists
	if _, err := os.Stat(*configPath); err == nil {
		return fmt.Errorf("%s already exists (remove it first or use --config to specify a different path)", *configPath)
	}

	scanner := bufio.NewScanner(stdin)
	w := newPromptWriter(stdout)

	prompt := func(question, defaultVal string) string {
		if defaultVal != "" {
			w.printf("%s [%s]: ", question, defaultVal)
		} else {
			w.printf("%s: ", question)
		}
		if !scanner.Scan() {
			return defaultVal
		}
		val := strings.TrimSpace(scanner.Text())
		if val == "" {
			return defaultVal
		}
		return val
	}

	promptYesNo := func(question string, defaultYes bool) bool {
		defStr := "Y/n"
		if !defaultYes {
			defStr = "y/N"
		}
		w.printf("%s [%s]: ", question, defStr)
		if !scanner.Scan() {
			return defaultYes
		}
		val := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if val == "" {
			return defaultYes
		}
		return val == "y" || val == "yes"
	}

	w.println("Feedreader Configuration")
	w.println(strings.Repeat("=", 40))
	w.println("")

	cfg := &config.Config{}

	// Server settings
	w.println("Server Settings")
	w.println(strings.Repeat("-", 20))
	cfg.Listen = prompt("Listen address", config.DefaultListen)
	cfg.DB = prompt("Database path", config.DefaultDB)
	w.println("")

	// Auth provider
	w.println("Authentication Provider")
	w.println(strings.Repeat("-", 20))
	w.println("Choose how users authenticate:")
	w.println("  1) proxy         - Generic reverse proxy headers (Caddy, nginx, etc.)")
	w.println("  2) tailscale     - Tailscale Serve/Funnel")
	w.println("  3) cloudflare    - Cloudflare Tunnel + Access")
	w.println("  4) authelia      - Authelia authentication server")
	w.println("  5) oauth2_proxy  - OAuth2 Proxy")
	w.println("  6) exedev        - exe.dev platform (legacy)")
	w.println("")

	providerChoice := prompt("Auth provider (1-6 or name)", "1")
	cfg.Auth.Provider = resolveProviderChoice(providerChoice)
	w.println("")

	// Provider-specific config
	switch cfg.Auth.Provider {
	case "proxy":
		w.println("Proxy Header Configuration")
		w.println(strings.Repeat("-", 20))
		userHeader := prompt("User ID header", "Remote-User")
		emailHeader := prompt("Email header", "Remote-Email")
		// Only include proxy config if non-default headers are used
		if userHeader != "Remote-User" || emailHeader != "Remote-Email" {
			cfg.Auth.Proxy = &config.ProxyAuthConfig{
				UserIDHeader: userHeader,
				EmailHeader:  emailHeader,
			}
		}
		w.println("")
	case "cloudflare":
		w.println("Cloudflare Access Configuration")
		w.println(strings.Repeat("-", 20))
		teamDomain := prompt("Team domain (e.g. myteam)", "")
		audience := prompt("Application Audience (AUD) tag", "")
		if teamDomain != "" || audience != "" {
			cfg.Auth.Cloudflare = &config.CloudflareAuthConfig{
				TeamDomain: teamDomain,
				Audience:   audience,
			}
		}
		w.println("")
	}

	// Newsletter configuration
	if promptYesNo("Configure newsletter ingestion?", false) {
		w.println("")
		w.println("Newsletter Configuration")
		w.println(strings.Repeat("-", 20))

		cfg.EmailDomain = prompt("Email domain for newsletters", "")

		if promptYesNo("Enable HTTP webhook ingestion?", false) {
			cfg.Newsletter.WebhookSecret = prompt("Webhook secret", "")
		}

		if promptYesNo("Enable built-in SMTP server?", false) {
			portStr := prompt("SMTP port", "2525")
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("invalid SMTP port %q: %w", portStr, err)
			}
			cfg.Newsletter.SMTPPort = port
		}
	}
	w.println("")

	// Write config
	if err := cfg.WriteFile(*configPath); err != nil {
		return err
	}

	w.printf("Config written to %s\n", *configPath)
	w.println("Start the server with:")
	w.printf("  feedreader --config %s\n", *configPath)
	return nil
}

// resolveProviderChoice maps numeric or name input to a provider identifier.
func resolveProviderChoice(input string) string {
	// Map numbers to provider names
	numMap := map[string]string{
		"1": "proxy",
		"2": "tailscale",
		"3": "cloudflare",
		"4": "authelia",
		"5": "oauth2_proxy",
		"6": "exedev",
	}
	if name, ok := numMap[input]; ok {
		return name
	}
	// Accept provider names directly
	for _, name := range config.AuthProviderNames {
		if strings.EqualFold(input, name) {
			return name
		}
	}
	// Default to proxy if unrecognized
	return "proxy"
}
