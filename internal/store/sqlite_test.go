package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// TestOpenSQLiteSetsBusyTimeoutOnAllConnections is a regression test: the
// DSN used `_busy_timeout=5000` — a mattn/go-sqlite3 parameter that
// modernc.org/sqlite silently ignores — and `PRAGMA busy_timeout` was only
// executed on ONE pooled connection. Every other connection in the pool
// failed immediately with SQLITE_BUSY on concurrent writes (observed as
// periodic "database is locked" from peer heartbeats).
func TestOpenSQLiteSetsBusyTimeoutOnAllConnections(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "busy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	// Hold MaxOpenConns connections SIMULTANEOUSLY so the pool must hand out
	// distinct connections — a sequential Conn() call may reuse the idle one
	// that ran the PRAGMA, hiding the bug.
	conns := make([]*sql.Conn, 4)
	for i := range conns {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns[i] = c
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	for i, c := range conns {
		var ms int
		if err := c.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&ms); err != nil {
			t.Fatal(err)
		}
		if ms < 5000 {
			t.Fatalf("connection %d busy_timeout = %d ms, want >= 5000 (pool connections without timeout fail instantly on write contention)", i, ms)
		}
	}
}

// TestOpenSQLiteWALMode ensures WAL is also applied per-connection via DSN.
func TestOpenSQLiteWALMode(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "wal.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}
