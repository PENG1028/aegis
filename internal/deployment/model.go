package deployment

import "time"

// Rollout strategy constants.
const (
	StrategyAll    = "all"
	StrategyCanary = "canary"
	StrategyStaged = "staged"
)

// Status constants.
const (
	StatusPending    = "pending"
	StatusRunning    = "running"
	StatusSuccess    = "success"
	StatusFailed     = "failed"
	StatusRolledBack = "rolled_back"
)

// Deployment represents a versioned deployment of a service to target nodes.
type Deployment struct {
	ID              string    `json:"id"`
	Version         string    `json:"version"`
	ServiceID       string    `json:"service_id"`
	TargetNodes     []string  `json:"target_nodes"`
	RolloutStrategy string    `json:"rollout_strategy"` // all | canary | staged
	Status          string    `json:"status"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DeploymentInstance tracks the deployment state on a single node.
type DeploymentInstance struct {
	ID                 string    `json:"id"`
	DeploymentID       string    `json:"deployment_id"`
	NodeID             string    `json:"node_id"`
	Status             string    `json:"status"`
	LastAppliedVersion string    `json:"last_applied_version"`
	AppliedAt          time.Time `json:"applied_at"`
	ErrorMessage       string    `json:"error_message"`
	CreatedAt          time.Time `json:"created_at"`
}
