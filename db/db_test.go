package db

import (
	"database/sql"
	"io/fs"
	"testing"
	"testing/fstest"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatal(err)
	}
	return db
}

func makeFS(files map[string]string) fs.ReadFileFS {
	m := fstest.MapFS{}
	for name, content := range files {
		m["migrations/"+name] = &fstest.MapFile{Data: []byte(content)}
	}
	return m
}

func TestRunMigrations_Basic(t *testing.T) {
	db := openTestDB(t)
	mfs := makeFS(map[string]string{
		"001-init.sql": `
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');
`,
		"002-add-col.sql": `
ALTER TABLE items ADD COLUMN value TEXT;
INSERT INTO migrations (migration_number, migration_name) VALUES (2, '002-add-col.sql');
`,
	})

	if err := runMigrations(db, mfs); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Verify tables exist
	var count int
	if err := db.QueryRow("SELECT count(*) FROM migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 migrations recorded, got %d", count)
	}

	// Insert a row to verify schema
	if _, err := db.Exec("INSERT INTO items (name, value) VALUES ('a', 'b')"); err != nil {
		t.Fatalf("insert into items: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openTestDB(t)
	mfs := makeFS(map[string]string{
		"001-init.sql": `
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');
`,
	})

	// Run twice — second run should be a no-op
	if err := runMigrations(db, mfs); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := runMigrations(db, mfs); err != nil {
		t.Fatalf("second run (should be idempotent): %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 migration recorded, got %d", count)
	}
}

func TestRunMigrations_SkipsAlreadyExecuted(t *testing.T) {
	db := openTestDB(t)

	initSQL := `
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');
`
	if err := runMigrations(db, makeFS(map[string]string{"001-init.sql": initSQL})); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Now add a second migration and run again — 001 should be skipped
	mfs := makeFS(map[string]string{
		"001-init.sql": initSQL,
		"002-add-col.sql": `
ALTER TABLE items ADD COLUMN value TEXT;
INSERT INTO migrations (migration_number, migration_name) VALUES (2, '002-add-col.sql');
`,
	})

	if err := runMigrations(db, mfs); err != nil {
		t.Fatalf("second run with new migration: %v", err)
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 migrations, got %d", count)
	}

	// Verify the new column exists
	if _, err := db.Exec("INSERT INTO items (name, value) VALUES ('a', 'b')"); err != nil {
		t.Fatalf("insert with new column: %v", err)
	}
}

func TestRunMigrations_TransactionRollback(t *testing.T) {
	db := openTestDB(t)

	initSQL := `
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');
`
	if err := runMigrations(db, makeFS(map[string]string{"001-init.sql": initSQL})); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Now try a migration that will fail partway through
	badFS := makeFS(map[string]string{
		"001-init.sql": initSQL,
		"002-bad.sql": `
CREATE TABLE new_table (id INTEGER PRIMARY KEY);
INSERT INTO nonexistent_table VALUES (1);
INSERT INTO migrations (migration_number, migration_name) VALUES (2, '002-bad.sql');
`,
	})

	err := runMigrations(db, badFS)
	if err == nil {
		t.Fatal("expected error from bad migration, got nil")
	}

	// The failed migration should have been rolled back:
	// - new_table should NOT exist
	// - migration 2 should NOT be recorded
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='new_table'").Scan(&tableName)
	if err == nil {
		t.Fatal("new_table should not exist after rollback, but it does")
	}

	var count int
	if err := db.QueryRow("SELECT count(*) FROM migrations WHERE migration_number = 2").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("migration 2 should not be recorded after rollback")
	}
}

func TestRunMigrations_RealMigrations(t *testing.T) {
	// Smoke test: run the actual embedded migrations against an in-memory DB
	db := openTestDB(t)
	if err := RunMigrations(db); err != nil {
		t.Fatalf("real migrations failed: %v", err)
	}

	// Verify key tables exist
	for _, table := range []string{"migrations", "feeds", "articles", "users", "categories", "queue_articles", "user_settings", "nntp_credentials", "usenet_feed_state", "usenet_article_meta"} {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found after migrations: %v", table, err)
		}
	}

	// Run again — all migrations are recorded so they should all be skipped
	if err := RunMigrations(db); err != nil {
		t.Fatalf("second run should skip all recorded migrations: %v", err)
	}

	// Verify correct number of migrations recorded
	var count int
	if err := db.QueryRow("SELECT count(*) FROM migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 23 {
		t.Fatalf("expected 23 migrations recorded, got %d", count)
	}
}

func TestRunMigrations_DisableFKDirective(t *testing.T) {
	db := openTestDB(t)

	initSQL := `
CREATE TABLE IF NOT EXISTS migrations (
  migration_number INTEGER PRIMARY KEY,
  migration_name TEXT NOT NULL,
  executed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE parent (id INTEGER PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE child (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER NOT NULL REFERENCES parent(id) ON DELETE CASCADE
);
INSERT INTO parent VALUES (1, 'a');
INSERT INTO parent VALUES (2, 'b');
INSERT INTO child VALUES (10, 1);
INSERT INTO child VALUES (20, 2);
INSERT INTO migrations (migration_number, migration_name) VALUES (1, '001-init.sql');
`

	// Migration that recreates the parent table with a different constraint.
	// Without pragma:disable_fk, the DROP TABLE would cascade-delete child rows.
	recreateSQL := `
-- pragma:disable_fk
CREATE TABLE parent_new (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  extra TEXT,
  UNIQUE(name)
);
INSERT INTO parent_new (id, name) SELECT id, name FROM parent;
DROP TABLE parent;
ALTER TABLE parent_new RENAME TO parent;
`

	mfs := makeFS(map[string]string{
		"001-init.sql":     initSQL,
		"002-recreate.sql": recreateSQL,
	})

	if err := runMigrations(db, mfs); err != nil {
		t.Fatalf("migrations failed: %v", err)
	}

	// Child rows must still exist (CASCADE did not fire)
	var count int
	if err := db.QueryRow("SELECT count(*) FROM child").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 child rows preserved, got %d", count)
	}

	// Parent rows must still exist
	if err := db.QueryRow("SELECT count(*) FROM parent").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 parent rows, got %d", count)
	}

	// Foreign keys should be re-enabled after migration
	var fkEnabled int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled); err != nil {
		t.Fatal(err)
	}
	if fkEnabled != 1 {
		t.Fatal("foreign keys should be re-enabled after migration")
	}
}

func TestOpen_PragmasApplied(t *testing.T) {
	// Open a real (temp-file) database via Open() and verify that
	// the _pragma DSN parameters are applied on every connection.
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"

	d, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })

	// Run migrations so tables exist
	if err := RunMigrations(d); err != nil {
		t.Fatal(err)
	}

	// Force multiple connections by setting pool size > 1
	d.SetMaxOpenConns(4)

	// Verify pragmas on several connections (grab them in parallel)
	for i := range 4 {
		var timeout int
		if err := d.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil {
			t.Fatal(err)
		}
		if timeout != 5000 {
			t.Errorf("connection %d: busy_timeout = %d, want 5000", i, timeout)
		}

		var journal string
		if err := d.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
			t.Fatal(err)
		}
		if journal != "wal" {
			t.Errorf("connection %d: journal_mode = %q, want wal", i, journal)
		}

		var fk int
		if err := d.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}
		if fk != 1 {
			t.Errorf("connection %d: foreign_keys = %d, want 1", i, fk)
		}
	}
}
