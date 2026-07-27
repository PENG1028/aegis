package topology

import "time"

// Edge status constants.
const (
	StatusUnknown      = "unknown"
	StatusVerified     = "verified"
	StatusMissingLink  = "missing_link"
	StatusUnreachable  = "unreachable"
	StatusDegraded     = "degraded"
)

// TopologyEdge represents a connectivity edge between two nodes.
type TopologyEdge struct {
	ID                string    `json:"id"`
	FromNodeID        string    `json:"from_node_id"`
	ToNodeID          string    `json:"to_node_id"`
	PrivateReachable  bool      `json:"private_reachable"`
	PublicReachable   bool      `json:"public_reachable"`
	PreferredGatewayID string   `json:"preferred_gateway_id,omitempty"`
	GatewayLinkID     string    `json:"gateway_link_id,omitempty"`
	Status            string    `json:"status"`
	LastVerifiedAt    time.Time `json:"last_verified_at,omitempty"`
	LastError         string    `json:"last_error,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateEdgeInput is the input for creating/updating a topology edge.
type CreateEdgeInput struct {
	FromNodeID        string `json:"from_node_id"`
	ToNodeID          string `json:"to_node_id"`
	PrivateReachable  *bool  `json:"private_reachable,omitempty"`
	PublicReachable   *bool  `json:"public_reachable,omitempty"`
	PreferredGatewayID string `json:"preferred_gateway_id,omitempty"`
	GatewayLinkID     string `json:"gateway_link_id,omitempty"`
}

// PathResult is the result of a path query.
type PathResult struct {
	FromNodeID        string `json:"from_node_id"`
	ToNodeID          string `json:"to_node_id"`
	PrivateReachable  bool   `json:"private_reachable"`
	PublicReachable   bool   `json:"public_reachable"`
	PreferredGatewayID string `json:"preferred_gateway_id,omitempty"`
	GatewayLinkID     string `json:"gateway_link_id,omitempty"`
	Status            string `json:"status"`
}
