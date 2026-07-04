// Package provider — pure data types for the 3-dimension architecture.
//
// This file defines value objects that flow across dimension boundaries.
// They carry NO behavior — only data and JSON tags.
//
// Ownership rules:
//   - ProviderState is WRITTEN by dimension 3 (lifecycle) and READ by dimensions 1 & 2.
//   - Plan is PRODUCED by dimension 2 (topology planner) and CONSUMED by dimension 1 (Provider.Render).
//   - ConfigFile is PRODUCED by dimension 1 (Provider.Render) and CONSUMED by dimension 1 (Provider.Apply).
//   - ForwardTarget is PRODUCED by dimension 2 and CONSUMED by transparent.Manager.
//
// Import rules: this file imports NOTHING outside the provider package.
// It is safe to import from topology, lifecycle, transparent, and main.go.
package provider

// ============================================================================
// PortBinding — display-only port allocation for a provider
// ============================================================================

// PortBinding represents a port allocation on a node. This is display-only —
// the authoritative source of port assignments is RuntimeMode.
type PortBinding struct {
	Port     int    `json:"port"`
	Owner    string `json:"owner"`    // provider ID
	Protocol string `json:"protocol"` // "tcp" | "udp"
	Purpose  string `json:"purpose"`  // "http" | "https" | "tls_sni_mux" | "internal_https" | "tcp_exposure" | "udp_exposure"
	Status   string `json:"status"`   // "active" | "planned" | "conflict"
}

// ============================================================================
// ProviderState — the single shared runtime snapshot across all 3 dimensions
// ============================================================================

// ProviderState is a point-in-time snapshot of a Provider's identity, health,
// and capabilities. Dimension 3 (lifecycle) updates it after install/start/stop
// operations. Dimensions 1 and 2 read it to make rendering and planning decisions.
type ProviderState struct {
	// Identity
	ID          string      `json:"id"`          // machine-readable: "caddy", "haproxy", "nginx"
	Name        string      `json:"name"`        // human-readable: "Caddy HTTP", "HAProxy"
	GatewayType GatewayType `json:"gateway_type"` // one of 5 fixed GatewayTypes

	// Status — derived from installed + running + diagnostic
	Status string `json:"status"` // "ready" | "degraded" | "unavailable"

	// Installation status (dimension 3 populates these)
	Installed  bool   `json:"installed"`
	Running    bool   `json:"running"`
	Version    string `json:"version"`
	BinaryPath string `json:"binary_path"`
	ConfigPath string `json:"config_path"`

	// Capabilities — what this provider CAN do (static declaration)
	Capabilities []Capability `json:"capabilities"`

	// Port bindings — what ports this provider currently owns
	Ports []PortBinding `json:"ports"`

	// Diagnostic — nil if provider not detected, else full diagnostic result
	Diagnostic *ProviderDiagnostic `json:"diagnostic,omitempty"`
}

// Healthy returns true if the provider is installed, running, and has no
// diagnostic errors that would prevent it from serving traffic.
func (s ProviderState) Healthy() bool {
	if !s.Installed || !s.Running {
		return false
	}
	if s.Diagnostic != nil {
		if s.Diagnostic.LastErrorCode != "" {
			return false
		}
	}
	return true
}

// HasCapability returns true if this provider declares the given capability.
func (s ProviderState) HasCapability(c Capability) bool {
	for _, cap := range s.Capabilities {
		if cap == c {
			return true
		}
	}
	return false
}

// ============================================================================
// Plan — what a Provider should render (dimension 2 → dimension 1)
// ============================================================================

// Plan describes the complete set of listeners and routes a Provider should
// generate configuration for. It is the output of the topology Planner and the
// input to Provider.Render().
//
// A Plan is provider-specific: the Planner allocates listeners and routes to
// individual providers based on capability matching. Each provider gets its
// own Plan containing only the routes it is responsible for.
type Plan struct {
	// Listeners are the ports this provider must bind.
	// Multiple routes can share a single listener (e.g., :80 serves many domains).
	Listeners []ListenerSpec `json:"listeners"`

	// Routes are the traffic routing rules this provider must handle.
	Routes []RouteSpec `json:"routes"`

	// ForwardTarget is set when transparent proxy is active on this node.
	// It tells the transparent proxy manager where to forward intercepted traffic.
	// nil if transparent proxy is not in use.
	ForwardTarget *ForwardTarget `json:"forward_target,omitempty"`
}

