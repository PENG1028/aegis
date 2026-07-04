package gateway

import (
	"net"
	"sync"
	"time"
)

// GatewayStatusInfo holds the current gateway status.
type GatewayStatusInfo struct {
	Enabled   bool   `json:"enabled"`
	BindAddr  string `json:"bind_addr"`
	Port      int    `json:"port"`
	Status    string `json:"status"`
	LastError string `json:"last_error,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

// GatewayStatus manages gateway status updates.
type GatewayStatus struct {
	mu   sync.RWMutex
	info GatewayStatusInfo
}

// NewGatewayStatus creates a new gateway status tracker.
func NewGatewayStatus() *GatewayStatus {
	return &GatewayStatus{
		info: GatewayStatusInfo{
			Status:    "unknown",
			BindAddr:  "127.0.0.1",
			Port:      18080,
			Enabled:   true,
		},
	}
}

// NewGatewayStatusFromConfig creates a new gateway status tracker populated from config.
func NewGatewayStatusFromConfig(cfg *Config) *GatewayStatus {
	return &GatewayStatus{
		info: GatewayStatusInfo{
			Enabled:  cfg.Enabled,
			BindAddr: cfg.ListenAddr(),
			Port:     cfg.ListenPort(),
			Status:   "unknown",
		},
	}
}

// SetStatus updates the gateway status.
func (s *GatewayStatus) SetStatus(status, lastError string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.info.Status = status
	s.info.LastError = lastError
	s.info.UpdatedAt = time.Now().Format(time.RFC3339)
}

// Get returns the current status.
func (s *GatewayStatus) Get() GatewayStatusInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

// listen creates a TCP listener for the gateway.
func listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
