package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// OpenSQLite opens a SQLite database, creating the directory if needed.
// Production safety: WAL mode, busy_timeout, single writer, integrity check.
func OpenSQLite(path string) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("create database directory %s: %w", dir, err)
		}
	}

	// busy_timeout=5000: wait up to 5s on SQLITE_BUSY instead of failing immediately.
	// Critical for multi-node setups where concurrent writes collide.
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %s: %w", path, err)
	}
	// Ensure WAL mode on existing databases (pragma only applies to new DBs).
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	// SQLite is fundamentally single-writer. Limit to 1 open conn to prevent
	// "database is locked" errors under concurrent write load.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	// Startup integrity check — detect corruption early, not at query time.
	var ok string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&ok); err != nil || ok != "ok" {
		db.Close()
		if err != nil {
			return nil, fmt.Errorf("integrity check failed: %w", err)
		}
		return nil, fmt.Errorf("database integrity check failed: %s", ok)
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
