package nodestate

import (
	"database/sql"
	"fmt"
	"time"
)

// Repository provides database access for node state data.
type Repository struct {
	DB *sql.DB
}

// NewRepository creates a new nodestate repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{DB: db}
}

// ============================================================================
// Desired State
// ============================================================================

// CreateDesiredState inserts a new desired state.
func (r *Repository) CreateDesiredState(ds *DesiredState) error {
	supersededAt := ""
	if !ds.SupersededAt.IsZero() {
		supersededAt = ds.SupersededAt.Format(time.RFC3339)
	}
	_, err := r.DB.Exec(
		`INSERT INTO node_desired_states (id, node_id, revision, state_hash, state_json, status, reason, created_by, created_at, superseded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ds.ID, ds.NodeID, ds.Revision, ds.StateHash, ds.StateJSON,
		ds.Status, ds.Reason, ds.CreatedBy,
		ds.CreatedAt.Format(time.RFC3339), supersededAt,
	)
	if err != nil {
		return fmt.Errorf("insert desired state: %w", err)
	}
	return nil
}

// GetLatestDesiredState returns the latest active desired state for a node.
func (r *Repository) GetLatestDesiredState(nodeID string) (*DesiredState, error) {
	ds, err := r.getDesiredState(
		`SELECT id, node_id, revision, state_hash, state_json, status, reason, created_by, created_at, superseded_at
		 FROM node_desired_states WHERE node_id = ? AND status = 'active'
		 ORDER BY revision DESC LIMIT 1`, nodeID)
	if err != nil {
		return nil, err
	}
	return ds, nil
}

// GetDesiredStateByRevision returns a specific revision for a node.
func (r *Repository) GetDesiredStateByRevision(nodeID string, revision int) (*DesiredState, error) {
	return r.getDesiredState(
		`SELECT id, node_id, revision, state_hash, state_json, status, reason, created_by, created_at, superseded_at
		 FROM node_desired_states WHERE node_id = ? AND revision = ?`, nodeID, revision)
}

// ListDesiredStates returns all desired states for a node, newest first.
func (r *Repository) ListDesiredStates(nodeID string) ([]DesiredState, error) {
	rows, err := r.DB.Query(
		`SELECT id, node_id, revision, state_hash, state_json, status, reason, created_by, created_at, superseded_at
		 FROM node_desired_states WHERE node_id = ? ORDER BY revision DESC`, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDesiredStates(rows)
}

// GetLatestRevision returns the latest revision number for a node.
func (r *Repository) GetLatestRevision(nodeID string) (int, error) {
	var rev int
	err := r.DB.QueryRow(
		`SELECT COALESCE(MAX(revision), 0) FROM node_desired_states WHERE node_id = ? AND status = 'active'`, nodeID).Scan(&rev)
	if err != nil {
		return 0, err
	}
	return rev, nil
}

// SupersedePrevious marks all active desired states for a node as superseded.
func (r *Repository) SupersedePrevious(nodeID string, exceptRevision int) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.DB.Exec(
		`UPDATE node_desired_states SET status=?, superseded_at=? WHERE node_id=? AND status='active' AND revision != ?`,
		DSStatusSuperseded, now, nodeID, exceptRevision)
	return err
}

func (r *Repository) getDesiredState(query string, args ...interface{}) (*DesiredState, error) {
	var ds DesiredState
	var createdAt, supersededAt string
	err := r.DB.QueryRow(query, args...).Scan(
		&ds.ID, &ds.NodeID, &ds.Revision, &ds.StateHash, &ds.StateJSON,
		&ds.Status, &ds.Reason, &ds.CreatedBy, &createdAt, &supersededAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query desired state: %w", err)
	}
	ds.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if supersededAt != "" {
		ds.SupersededAt, _ = time.Parse(time.RFC3339, supersededAt)
	}
	return &ds, nil
}

func scanDesiredStates(rows *sql.Rows) ([]DesiredState, error) {
	var states []DesiredState
	for rows.Next() {
		var ds DesiredState
		var createdAt, supersededAt string
		if err := rows.Scan(&ds.ID, &ds.NodeID, &ds.Revision, &ds.StateHash, &ds.StateJSON,
			&ds.Status, &ds.Reason, &ds.CreatedBy, &createdAt, &supersededAt); err != nil {
			return nil, fmt.Errorf("scan desired state: %w", err)
		}
		ds.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if supersededAt != "" {
			ds.SupersededAt, _ = time.Parse(time.RFC3339, supersededAt)
		}
		states = append(states, ds)
	}
	if states == nil {
		states = []DesiredState{}
	}
	return states, rows.Err()
}

// ============================================================================
// Actual State
// ============================================================================

// UpsertActualState creates or updates the actual state for a node.
func (r *Repository) UpsertActualState(as *ActualState) error {
	now := as.UpdatedAt.Format(time.RFC3339)
	lastApply := ""
	if !as.LastApplyAt.IsZero() {
		lastApply = as.LastApplyAt.Format(time.RFC3339)
	}
	lastSuccess := ""
	if !as.LastSuccessAt.IsZero() {
		lastSuccess = as.LastSuccessAt.Format(time.RFC3339)
	}
	reportedAt := ""
	if !as.ReportedAt.IsZero() {
		reportedAt = as.ReportedAt.Format(time.RFC3339)
	}

	_, err := r.DB.Exec(
		`INSERT INTO node_actual_states (id, node_id, applied_revision, state_hash, status, last_apply_at, last_success_at, last_error, provider_status, relay_status, gateway_status, diagnostics_status, reported_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(node_id) DO UPDATE SET
		 applied_revision=excluded.applied_revision, state_hash=excluded.state_hash, status=excluded.status,
		 last_apply_at=excluded.last_apply_at, last_success_at=excluded.last_success_at, last_error=excluded.last_error,
		 provider_status=excluded.provider_status, relay_status=excluded.relay_status, gateway_status=excluded.gateway_status,
		 diagnostics_status=excluded.diagnostics_status, reported_at=excluded.reported_at, updated_at=excluded.updated_at`,
		as.ID, as.NodeID, as.AppliedRevision, as.StateHash, as.Status,
		lastApply, lastSuccess, as.LastError,
		as.ProviderStatus, as.RelayStatus, as.GatewayStatus, as.DiagnosticsStatus,
		reportedAt, as.CreatedAt.Format(time.RFC3339), now,
	)
	if err != nil {
		return fmt.Errorf("upsert actual state: %w", err)
	}
	return nil
}

// GetActualState returns the latest actual state for a node.
func (r *Repository) GetActualState(nodeID string) (*ActualState, error) {
	var as ActualState
	var createdAt, updatedAt, lastApply, lastSuccess, reportedAt string
	err := r.DB.QueryRow(
		`SELECT id, node_id, applied_revision, state_hash, status, last_apply_at, last_success_at, last_error,
		 provider_status, relay_status, gateway_status, diagnostics_status, reported_at, created_at, updated_at
		 FROM node_actual_states WHERE node_id = ?`, nodeID,
	).Scan(&as.ID, &as.NodeID, &as.AppliedRevision, &as.StateHash, &as.Status,
		&lastApply, &lastSuccess, &as.LastError,
		&as.ProviderStatus, &as.RelayStatus, &as.GatewayStatus, &as.DiagnosticsStatus,
		&reportedAt, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query actual state: %w", err)
	}
	as.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	as.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if lastApply != "" {
		as.LastApplyAt, _ = time.Parse(time.RFC3339, lastApply)
	}
	if lastSuccess != "" {
		as.LastSuccessAt, _ = time.Parse(time.RFC3339, lastSuccess)
	}
	if reportedAt != "" {
		as.ReportedAt, _ = time.Parse(time.RFC3339, reportedAt)
	}
	return &as, nil
}
