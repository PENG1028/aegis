package route

import (
	"fmt"
	"strings"
	"time"
)

// Route represents a routing rule that maps a domain to a backend service.
type Route struct {
	ID                 string    `json:"id"`
	Domain             string    `json:"domain"`
	PathPrefix         string    `json:"path_prefix"`
	StripPrefix        bool      `json:"strip_prefix"`
	ServiceID          string    `json:"service_id"`
	TLSEnabled          bool      `json:"tls_enabled"`
	Status             string    `json:"status"` // active | disabled
	MaintenanceEnabled bool      `json:"maintenance_enabled"`
	MaintenanceMessage string    `json:"maintenance_message"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateRouteInput is the input for creating a route.
type CreateRouteInput struct {
	Domain      string
	PathPrefix  string
	StripPrefix bool
	ServiceID   string
}

// SwitchRouteInput is the input for switching a route's service.
type SwitchRouteInput struct {
	ServiceID string
}

// ValidatePathPrefix checks path_prefix validity.
func ValidatePathPrefix(path string) error {
	if path == "" {
		return nil // empty is valid (domain-only route)
	}
	if !strings.HasPrefix(path, "/") {
		return fmt.Errorf("path_prefix must start with /")
	}
	if path == "*" {
		return fmt.Errorf("path_prefix cannot be '*'")
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("path_prefix cannot contain '..'")
	}
	if strings.Contains(path, " ") {
		return fmt.Errorf("path_prefix cannot contain spaces")
	}
	return nil
}

// PathDepth returns the depth of a path (used for sorting).
func PathDepth(path string) int {
	if path == "" {
		return 0
	}
	return len(strings.Split(strings.Trim(path, "/"), "/"))
}