// ForwardTarget tells the transparent proxy where to forward intercepted TCP connections.
// This is computed by the Planner based on which Provider has [route_host, upstream_tcp]
// capability — typically Caddy :80, but could be Nginx :8080 or any HTTP router.
type ForwardTarget struct {
	Host string `json:"host"` // "127.0.0.1"
	Port int    `json:"port"` // 80 (Caddy), 8443 (Caddy behind HAProxy), 8080 (Nginx)
}

// ============================================================================
// ListenerSpec — a port binding request
// ============================================================================

// ListenerSpec describes a port that a Provider must bind and listen on.
type ListenerSpec struct {
	Port     int    `json:"port"`     // port number
	Protocol string `json:"protocol"` // "tcp" | "udp"
	Purpose  string `json:"purpose"`  // "http" | "https" | "tls_sni_mux" | "internal_https" | "tcp_exposure" | "udp_exposure"
}

// ============================================================================
// RouteSpec — a single traffic routing rule (5-dimension intent model)
// ============================================================================

// RouteSpec describes a single traffic routing rule using the 5-dimension
// intent model: transport × tlsMode × appProtocol × match × upstream.
//
// Unlike the old proxy.RouteConfig (which was Caddy-centric), RouteSpec is
// middleware-agnostic. Each Provider's Render() translates it into the
// native config syntax.
type RouteSpec struct {
	// Transport layer
	Transport string `json:"transport"` // "tcp" | "udp"

	// TLS mode (session layer)
	TLSMode string `json:"tls_mode"` // "none" | "terminate" | "passthrough"

	// Match criteria (how to identify this traffic)
	Match MatchSpec `json:"match"`

	// Application protocol
	AppProtocol string `json:"app_protocol"`           // "http" | "grpc" | "websocket" | "raw"
	HTTPVersion string `json:"http_version,omitempty"` // "h1" | "h2" | "h3" (only when AppProtocol is http)

	// Upstream target
	Upstream UpstreamSpec `json:"upstream"`

	// Operational flags
	MaintenanceEnabled bool              `json:"maintenance_enabled,omitempty"`
	MaintenanceMessage string            `json:"maintenance_message,omitempty"`
	ExtraHeaders       map[string]string `json:"extra_headers,omitempty"` // headers to inject (e.g. Gateway Link auth)
	StripPathPrefix    bool              `json:"strip_path_prefix,omitempty"`
}

// ============================================================================
// MatchSpec — traffic matching criteria
// ============================================================================

// MatchSpec describes how to identify incoming traffic for a route.
// At least one field must be set. Fields are AND-ed together within a MatchSpec.
type MatchSpec struct {
	// L6: TLS SNI hostname (from ClientHello, no decryption needed)
	SNI string `json:"sni,omitempty"`

	// L7: HTTP Host header (requires TLS termination)
	Host string `json:"host,omitempty"`

	// L7: URL path prefix (requires TLS termination + HTTP parsing)
	Path string `json:"path,omitempty"`

	// L6: ALPN protocol negotiation values (e.g. ["h2", "http/1.1"])
	ALPN []string `json:"alpn,omitempty"`

	// L4: match by destination port number
	Port int `json:"port,omitempty"`

	// L3: match by source IP CIDR
	SrcIP string `json:"src_ip,omitempty"`
}

// IsEmpty returns true if no match criteria are set.
func (m MatchSpec) IsEmpty() bool {
	return m.SNI == "" && m.Host == "" && m.Path == "" &&
		len(m.ALPN) == 0 && m.Port == 0 && m.SrcIP == ""
}

// ============================================================================
// UpstreamSpec — where to forward matched traffic
// ============================================================================

// UpstreamSpec describes the target backend for forwarded traffic.
type UpstreamSpec struct {
	// Type is the transport protocol to the upstream.
	// "tcp" — raw TCP connection to host:port
	// "udp" — raw UDP datagrams to host:port
	// "unix" — Unix domain socket (target is a filesystem path)
	// "http" — HTTP reverse proxy (target is an http:// URL)
	Type string `json:"type"`

	// Target is the upstream address.
	// For "tcp" / "udp": "host:port" (e.g. "127.0.0.1:3000")
	// For "unix": "/run/app.sock" or "unix:///run/app.sock"
	// For "http": "http://host:port" or "https://host:port"
	Target string `json:"target"`
}

// ============================================================================
// ConfigFile — rendered configuration output (dimension 1)
// ============================================================================

// ConfigFile is a single rendered configuration file ready to be validated
// and written to disk. One Provider.Render() call may produce multiple files
// (e.g., HAProxy generates haproxy.cfg + haproxy_tcp.cfg).
type ConfigFile struct {
	// Path is the absolute filesystem path where this file should be written.
	Path string `json:"path"`

	// Content is the rendered configuration text.
	Content []byte `json:"content"`
}
