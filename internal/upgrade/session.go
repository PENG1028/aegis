package upgrade

import (
	"fmt"
	"time"

	"aegis/internal/id"
)

// Status constants.
const (
	StatusRunning   = "running"
	StatusSuccess   = "success"
	StatusFailed    = "failed"
	StatusRolledBack = "rolled_back"
)

// Session tracks a single upgrade lifecycle.
type Session struct {
	ID                string    `json:"id"`
	FromVersion       string    `json:"from_version"`
	ToVersion         string    `json:"to_version"`
	StateVersionStart uint64    `json:"state_version_start"`
	StateVersionEnd   uint64    `json:"state_version_end"`
	Status            string    `json:"status"`
	ErrorMessage      string    `json:"error_message,omitempty"`
	StartTime         time.Time `json:"start_time"`
	EndTime           time.Time `json:"end_time,omitempty"`
	Steps             []Step    `json:"steps"`
}

// NewSession creates a new upgrade session.
func NewSession(fromVer, toVer string, stateVerStart uint64) *Session {
	return &Session{
		ID:                id.New("upgrade"),
		FromVersion:       fromVer,
		ToVersion:         toVer,
		StateVersionStart: stateVerStart,
		Status:            StatusRunning,
		StartTime:         time.Now(),
	}
}

// AddStep appends a step trace to the session.
func (s *Session) AddStep(name, status, message string) {
	s.Steps = append(s.Steps, Step{
		Name:      name,
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
	})
}

// MarkSuccess marks the session as successful.
func (s *Session) MarkSuccess(endVersion uint64) {
	s.Status = StatusSuccess
	s.StateVersionEnd = endVersion
	s.EndTime = time.Now()
	s.AddStep("commit", "success", fmt.Sprintf("upgraded to v%d", endVersion))
}

// MarkFailed marks the session as failed.
func (s *Session) MarkFailed(err error) {
	s.Status = StatusFailed
	s.ErrorMessage = err.Error()
	s.EndTime = time.Now()
	s.AddStep("fail", "failed", err.Error())
}

// MarkRolledBack marks the session as rolled back.
func (s *Session) MarkRolledBack(reason string) {
	s.Status = StatusRolledBack
	s.ErrorMessage = reason
	s.EndTime = time.Now()
	s.AddStep("rollback", "rolled_back", reason)
}
