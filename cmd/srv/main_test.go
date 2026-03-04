package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
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
