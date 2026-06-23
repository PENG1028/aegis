package listener

import (
	"fmt"
	"time"
)

// Listener represents a bind point owned by a provider.
type Listener struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider"`  // caddy_http | haproxy_tcp
	Protocol  string    `json:"protocol"` // http | https | tcp | udp | tls_mux
	BindIP    string    `json:"bind_ip"`
	Port      int       `json:"port"`
	Status    string    `json:"status"`   // active | reserved
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DefaultListeners returns the standard listeners that are always reserved.
func DefaultListeners() []Listener {
	now := time.Now()
	return []Listener{
		{ID: "listener_default_http", Provider: "caddy_http", Protocol: "http", BindIP: "0.0.0.0", Port: 80, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "listener_default_https", Provider: "caddy_http", Protocol: "https", BindIP: "0.0.0.0", Port: 443, Status: "active", CreatedAt: now, UpdatedAt: now},
	}
}

// ConflictError represents a listener conflict.
type ConflictError struct {
	ExistingListener Listener
	RequestedBind    string
	RequestedPort    int
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("LISTENER_CONFLICT: %s:%d already bound by provider %s (%s). Enable edge_mux mode or choose another port.",
		e.ExistingListener.BindIP, e.ExistingListener.Port, e.ExistingListener.Provider, e.ExistingListener.Protocol)
}
