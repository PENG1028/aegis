// Package flowbridge — managed FlowBridge data-plane instances.
//
// A FlowBridge instance is a remote HTTP data-plane process that Aegis routes
// point at instead of a plain service endpoint. Aegis only terminates TLS and
// forwards to the instance's data-plane listener; routing/switch/health inside
// FlowBridge is owned by FlowBridge itself.
//
// Instance health is probed against the FlowBridge control-plane /health
// endpoint (deliberately unauthenticated there) — never against business
// targets behind FlowBridge.
package flowbridge

import (
	"fmt"
	"net"
	"time"
)

// Health status values for a FlowBridge instance.
const (
	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

// Instance is a managed FlowBridge data-plane process.
type Instance struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	MachineIP         string    `json:"machine_ip"`
	DataPlanePort     int       `json:"data_plane_port"`
	ControlAddress    string    `json:"control_address"` // control-plane host:port, e.g. "127.0.0.1:9090"
	Enabled           bool      `json:"enabled"`
	LastHealthStatus  string    `json:"last_health_status"` // unknown | healthy | unhealthy
	LastHealthLatency int64     `json:"last_health_latency_ms"`
	LastHealthMessage string    `json:"last_health_message"`
	LastCheckedAt     time.Time `json:"last_checked_at"`
	SpaceID           string    `json:"space_id"`
	OwnerType         string    `json:"owner_type"` // space | admin
	OwnerID           string    `json:"owner_id"`
	CreatedByTokenID  string    `json:"created_by_token_id"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateInstanceInput is the input for creating an instance.
type CreateInstanceInput struct {
	Name           string `json:"name"`
	MachineIP      string `json:"machine_ip"`
	DataPlanePort  int    `json:"data_plane_port"`
	ControlAddress string `json:"control_address"`
}

// UpdateInstanceInput is the input for updating an instance.
type UpdateInstanceInput struct {
	Name           *string `json:"name,omitempty"`
	MachineIP      *string `json:"machine_ip,omitempty"`
	DataPlanePort  *int    `json:"data_plane_port,omitempty"`
	ControlAddress *string `json:"control_address,omitempty"`
	Enabled        *bool   `json:"enabled,omitempty"`
}

// Validate checks instance identity fields.
func (in CreateInstanceInput) Validate() error {
	if in.Name == "" {
		return fmt.Errorf("name is required")
	}
	if len(in.Name) > 100 {
		return fmt.Errorf("name must be at most 100 characters")
	}
	if net.ParseIP(in.MachineIP) == nil {
		return fmt.Errorf("machine_ip must be a valid IP address")
	}
	if in.DataPlanePort <= 0 || in.DataPlanePort > 65535 {
		return fmt.Errorf("data_plane_port must be in range 1-65535")
	}
	if in.ControlAddress == "" {
		return fmt.Errorf("control_address is required")
	}
	if _, _, err := net.SplitHostPort(in.ControlAddress); err != nil {
		return fmt.Errorf("control_address must be host:port: %v", err)
	}
	return nil
}

// Validate checks the fields present in a partial update.
func (in UpdateInstanceInput) Validate() error {
	if in.Name != nil {
		if *in.Name == "" {
			return fmt.Errorf("name is required")
		}
		if len(*in.Name) > 100 {
			return fmt.Errorf("name must be at most 100 characters")
		}
	}
	if in.MachineIP != nil && net.ParseIP(*in.MachineIP) == nil {
		return fmt.Errorf("machine_ip must be a valid IP address")
	}
	if in.DataPlanePort != nil && (*in.DataPlanePort <= 0 || *in.DataPlanePort > 65535) {
		return fmt.Errorf("data_plane_port must be in range 1-65535")
	}
	if in.ControlAddress != nil {
		if *in.ControlAddress == "" {
			return fmt.Errorf("control_address is required")
		}
		if _, _, err := net.SplitHostPort(*in.ControlAddress); err != nil {
			return fmt.Errorf("control_address must be host:port: %v", err)
		}
	}
	return nil
}
