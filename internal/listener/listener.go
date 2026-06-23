package listener

import (
	"fmt"
	"time"
)

// Listener represents a bind point owned by a provider.
type Listener struct {
	ID        string    `json:"id"`
	NodeID    string    `json:"node_id"`
	Provider  string    `json:"provider"`  // caddy_http | haproxy_edge_mux | haproxy_tcp
	Protocol  string    `json:"protocol"` // http | https | tcp | tls_mux
	BindIP    string    `json:"bind_ip"`
	Port      int       `json:"port"`
	Purpose   string    `json:"purpose"`  // public_http | public_tls_mux | internal_https | tcp_exposure
	Status    string    `json:"status"`   // planned | active | disabled | conflict
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EdgeMuxDefaults returns the standard EdgeMux listener configuration.
// HAProxy EdgeMux owns 0.0.0.0:443 (TLS SNI passthrough).
// Caddy owns 0.0.0.0:80 (HTTP) and 127.0.0.1:8443 (internal HTTPS).
func EdgeMuxDefaults() []Listener {
	now := time.Now()
	return []Listener{
		{ID: "listener_edge_http", Provider: "caddy_http", Protocol: "http", BindIP: "0.0.0.0", Port: 80, Purpose: "public_http", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "listener_edge_tls_mux", Provider: "haproxy_edge_mux", Protocol: "tls_mux", BindIP: "0.0.0.0", Port: 443, Purpose: "public_tls_mux", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "listener_edge_internal_https", Provider: "caddy_http", Protocol: "https", BindIP: "127.0.0.1", Port: 8443, Purpose: "internal_https", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
}

// DefaultListeners returns the legacy Caddy-only defaults (non-EdgeMux mode).
func DefaultListeners() []Listener {
	now := time.Now()
	return []Listener{
		{ID: "listener_default_http", Provider: "caddy_http", Protocol: "http", BindIP: "0.0.0.0", Port: 80, Purpose: "public_http", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "listener_default_https", Provider: "caddy_http", Protocol: "https", BindIP: "0.0.0.0", Port: 443, Purpose: "public_http", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
}

// ConflictError represents a listener conflict.
type ConflictError struct {
	ExistingListener Listener
	RequestedBind    string
	RequestedPort    int
}

func (e *ConflictError) Error() string {
	if e.ExistingListener.Port == 443 && e.ExistingListener.Provider == "haproxy_edge_mux" {
		return fmt.Sprintf("LISTENER_CONFLICT: port 443 is owned by haproxy_edge_mux. Register a TLS SNI edge rule instead, or choose another port.")
	}
	return fmt.Sprintf("LISTENER_CONFLICT: %s:%d already bound by provider %s (%s). Enable edge_mux mode or choose another port.",
		e.ExistingListener.BindIP, e.ExistingListener.Port, e.ExistingListener.Provider, e.ExistingListener.Protocol)
}
