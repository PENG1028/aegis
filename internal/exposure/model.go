package exposure

import "time"

// Type constants.
const (
	TypeHTTP    = "http"
	TypeTCP     = "tcp"
	TypeUDP     = "udp"
	TypeTunnel  = "tunnel"
	TypeInternal = "internal"
)

// Mode constants.
const (
	ModePublic  = "public"
	ModePrivate = "private"
	ModeInternal = "internal"
)

// Status constants.
const (
	StatusPending        = "pending"
	StatusActive         = "active"
	StatusActiveRecorded = "active_recorded"
	StatusDisabled       = "disabled"
	StatusFailed         = "failed"
)

// GeneratesConfig returns true if this exposure type generates real proxy config.
func GeneratesConfig(exposureType string) bool {
	return exposureType == TypeHTTP || exposureType == TypeTCP
}

// Exposure represents an external service exposure request.
// HTTP exposures generate Caddy routes; TCP exposures start listeners.
type Exposure struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	Type       string    `json:"type"`   // http | tcp | udp | tunnel | internal
	Mode       string    `json:"mode"`   // public | private | internal
	Host       string    `json:"host"`       // entry host
	Port       int       `json:"port"`       // entry port
	Path       string    `json:"path"`       // entry path (HTTP only)
	TargetHost string    `json:"target_host"` // upstream host
	TargetPort int       `json:"target_port"` // upstream port
	ServiceID  string    `json:"service_id"`
	NodeID     string    `json:"node_id"`
	OwnerRef   string    `json:"owner_ref"`
	TargetRef  string    `json:"target_ref"`
	AllowPublicTCP bool   `json:"allow_public_tcp"` // admin override for public TCP bind
	Provider    string    `json:"provider"`     // caddy_http | haproxy_tcp
	ListenerID  string    `json:"listener_id"`  // bound listener
	Status     string    `json:"status"` // pending | active | active_recorded | disabled | failed | pending_adapter | pending_provider
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateExposureInput is the input for creating an exposure.
type CreateExposureInput struct {
	Type           string `json:"type"`
	Mode           string `json:"mode"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Path           string `json:"path"`
	TargetHost     string `json:"target_host"`
	TargetPort     int    `json:"target_port"`
	ServiceID      string `json:"service_id"`
	NodeID         string `json:"node_id"`
	OwnerRef       string `json:"owner_ref"`
	TargetRef      string `json:"target_ref"`
	AllowPublicTCP bool   `json:"allow_public_tcp"`
}

// UpdateExposureInput is the input for updating an exposure.
type UpdateExposureInput struct {
	Host   *string `json:"host"`
	Port   *int    `json:"port"`
	Path   *string `json:"path"`
	Status *string `json:"status"`
	Message *string `json:"message"`
}

// Stats holds exposure statistics grouped by type and status.
type Stats struct {
	Total    int            `json:"total"`
	ByType   map[string]int `json:"by_type"`
	ByStatus map[string]int `json:"by_status"`
	HTTPActive    int `json:"http_active"`
	NonHTTPRecorded int `json:"non_http_recorded"`
}
