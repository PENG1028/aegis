package gateway

import "time"

// Status constants for gateway resources.
const (
	StatusActive       = "active"
	StatusDisabled     = "disabled"
	StatusProvisioning = "provisioning"
	StatusFailed       = "failed"
)

// GatewayDomain represents a provider-agnostic gateway domain binding.
type GatewayDomain struct {
	ID          string    `json:"id"`
	Domain      string    `json:"domain"`
	NodeID      string    `json:"node_id"`
	TLSEnabled  bool      `json:"tls_enabled"`
	TLSProvider string    `json:"tls_provider"` // "" | "caddy" | "haproxy"
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// GatewayRoute represents a path-based route within a gateway domain.
type GatewayRoute struct {
	ID            string    `json:"id"`
	DomainID      string    `json:"domain_id"`
	Path          string    `json:"path"`
	TargetService string    `json:"target_service"`
	TargetPort    int       `json:"target_port"`
	Protocol      string    `json:"protocol"` // http | https | tcp
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GatewayListener represents a port binding on a specific node.
type GatewayListener struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	Port       int       `json:"port"`
	TLSEnabled bool      `json:"tls_enabled"`
	Protocol   string    `json:"protocol"` // http | https | tcp | tls_mux
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
