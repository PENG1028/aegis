package cluster

import (
	"database/sql"
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
func (ps *PendingState) PendingRevision() string {
	var revision string
	_ = ps.db.QueryRow(
		`SELECT updated_at FROM cluster_state WHERE key='pending_apply' AND value='true'`,
	).Scan(&revision)
	return revision
}

// ClearPendingIfUnchanged avoids erasing a mutation that arrived during Apply.
func (ps *PendingState) ClearPendingIfUnchanged(revision string) error {
	if revision == "" {
		return nil
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
