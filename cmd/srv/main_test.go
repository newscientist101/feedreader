package main

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"srv.exe.dev/srv"
)

func TestFlagDefault(t *testing.T) {
	// The package-level flag should default to ":8000".
	want := ":8000"
	got := *flagListenAddr
	if got != want {
		t.Errorf("default listen address = %q, want %q", got, want)
	}

	// Also verify it's registered in the flag set.
	f := flag.Lookup("listen")
	if f == nil {
		t.Fatal(`flag "listen" not registered`)
	}
	if f.DefValue != want {
		t.Errorf("flag DefValue = %q, want %q", f.DefValue, want)
	}
}

func TestNewServerInvalidDBPath(t *testing.T) {
	// A path inside a non-existent directory should fail to open/create.
	badPath := filepath.Join(t.TempDir(), "no", "such", "dir", "db.sqlite3")
	_, err := srv.New(badPath, "testhost")
	if err == nil {
		t.Fatal("expected error for invalid DB path, got nil")
	}
}

func TestNewServerValidDBPath(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.sqlite3")
	server, err := srv.New(dbPath, "testhost")
	if err != nil {
		t.Fatalf("srv.New with temp DB: %v", err)
	}
	if server == nil {
		t.Fatal("srv.New returned nil server")
	}
	// Clean up the DB connection.
	if server.DB != nil {
		server.DB.Close()
	}
	// The DB file should have been created.
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("expected DB file to be created")
	}
}
