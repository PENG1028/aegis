package nodestate

import "time"

// DesiredState status constants.
const (
	DSStatusActive     = "active"
	DSStatusSuperseded = "superseded"
	DSStatusFailed     = "failed"
)

// DesiredState represents a target configuration state for a node.
type DesiredState struct {
	ID          string    `json:"id"`
	NodeID      string    `json:"node_id"`
	Revision    int       `json:"revision"`
	StateHash   string    `json:"state_hash"`
	StateJSON   string    `json:"state_json"`
	Status      string    `json:"status"` // active | superseded | failed
	Reason      string    `json:"reason,omitempty"`
	CreatedBy   string    `json:"created_by,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	SupersededAt time.Time `json:"superseded_at,omitempty"`
}

// ActualState status constants.
const (
	ASStatusUnknown   = "unknown"
	ASStatusApplying  = "applying"
	ASStatusApplied   = "applied"
	ASStatusFailed    = "failed"
	ASStatusDegraded  = "degraded"
)

// ActualState represents a node's reported actual state.
type ActualState struct {
	ID               string    `json:"id"`
	NodeID           string    `json:"node_id"`
	AppliedRevision  int       `json:"applied_revision"`
	StateHash        string    `json:"state_hash,omitempty"`
	Status           string    `json:"status"` // unknown | applying | applied | failed | degraded
	LastApplyAt      time.Time `json:"last_apply_at,omitempty"`
	LastSuccessAt    time.Time `json:"last_success_at,omitempty"`
	LastError        string    `json:"last_error,omitempty"`
	ProviderStatus   string    `json:"provider_status,omitempty"`
	RelayStatus      string    `json:"relay_status,omitempty"`
	GatewayStatus    string    `json:"gateway_status,omitempty"`
	DiagnosticsStatus string   `json:"diagnostics_status,omitempty"`
	ReportedAt       time.Time `json:"reported_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// SyncStatus represents the sync state comparison.
type SyncStatus struct {
	NodeID          string `json:"node_id"`
	Status          string `json:"status"` // in_sync | outdated | no_desired_state | no_actual_state | failed | degraded
	DesiredRevision int    `json:"desired_revision"`
	AppliedRevision int    `json:"applied_revision"`
	DesiredHash     string `json:"desired_hash,omitempty"`
	ActualHash      string `json:"actual_hash,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

// Sync status constants.
const (
	SyncInSync         = "in_sync"
	SyncOutdated       = "outdated"
	SyncNoDesiredState = "no_desired_state"
	SyncNoActualState  = "no_actual_state"
	SyncFailed         = "failed"
	SyncDegraded       = "degraded"
)

// CreateDesiredStateInput is the input for creating a desired state.
type CreateDesiredStateInput struct {
	NodeID    string `json:"node_id"`
	StateJSON string `json:"state_json"`
	Reason    string `json:"reason"`
	CreatedBy string `json:"created_by"`
}
