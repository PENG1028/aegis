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
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := ps.db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply', 'true', ?)`,
		now,
	)
	if err != nil {
		return err
	}
	// Store reason + since separately
	_, _ = ps.db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply_reason', ?, ?)`,
		reason, now,
	)
	_, _ = ps.db.Exec(
		`INSERT OR REPLACE INTO cluster_state (key, value, updated_at)
		 VALUES ('pending_apply_since', ?, ?)`,
		now, now,
	)
	return nil
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
