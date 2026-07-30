package cluster

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

// PendingState tracks whether desired state differs from applied state.
// Uses the cluster_state key-value table with key 'pending_apply'.
type PendingState struct {
	db *sql.DB
}

// NewPendingState creates a pending state tracker.
func NewPendingState(db *sql.DB) *PendingState {
	return &PendingState{db: db}
}

// PendingApplyStatus represents the current pending apply state.
type PendingApplyStatus struct {
	Pending bool   `json:"pending"`
	Since   string `json:"since,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// MarkPending sets the pending_apply flag to true with a reason.
func (ps *PendingState) MarkPending(reason string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := ps.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply', 'true', ?)`,
		now,
	)
	if err != nil {
		return err
	}
	// Store reason + since separately
	if _, err = tx.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply_reason', ?, ?)`,
		reason, now,
	); err != nil {
		return err
	}
	if _, err = tx.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply_since', ?, ?)`,
		now, now,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingRevision returns the revision an Apply operation is allowed to clear.
// An empty revision means no change was pending when Apply started.
// PendingRevision returns the updated_at of the current pending marker, or "" when
// nothing is pending or the row has no readable timestamp.
//
// sql.ErrNoRows is the normal "nothing pending" case and is not worth reporting. Any
// other error means the query itself failed, and swallowing it would present a
// storage problem as an absent marker — the caller then skips a compare-and-set it
// should have performed.
func (ps *PendingState) PendingRevision() string {
	var revision sql.NullString
	err := ps.db.QueryRow(
		`SELECT updated_at FROM cluster_state WHERE key='pending_apply' AND value='true'`,
	).Scan(&revision)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("pending-state: read revision: %v", err)
		return ""
	}
	return revision.String
}

// ClearPendingIfUnchanged avoids erasing a mutation that arrived during Apply.
//
// An empty revision means PendingRevision() found no row with a readable
// updated_at — either nothing is pending, or the row predates this scheme and
// carries no timestamp. The compare-and-set below cannot match either case, so it
// falls back to an unconditional clear rather than leaving the flag set forever.
// A stale pending marker is worse than a lost one: it tells an operator an apply is
// outstanding when none is, and the r7 report found one that had survived fifteen
// days and every restart in between.
func (ps *PendingState) ClearPendingIfUnchanged(revision string) error {
	if revision == "" {
		return ps.ClearPending()
	}
	_, err := ps.db.Exec(
		`UPDATE cluster_state SET value='false', updated_at=?
		 WHERE key='pending_apply' AND value='true' AND updated_at=?`,
		time.Now().UTC().Format(time.RFC3339Nano), revision,
	)
	return err
}

// ClearPending resets the pending_apply flag after successful apply.
func (ps *PendingState) ClearPending() error {
	_, err := ps.db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply', 'false', datetime('now'))`)
	return err
}

// Status returns the current pending apply status.
func (ps *PendingState) Status() PendingApplyStatus {
	var pending string
	err := ps.db.QueryRow(
		`SELECT COALESCE(value, 'false') FROM cluster_state WHERE key = 'pending_apply'`).Scan(&pending)
	if err != nil || pending != "true" {
		return PendingApplyStatus{Pending: false}
	}

	var reason, since string
	_ = ps.db.QueryRow(
		`SELECT COALESCE(value, '') FROM cluster_state WHERE key = 'pending_apply_reason'`).Scan(&reason)
	_ = ps.db.QueryRow(
		`SELECT COALESCE(value, '') FROM cluster_state WHERE key = 'pending_apply_since'`).Scan(&since)

	return PendingApplyStatus{
		Pending: true,
		Since:   since,
		Reason:  reason,
	}
}
