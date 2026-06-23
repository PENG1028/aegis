package route

import "time"

// Route represents a routing rule that maps a domain to a backend service.
type Route struct {
	ID                  string    `json:"id"`
	Domain              string    `json:"domain"`
	ServiceID           string    `json:"service_id"`
	TLSEnabled           bool      `json:"tls_enabled"`
	Status              string    `json:"status"` // active | disabled
	MaintenanceEnabled  bool      `json:"maintenance_enabled"`
	MaintenanceMessage  string    `json:"maintenance_message"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// CreateRouteInput is the input for creating a route.
type CreateRouteInput struct {
	Domain    string
	ServiceID string
}

// SwitchRouteInput is the input for switching a route's service.
type SwitchRouteInput struct {
	ServiceID string
}
