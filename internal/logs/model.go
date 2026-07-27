package logs

import "time"

// OperationLog represents a recorded operation.
type OperationLog struct {
	ID         string    `json:"id"`
	Action     string    `json:"action"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Result     string    `json:"result"` // success | failed
	Message    string    `json:"message"`
	Actor      string    `json:"actor"` // cli | api | system
	CreatedAt  time.Time `json:"created_at"`
}

// ApplyLog records a single apply operation with step-level detail.
type ApplyLog struct {
	ID                  string    `json:"id"`
	OperationID         string    `json:"operation_id"`
	StateVersion        uint64    `json:"state_version"`
	ConfigHashBefore    string    `json:"config_hash_before"`
	ConfigHashAfter     string    `json:"config_hash_after"`
	Provider            string    `json:"provider"`
	ValidateStatus      string    `json:"validate_status"`
	ReloadStatus        string    `json:"reload_status"`
	RuntimeVerifyStatus string    `json:"runtime_verify_status"`
	Stderr              string    `json:"stderr"`
	StepLog             string    `json:"step_log"` // JSON array of steps
	CreatedAt           time.Time `json:"created_at"`
}

// ApplyStep represents a single step in the apply process.
type ApplyStep struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // running | success | failed | skipped
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// AuditLog records security-relevant events.
type AuditLog struct {
	ID         string    `json:"id"`
	ActorType  string    `json:"actor_type"` // admin | service_key | system
	ActorID    string    `json:"actor_id"`
	EventType  string    `json:"event_type"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	Result     string    `json:"result"`
	ErrorCode  string    `json:"error_code"`
	CreatedAt  time.Time `json:"created_at"`
}

// NodeEvent records cluster node lifecycle events.
type NodeEvent struct {
	ID           string    `json:"id"`
	NodeID       string    `json:"node_id"`
	EventType    string    `json:"event_type"`
	StateVersion uint64    `json:"state_version"`
	Severity     string    `json:"severity"` // info | warning | error | critical
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

// Node event type constants.
const (
	NodeEventLeaderElected      = "leader_elected"
	NodeEventDriftDetected      = "drift_detected"
	NodeEventReconcileStarted   = "reconcile_started"
	NodeEventReconcileFinished  = "reconcile_finished"
	NodeEventNodeStale          = "node_stale"
	NodeEventACKTimeout         = "ack_timeout"
	NodeEventSplitBrain         = "split_brain"
)
