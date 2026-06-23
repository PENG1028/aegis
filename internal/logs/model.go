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
