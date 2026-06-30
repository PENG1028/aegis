// Package testutil provides shared test helpers for Aegis tests.
// Import this package in _test.go files to eliminate duplicated setup code.
package testutil

import (
	"database/sql"
	"testing"

	"aegis/internal/store"

	_ "github.com/mattn/go-sqlite3"
)

// SetupTestDB creates an in-memory SQLite database with all migrations applied.
// The returned cleanup function closes the DB. Use in tests like:
//
//	db, cleanup := testutil.SetupTestDB(t)
//	defer cleanup()
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := store.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, cleanup
}

// SetupTestDBFile creates a file-backed SQLite database for integration tests.
// The temp file is removed on cleanup.
func SetupTestDBFile(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()

	path := t.TempDir() + "/test.db"
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	if err := store.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	cleanup := func() {
		db.Close()
	}

	return db, path, cleanup
}
