package main

import (
	"flag"
	"fmt"
	"os"

	"srv.exe.dev/srv"
)

var flagListenAddr = flag.String("listen", ":8000", "address to listen on")

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}

func run() error {
	flag.Parse()
	server, err := initServer("db.sqlite3")
	if err != nil {
		return err
	}
	return server.Serve(*flagListenAddr)
}

// initServer creates and configures the server. Separated from run()
// so tests can exercise initialization without blocking on Serve().
func initServer(dbPath string) (*srv.Server, error) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	server, err := srv.New(dbPath, hostname)
	if err != nil {
		return nil, fmt.Errorf("create server: %w", err)
	}
	return server, nil
}
