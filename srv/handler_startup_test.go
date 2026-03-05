package srv

import (
	"os"
	"path/filepath"
	"testing"
)

// --------------- New ---------------

func TestNew(t *testing.T) {
	// New creates a real DB file, so use a temp dir.
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	srv, err := New(dbPath, "testhost")
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer srv.DB.Close()

	if srv.Hostname != "testhost" {
		t.Errorf("Hostname = %q, want \"testhost\"", srv.Hostname)
	}
	if srv.DB == nil {
		t.Error("DB is nil")
	}
	if srv.Fetcher == nil {
		t.Error("Fetcher is nil")
	}
	if srv.ScraperRunner == nil {
		t.Error("ScraperRunner is nil")
	}
	if srv.StaticHashes == nil {
		t.Error("StaticHashes is nil")
	}
	if srv.TemplatesDir == "" {
		t.Error("TemplatesDir is empty")
	}
	if srv.StaticDir == "" {
		t.Error("StaticDir is empty")
	}

	// Verify DB is functional by running a simple query.
	var n int
	if err := srv.DB.QueryRow("SELECT 1").Scan(&n); err != nil {
		t.Fatalf("DB query failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("SELECT 1 = %d", n)
	}

	// Verify migrations ran — the migrations table should exist.
	var tableName string
	err = srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("migrations table not found: %v", err)
	}
}

func TestNew_BadPath(t *testing.T) {
	// An invalid path should return an error.
	_, err := New("/nonexistent/dir/that/does/not/exist/db.sqlite", "host")
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
}

// --------------- hashStaticFiles ---------------

func TestHashStaticFiles(t *testing.T) {
	// Create a temp dir with known files to test hashing.
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "style.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	subDir := filepath.Join(tmpDir, "icons")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "icon.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}

	hashes := hashStaticFiles(tmpDir)

	if len(hashes) != 2 {
		t.Fatalf("expected 2 hashes, got %d: %v", len(hashes), hashes)
	}

	// Check that style.css has a hash.
	if h, ok := hashes["style.css"]; !ok {
		t.Error("missing hash for style.css")
	} else if len(h) != 8 {
		t.Errorf("hash for style.css has length %d, want 8", len(h))
	}

	// Check the nested file.
	key := filepath.Join("icons", "icon.svg")
	if h, ok := hashes[key]; !ok {
		t.Errorf("missing hash for %s", key)
	} else if len(h) != 8 {
		t.Errorf("hash for %s has length %d, want 8", key, len(h))
	}
}

func TestHashStaticFiles_NonexistentDir(t *testing.T) {
	hashes := hashStaticFiles("/nonexistent/dir/xyz")
	if len(hashes) != 0 {
		t.Errorf("expected empty map for missing dir, got %v", hashes)
	}
}

func TestHashStaticFiles_Deterministic(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.js"), []byte("console.log('hi')"), 0o644)

	h1 := hashStaticFiles(tmpDir)
	h2 := hashStaticFiles(tmpDir)

	if h1["a.js"] != h2["a.js"] {
		t.Errorf("hashes not deterministic: %q vs %q", h1["a.js"], h2["a.js"])
	}
}

// --------------- setUpDatabase ---------------

func TestSetUpDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "setup_test.db")

	srv := &Server{}
	if err := srv.setUpDatabase(dbPath); err != nil {
		t.Fatalf("setUpDatabase() error: %v", err)
	}
	defer srv.DB.Close()

	// DB should be set.
	if srv.DB == nil {
		t.Fatal("DB is nil after setUpDatabase")
	}

	// Migrations table should exist.
	var tableName string
	err := srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	if err != nil {
		t.Fatalf("migrations table not found: %v", err)
	}

	// Core tables should exist (feeds, articles, users, etc.).
	for _, table := range []string{"feeds", "articles", "users", "categories", "scraper_modules"} {
		var name string
		err := srv.DB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", table, err)
		}
	}

	// WAL mode should be enabled.
	var journalMode string
	srv.DB.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\"", journalMode)
	}

	// Foreign keys should be enabled.
	var fk int
	srv.DB.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	if fk != 1 {
		t.Errorf("foreign_keys = %d, want 1", fk)
	}
}

func TestSetUpDatabase_BadPath(t *testing.T) {
	srv := &Server{}
	err := srv.setUpDatabase("/nonexistent/dir/that/does/not/exist/db.sqlite")
	if err == nil {
		t.Fatal("expected error for invalid DB path")
	}
}

// --------------- Serve (skipped) ---------------
// Serve is not unit-tested because it calls http.ListenAndServe which
// blocks until the server shuts down, starts background goroutines
// (Fetcher, RetentionManager, EmailWatcher), and binds a real TCP port.
// Integration testing of Serve would require orchestrating a clean
// shutdown which is out of scope for unit tests. The Handler() method
// it uses is indirectly tested through all the API handler tests.

// --------------- Notes ---------------
// hashStaticFiles: Testable directly since it uses os.ReadFile on a real
// directory (not embed.FS). Tests above use temp directories.
