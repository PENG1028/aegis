package upgrade

import "time"

// Step represents a single step in an upgrade.
type Step struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // running | success | failed | skipped
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Step constants for tracing.
const (
	StepSnapshot     = "snapshot"
	StepValidate     = "validation"
	StepHealthGate   = "health_gate"
	StepPushState    = "push_state"
	StepACKWait      = "ack_wait"
	StepSwitchLeader = "switch_leader"
	StepVerify       = "verify"
	StepRollback     = "rollback"
	StepCommit       = "commit"
)

// Tracker records step-level progress of an upgrade session.
type Tracker struct {
	session *Session
}

// NewTracker creates a step tracker for a session.
func NewTracker(session *Session) *Tracker {
	return &Tracker{session: session}
}

// Record records a step with status.
func (t *Tracker) Record(name, status, message string) {
	t.session.AddStep(name, status, message)
}

// RecordSuccess records a successful step.
func (t *Tracker) RecordSuccess(name, message string) {
	t.Record(name, "success", message)
}

// RecordFailed records a failed step.
func (t *Tracker) RecordFailed(name string, err error) {
	t.Record(name, "failed", err.Error())
}

// RecordSkipped records a skipped step.
func (t *Tracker) RecordSkipped(name, reason string) {
	t.Record(name, "skipped", reason)
}
