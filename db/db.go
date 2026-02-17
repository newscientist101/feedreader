package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
	"strings"

	_ "modernc.org/sqlite"
)

//go:generate go tool github.com/sqlc-dev/sqlc/cmd/sqlc generate

//go:embed migrations/*.sql
var migrationFS embed.FS

// Open opens an sqlite database and prepares pragmas suitable for a small web app.
func Open(path string) (*sql.DB, error) {
	// _pragma is applied by modernc.org/sqlite on every new connection,
	// ensuring all pool connections share the same settings.
	dsn := path +
		"?_time_format=sqlite" +
		"&_pragma=journal_mode(wal)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(on)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// RunMigrations executes database migrations in numeric order (NNN-*.sql),
// similar in spirit to exed's exedb.RunMigrations.
func RunMigrations(db *sql.DB) error {
	return runMigrations(db, migrationFS)
}

func runMigrations(db *sql.DB, mfs fs.ReadFileFS) error {
	entries, err := fs.ReadDir(mfs, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var migrations []string
	pat := regexp.MustCompile(`^(\d{3})-.*\.sql$`)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if pat.MatchString(name) {
			migrations = append(migrations, name)
		}
	}
	sort.Strings(migrations)

	executed := make(map[int]bool)
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&tableName)
	switch {
	case err == nil:
		rows, err := db.Query("SELECT migration_number FROM migrations")
		if err != nil {
			return fmt.Errorf("query executed migrations: %w", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var n int
			if err := rows.Scan(&n); err != nil {
				return fmt.Errorf("scan migration number: %w", err)
			}
			executed[n] = true
		}
	case errors.Is(err, sql.ErrNoRows):
		slog.Info("db: migrations table not found; running all migrations")
	default:
		return fmt.Errorf("check migrations table: %w", err)
	}

	for _, m := range migrations {
		match := pat.FindStringSubmatch(m)
		if len(match) != 2 {
			return fmt.Errorf("invalid migration filename: %s", m)
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return fmt.Errorf("parse migration number %s: %w", m, err)
		}
		if executed[n] {
			continue
		}
		if err := executeMigration(db, mfs, m, n); err != nil {
			return fmt.Errorf("execute %s: %w", m, err)
		}
		slog.Info("db: applied migration", "file", m, "number", n)
	}
	return nil
}

func executeMigration(db *sql.DB, mfs fs.ReadFileFS, filename string, number int) error {
	content, err := fs.ReadFile(mfs, "migrations/"+filename)
	if err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	// Migrations that recreate tables need foreign keys disabled to
	// prevent ON DELETE CASCADE from wiping referencing rows.
	// PRAGMA foreign_keys cannot be changed inside a transaction,
	// so we toggle it outside.
	if strings.Contains(string(content), "-- pragma:disable_fk") {
		if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
			return fmt.Errorf("disable fk for %s: %w", filename, err)
		}
		defer func() { _, _ = db.Exec("PRAGMA foreign_keys = ON") }()
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx for %s: %w", filename, err)
	}
	if _, err := tx.Exec(string(content)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec %s: %w", filename, err)
	}
	// Ensure the migration is recorded (some SQL files self-register,
	// so use INSERT OR IGNORE to avoid duplicates).
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO migrations (migration_number, migration_name) VALUES (?, ?)",
		number, filename,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record %s: %w", filename, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit %s: %w", filename, err)
	}
	return nil
}
