package endpoint

import (
	"net"
	"strconv"
	"time"
)

// Endpoint represents a network endpoint for a service.
type Endpoint struct {
	ID        string    `json:"id"`
	ServiceID string    `json:"service_id"`
	Type      string    `json:"type"` // local | private | public
	Address   string    `json:"address"`
	Enabled   bool      `json:"enabled"`
	NodeID    string    `json:"node_id,omitempty"` // v1.8B — which node this endpoint runs on
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HostPort returns the host and port parsed from Address.
// Address format is expected to be "host:port".
func (e *Endpoint) HostPort() (string, int) {
	h, pStr, err := net.SplitHostPort(e.Address)
	if err != nil {
		return e.Address, 0
	}
	p, _ := strconv.Atoi(pStr)
	return h, p
}

// Type priority order for resolution.
var typePriority = map[string]int{
	"local":   0,
	"private": 1,
	"public":  2,
}

// Priority returns the resolution priority (lower = tried first).
func (e *Endpoint) Priority() int {
	if p, ok := typePriority[e.Type]; ok {
		return p
	}
	return 99
}

// CreateEndpointInput is the input for creating an endpoint.
type CreateEndpointInput struct {
	ServiceID string
	Type      string
	Address   string
}
