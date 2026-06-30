package endpoint

import (
	"time"

	"aegis/internal/addr"
)

// Endpoint represents a network endpoint for a service.
// Address can be "host:port", "tcp://host:port", or "unix:///path/to/sock".
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
// For Unix sockets, returns the path and -1.
func (e *Endpoint) HostPort() (string, int) {
	a, err := addr.Parse(e.Address)
	if err != nil {
		return e.Address, 0
	}
	if a.IsUnix() {
		return a.Path, -1
	}
	return a.Host, a.Port
}

// Addr returns the parsed address of this endpoint.
func (e *Endpoint) Addr() *addr.Addr {
	a, err := addr.Parse(e.Address)
	if err != nil {
		// Fallback: treat as TCP with no port
		return &addr.Addr{Network: addr.NetTCP, Host: e.Address}
	}
	return a
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
	NodeID    string // v1.8F — which node this endpoint runs on
}
