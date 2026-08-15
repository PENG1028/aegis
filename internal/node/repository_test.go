package node

import (
	"database/sql"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// TestScanToleratesLegacyDatetimeFormat is a regression test: migration 043
// rewrote node rows with SQLite `datetime('now')` which stores
// "YYYY-MM-DD HH:MM:SS" — the scan code only understood RFC3339 and silently
// zeroed every timestamp (with the error ignored). Legacy-formatted rows
// must still parse to a usable time.
func TestScanToleratesLegacyDatetimeFormat(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	_, err := db.Exec(
		`INSERT INTO nodes (id, node_id, hostname, is_current, last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?, ?)`,
		"node_1", "node_test", "test-host",
		"2026-08-15 12:00:00", // legacy datetime('now') format
		"2026-08-15 11:00:00",
		"2026-08-15 10:00:00",
	)
	if err != nil {
		t.Fatal(err)
	}

	nodes, err := repo.FindAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(nodes))
	}
	n := nodes[0]
	if n.LastSeen.IsZero() {
		t.Fatal("LastSeen parsed to zero — legacy datetime format not handled")
	}
	if n.CreatedAt.IsZero() {
		t.Fatal("CreatedAt parsed to zero")
	}
	if n.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt parsed to zero")
	}
	// And the value must be the intended one, not just non-zero.
	if n.LastSeen.Format("2006-01-02 15:04:05") != "2026-08-15 12:00:00" {
		t.Fatalf("LastSeen = %v, want 2026-08-15 12:00:00", n.LastSeen)
	}
}

// TestScanParsesRFC3339 keeps the modern format working.
func TestScanParsesRFC3339(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	now := time.Now().Truncate(time.Second).Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO nodes (id, node_id, hostname, is_current, last_seen, created_at, updated_at)
		 VALUES (?, ?, ?, 1, ?, ?, ?)`,
		"node_2", "node_rfc", "host2", now, now, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := repo.FindAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].LastSeen.IsZero() {
		t.Fatalf("RFC3339 row parsed incorrectly: %+v", nodes)
	}
}
