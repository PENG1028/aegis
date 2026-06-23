package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// OpenSQLite opens a SQLite database, creating the directory if needed.
func OpenSQLite(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create database directory %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %s: %w", path, err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return db, nil
}

// Initialize runs versioned database migrations.
func Initialize(db *sql.DB) error {
	return RunMigrations(db)
}

// GetMigrations returns the raw SQL statements (legacy API).
// Deprecated: use AllMigrations() for versioned migrations.
func GetMigrations() []string {
	migrations := AllMigrations()
	sqls := make([]string, len(migrations))
	for i, m := range migrations {
		sqls[i] = m.UpSQL
	}
	return sqls
}
