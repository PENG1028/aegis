package routingpolicy

// Mode constants for gateway policy.
const (
	ModeAuto     = "auto"
	ModeFixed    = "fixed"
	ModeMulti    = "multi"
	ModeDisabled = "disabled"
)

// TLSMode constants.
const (
	TLSModeHTTPOnly         = "http_only"
	TLSModeTerminateLocal   = "terminate_local"
	TLSModePassthroughDefer = "passthrough_deferred"
)

// ServiceGatewayPolicy represents a gateway policy bound to a service.
type ServiceGatewayPolicy struct {
	PolicyID              string `json:"policy_id"`
	ServiceID             string `json:"service_id"`
	Mode                  string `json:"mode"`
	PrimaryGatewayID      string `json:"primary_gateway_id,omitempty"`
	FallbackGatewayIDs    []string `json:"fallback_gateway_ids,omitempty"`
	AllowLocal            bool   `json:"allow_local"`
	AllowPrivate          bool   `json:"allow_private"`
	AllowPublic           bool   `json:"allow_public"`
	RequireGatewayLink    bool   `json:"require_gateway_link"`
	RequireRelay          bool   `json:"require_relay"`
	PreserveHost          bool   `json:"preserve_host"`
	TLSMode               string `json:"tls_mode"`
	Priority              int    `json:"priority"`
	Enabled               bool   `json:"enabled"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

// RouteGatewayPolicy represents a gateway policy bound to a route.
type RouteGatewayPolicy struct {
	PolicyID              string `json:"policy_id"`
	RouteID               string `json:"route_id"`
	Mode                  string `json:"mode"`
	PrimaryGatewayID      string `json:"primary_gateway_id,omitempty"`
	FallbackGatewayIDs    []string `json:"fallback_gateway_ids,omitempty"`
	AllowLocal            bool   `json:"allow_local"`
	AllowPrivate          bool   `json:"allow_private"`
	AllowPublic           bool   `json:"allow_public"`
	RequireGatewayLink    bool   `json:"require_gateway_link"`
	RequireRelay          bool   `json:"require_relay"`
	PreserveHost          bool   `json:"preserve_host"`
	TLSMode               string `json:"tls_mode"`
	Priority              int    `json:"priority"`
	Enabled               bool   `json:"enabled"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

// PolicyInput is used for creating or updating a policy.
type PolicyInput struct {
	ServiceID string `json:"service_id,omitempty"`
	RouteID   string `json:"route_id,omitempty"`
	Mode      string `json:"mode"`
	PrimaryGatewayID      string   `json:"primary_gateway_id,omitempty"`
	FallbackGatewayIDs    []string `json:"fallback_gateway_ids,omitempty"`
	AllowLocal            *bool    `json:"allow_local,omitempty"`
	AllowPrivate          *bool    `json:"allow_private,omitempty"`
	AllowPublic           *bool    `json:"allow_public,omitempty"`
	RequireGatewayLink    *bool    `json:"require_gateway_link,omitempty"`
	RequireRelay          *bool    `json:"require_relay,omitempty"`
	PreserveHost          *bool    `json:"preserve_host,omitempty"`
	TLSMode               string   `json:"tls_mode,omitempty"`
	Priority              int      `json:"priority,omitempty"`
	Enabled               *bool    `json:"enabled,omitempty"`
}

// ResolvedPolicy is the effective policy after precedence resolution.
type ResolvedPolicy struct {
	Source              string `json:"source"` // "route", "service", "default"
	Mode                string `json:"mode"`
	PrimaryGatewayID    string `json:"primary_gateway_id,omitempty"`
	FallbackGatewayIDs  []string `json:"fallback_gateway_ids,omitempty"`
	AllowLocal          bool   `json:"allow_local"`
	AllowPrivate        bool   `json:"allow_private"`
	AllowPublic         bool   `json:"allow_public"`
	RequireGatewayLink  bool   `json:"require_gateway_link"`
	RequireRelay        bool   `json:"require_relay"`
	PreserveHost        bool   `json:"preserve_host"`
	TLSMode             string `json:"tls_mode"`
}

// DefaultPolicy returns the system default gateway policy.
func DefaultPolicy() ResolvedPolicy {
	return ResolvedPolicy{
		Source:             "default",
		Mode:               ModeAuto,
		AllowLocal:         true,
		AllowPrivate:       true,
		AllowPublic:        false,
		RequireGatewayLink: true,
		RequireRelay:       true,
		PreserveHost:       true,
		TLSMode:            TLSModeHTTPOnly,
	}
}

// ValidModes returns all valid policy modes.
func ValidModes() []string {
	return []string{ModeAuto, ModeFixed, ModeMulti, ModeDisabled}
}

// ValidTLSModes returns all valid TLS modes.
func ValidTLSModes() []string {
	return []string{TLSModeHTTPOnly, TLSModeTerminateLocal, TLSModePassthroughDefer}
}

// IsValidMode returns true if the mode is valid.
func IsValidMode(mode string) bool {
	for _, m := range ValidModes() {
		if m == mode {
			return true
		}
	}
	return false
}

// IsValidTLSMode returns true if the TLS mode is valid.
func IsValidTLSMode(tlsMode string) bool {
	for _, m := range ValidTLSModes() {
		if m == tlsMode {
			return true
		}
	}
	return false
}
