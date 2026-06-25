package cluster

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupPendingDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cluster_state (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func TestPendingStateMarkPending(t *testing.T) {
	db := setupPendingDB(t)
	defer db.Close()

	ps := NewPendingState(db)
	err := ps.MarkPending("route updated by admin")
	if err != nil {
		t.Fatalf("MarkPending: %v", err)
	}

	status := ps.Status()
	if !status.Pending {
		t.Error("expected pending=true after MarkPending")
	}
	if status.Reason != "route updated by admin" {
		t.Errorf("expected reason='route updated by admin', got '%s'", status.Reason)
	}
	if status.Since == "" {
		t.Error("expected non-empty since timestamp")
	}
	t.Logf("MarkPending OK: pending=%v reason=%s since=%s", status.Pending, status.Reason, status.Since)
}

func TestPendingStateClearPending(t *testing.T) {
	db := setupPendingDB(t)
	defer db.Close()

	ps := NewPendingState(db)
	ps.MarkPending("test reason")
	ps.ClearPending()

	status := ps.Status()
	if status.Pending {
		t.Error("expected pending=false after ClearPending")
	}
	t.Logf("ClearPending OK: pending=%v", status.Pending)
}

func TestPendingStateInitialState(t *testing.T) {
	db := setupPendingDB(t)
	defer db.Close()

	ps := NewPendingState(db)
	status := ps.Status()
	if status.Pending {
		t.Error("expected pending=false initially (no state set)")
	}
	t.Logf("Initial state OK: pending=%v", status.Pending)
}

func TestPendingStateRepeatedMark(t *testing.T) {
	db := setupPendingDB(t)
	defer db.Close()

	ps := NewPendingState(db)

	ps.MarkPending("first change")
	ps.MarkPending("second change")

	status := ps.Status()
	if !status.Pending {
		t.Error("expected pending=true after repeated marks")
	}
	if status.Reason != "second change" {
		t.Errorf("expected reason='second change', got '%s'", status.Reason)
	}
	t.Logf("Repeated mark OK: reason=%s", status.Reason)
}

func TestPendingStateMarkClearMark(t *testing.T) {
	db := setupPendingDB(t)
	defer db.Close()

	ps := NewPendingState(db)

	// Mark → Clear → Mark cycle
	ps.MarkPending("change 1")
	ps.ClearPending()
	ps.MarkPending("change 2")

	status := ps.Status()
	if !status.Pending {
		t.Error("expected pending=true after mark/clear/mark cycle")
	}
	if status.Reason != "change 2" {
		t.Errorf("expected reason='change 2', got '%s'", status.Reason)
	}
	t.Logf("Mark/Clear/Mark cycle OK: pending=%v reason=%s", status.Pending, status.Reason)
}
