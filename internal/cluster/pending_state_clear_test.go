package cluster

import (
	"database/sql"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

func pendingFixture(t *testing.T) (*PendingState, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return NewPendingState(db), db
}

// TestClearPendingIfUnchangedClearsMarkerWithoutRevision is the regression for a
// pending marker that outlived fifteen days and every restart in between.
//
// ClearPendingIfUnchanged used to return nil when the revision was empty, doing
// nothing. An empty revision is exactly what a row written by an older scheme
// produces — no readable updated_at — so the compare-and-set could never match it
// and the flag stayed true forever. An operator reading it believes an apply is
// outstanding when none is.
func TestClearPendingIfUnchangedClearsMarkerWithoutRevision(t *testing.T) {
	ps, db := pendingFixture(t)

	// A marker with no timestamp, the shape a pre-revision writer left behind.
	if _, err := db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at) VALUES ('pending_apply', 'true', NULL)`,
	); err != nil {
		t.Fatal(err)
	}
	if !ps.Status().Pending {
		t.Fatal("fixture did not mark pending")
	}
	if rev := ps.PendingRevision(); rev != "" {
		t.Fatalf("expected an empty revision for a NULL timestamp, got %q", rev)
	}

	if err := ps.ClearPendingIfUnchanged(ps.PendingRevision()); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if ps.Status().Pending {
		t.Error("marker survived the clear — this is the state that persisted for fifteen days")
	}
}

// TestClearPendingIfUnchangedKeepsMarkerWrittenDuringApply pins the reason the
// compare-and-set exists. A mutation that lands while Apply is running must not have
// its marker erased by that Apply finishing, or the new change is stranded unapplied.
func TestClearPendingIfUnchangedKeepsMarkerWrittenDuringApply(t *testing.T) {
	ps, _ := pendingFixture(t)

	if err := ps.MarkPending("first change"); err != nil {
		t.Fatal(err)
	}
	revision := ps.PendingRevision()
	if revision == "" {
		t.Fatal("MarkPending produced no revision")
	}

	// A second mutation arrives mid-apply, moving updated_at forward.
	time.Sleep(2 * time.Millisecond)
	if err := ps.MarkPending("second change arrived during apply"); err != nil {
		t.Fatal(err)
	}

	// Apply finishes and tries to clear the revision it captured on entry.
	if err := ps.ClearPendingIfUnchanged(revision); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if !ps.Status().Pending {
		t.Error("the second mutation's marker was erased — that change would never be applied")
	}
	if got := ps.Status().Reason; got != "second change arrived during apply" {
		t.Errorf("reason = %q, want the second mutation's", got)
	}
}

// TestClearPendingIfUnchangedClearsMatchingRevision is the ordinary path: nothing
// arrived during Apply, so the marker Apply saw is the one it clears.
func TestClearPendingIfUnchangedClearsMatchingRevision(t *testing.T) {
	ps, _ := pendingFixture(t)

	if err := ps.MarkPending("a change"); err != nil {
		t.Fatal(err)
	}
	if err := ps.ClearPendingIfUnchanged(ps.PendingRevision()); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if ps.Status().Pending {
		t.Error("an unchanged marker should have been cleared")
	}
}

// TestPendingRevisionEmptyWhenNothingPending guards that the no-rows case is not
// treated as an error and does not log noise on every apply.
func TestPendingRevisionEmptyWhenNothingPending(t *testing.T) {
	ps, _ := pendingFixture(t)

	if rev := ps.PendingRevision(); rev != "" {
		t.Errorf("revision = %q, want empty when nothing is pending", rev)
	}
	if err := ps.ClearPendingIfUnchanged(""); err != nil {
		t.Errorf("clearing with nothing pending should be a no-op, got %v", err)
	}
}
