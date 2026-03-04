package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/newscientist101/feedreader/srv"
)

var flagListenAddr = flag.String("listen", ":8000", "address to listen on")
var flagDBPath = flag.String("db", "db.sqlite3", "path to SQLite database file")
var flagEmailDomain = flag.String("email-domain", "", "email domain suffix (default: auto-detect from hostname)")

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	flag.Parse()
	server, err := initServer(*flagDBPath, *flagEmailDomain)
	if err != nil {
		return err
	}
	return server.Serve(*flagListenAddr)
}

// initServer creates and configures the server. Separated from run()
// so tests can exercise initialization without blocking on Serve().
func initServer(dbPath, emailDomain string) (*srv.Server, error) {
	if emailDomain == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		// The OS hostname is short (e.g. "lynx-fairy"); the exe.dev
		// email domain requires the full ".exe.xyz" suffix.
		if !strings.Contains(hostname, ".") {
			hostname += ".exe.xyz"
		}
		emailDomain = hostname
	}
	server, err := srv.New(dbPath, emailDomain)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	return server, nil
}
