// Package provider — data types for the 3-dimension gateway architecture.
//
// This file defines all value objects that flow across dimension boundaries.
// They carry NO behavior beyond simple field accessors — only data and JSON tags.
//
// ## Data types (in declaration order)
//
//   PortBinding        — display-only port allocation (authoritative source: RuntimeMode)
//   ProviderState      — identity + health + capabilities snapshot (dim 3 writes, dim 1&2 read)
//   Plan               — what a Provider should render (dim 2 produces, dim 1 consumes)
//   ForwardTarget      — transparent proxy forwarding target
//   ListenerSpec       — port binding request
//   RouteSpec          — 5-dimension traffic routing rule
//   MatchSpec          — traffic matching criteria
//   UpstreamSpec       — backend target
//   ConfigFile         — rendered configuration output
//   GatewayType        — 5 fixed gateway types (HTTP/TLS-SNI/TCP/UDP/Transparent)
//   GatewayTypeDef     — static properties of a gateway type
//   MatchKey           — gateway traffic matching dimension
//   ForwardKind        — forwarding transport type
//   RoutingGranularity — route density per port
//   DepStrength        — dependency edge classification (hard/soft)
//   DepNode            — dependency graph node
//   DependencyEdge     — directed dependency between two entities
//
// ## Import rules
//
// This file imports NOTHING outside the provider package. It is safe to import
// from topology, lifecycle, transparent, and main.go.

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

// Issue describes why a provider cannot start or has a problem.
// Provider-agnostic — codes are generic, messages come from the provider itself.
type Issue struct {
	Code    string `json:"code"`    // "port_conflict" | "config_invalid" | "binary_missing" | "service_error"
	Message string `json:"message"` // human-readable
	Detail  string `json:"detail"`  // raw error from systemctl / validator
}

// ProviderState is a point-in-time snapshot of a Provider's identity, health,
// and capabilities. Dimension 3 (lifecycle) updates it after install/start/stop
// operations. Dimensions 1 and 2 read it to make rendering and planning decisions.
type ProviderState struct {
	// Identity
	ID          string      `json:"id"`           // machine-readable: "caddy", "haproxy", "nginx"
	Name        string      `json:"name"`         // human-readable: "Caddy HTTP", "HAProxy"
	GatewayType GatewayType `json:"gateway_type"` // one of 5 fixed GatewayTypes

	// Status — derived from installed + running + diagnostic
	Status string `json:"status"` // "ready" | "degraded" | "unavailable"

	// Ready — true if the provider can be started right now (config valid, ports free)
	Ready  bool    `json:"ready"`
	Issues []Issue `json:"issues,omitempty"` // why not ready, or last start failure

	// Installation status (dimension 3 populates these)
	Installed  bool   `json:"installed"`
	Running    bool   `json:"running"`
	Version    string `json:"version"`
	BinaryPath string `json:"binary_path"`
	ConfigPath string `json:"config_path"`

	// Capabilities — what this provider CAN do (static declaration)
	Capabilities []Capability `json:"capabilities"`

	// Port bindings — what ports this provider currently owns (display-only)
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

	// TLS custom certificate (certstore ID). Empty = use provider's auto-cert mechanism.
	CertID   string `json:"cert_id,omitempty"`
	CertPath string `json:"cert_path,omitempty"` // resolved filesystem path to PEM cert (set by planner)
	KeyPath  string `json:"key_path,omitempty"`  // resolved filesystem path to PEM key (set by planner)

	// Operational flags
	MaintenanceEnabled bool              `json:"maintenance_enabled,omitempty"`
	MaintenanceMessage string            `json:"maintenance_message,omitempty"`
	ExtraHeaders       map[string]string `json:"extra_headers,omitempty"` // headers to inject (e.g. Gateway Link auth)
	StripPathPrefix    bool              `json:"strip_path_prefix,omitempty"`
	Priority           int               `json:"priority,omitempty"` // higher priority routes match first
}

const (
	// RoutePriorityDefault is used for normal user/business routes.
	RoutePriorityDefault = 0

	// RoutePriorityControlPlane is reserved for local node control-plane routes.
	// Providers must match these before business fallback routes on the same
	// listener so gateway-link or catch-all routes cannot capture node RPC.
	RoutePriorityControlPlane = 1000
)

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

// ============================================================================
// GatewayType — 5 fixed types, closed set
// ============================================================================

// GatewayType identifies the network-layer entry paradigm of a gateway.
// This is a CLOSED SET — there are exactly 5 ways traffic can enter a gateway
// on a TCP/IP network. Do not add new types without a protocol-level justification.
type GatewayType string

const (
	// TypeHTTPTerm — L7 HTTP termination gateway.
	// Terminates TLS, reads HTTP Host/Path headers, reverse-proxies to upstream.
	// Match key: domain + path + headers.  One port can serve unlimited domains.
	// Implemented by: Caddy, Nginx, Traefik, Envoy, Apache.
	TypeHTTPTerm GatewayType = "http_terminate"

	// TypeSNIPass — L5 TLS SNI passthrough gateway.
	// Reads the SNI field from TLS ClientHello (does NOT decrypt), forwards raw TCP
	// stream to a TLS-terminating backend. Match key: SNI hostname.
	// Implemented by: HAProxy (mode tcp), Envoy, Traefik.
	TypeSNIPass GatewayType = "sni_passthrough"

	// TypeTCPForward — L4 TCP port forwarding gateway.
	// Accepts TCP connections on a specific port and forwards the raw stream.
	// Match key: port number.  One port = one service (no domain routing possible).
	// Implemented by: Aegis TCP Manager, HAProxy (mode tcp), Nginx stream.
	TypeTCPForward GatewayType = "tcp_forward"

	// TypeUDPForward — L4 UDP datagram forwarding gateway.
	// Accepts UDP datagrams on a specific port and forwards them.
	// Match key: port number.  One port = one service. Session-managed.
	// Implemented by: Aegis UDP Manager.
	TypeUDPForward GatewayType = "udp_forward"

	// TypeTransparent — L3 transparent interception gateway.
	// Intercepts outbound TCP connections via iptables DNAT and redirects to a local
	// proxy. Match key: destination IP:port. Linux-only (requires SO_ORIGINAL_DST).
	// Implemented by: Aegis Transparent Manager (iptables).
	TypeTransparent GatewayType = "transparent"
)

// GatewayTypeDef describes the fixed properties of a gateway type.
// Each GatewayType has exactly one GatewayTypeDef — this is how the system knows
// what a gateway of a given type can and cannot do.

// MatchKey is what the gateway uses to match incoming traffic to a route.
type MatchKey string

const (
	MatchHostPath MatchKey = "host_path" // domain + path + headers (HTTP)
	MatchSNI      MatchKey = "sni"       // TLS SNI hostname
	MatchPort     MatchKey = "port"      // TCP/UDP port number
	MatchDestAddr MatchKey = "dest_addr" // destination IP:port (transparent proxy)
)

// ForwardKind is what the gateway forwards to the target.
type ForwardKind string

const (
	ForwardHTTPProxy   ForwardKind = "http_proxy"   // HTTP reverse proxy
	ForwardTCPStream   ForwardKind = "tcp_stream"   // raw TCP stream
	ForwardUDPDatagram ForwardKind = "udp_datagram" // raw UDP datagrams
)

// RoutingGranularity describes route density per port.
type RoutingGranularity string

const (
	RouteByDomainPath RoutingGranularity = "domain+path" // unlimited routes/port
	RouteByDomain     RoutingGranularity = "domain"      // one per SNI hostname
	RouteByPort       RoutingGranularity = "port"        // one route per port
)

// GatewayTypeDefs maps each GatewayType to its fixed definition.
// This is the single source of truth for gateway type capabilities.

// ============================================================================
// Layer 3: Dependency model (hard vs soft)
// ============================================================================

// DepStrength classifies a dependency edge.
type DepStrength string

const (
	// DepHard — if this dependency fails, the dependent entity is UNAVAILABLE.
	// e.g. Service → Endpoint: no endpoint means the service cannot serve traffic.
	DepHard DepStrength = "hard"

	// DepSoft — if this dependency fails, the dependent entity is DEGRADED but not dead.
	// e.g. External HTTPS → HAProxy SNI: HAProxy down → external HTTPS broken,
	// but HTTP on :80 and internal :8443 still work.
	DepSoft DepStrength = "soft"
)

// DepNode identifies a node in the dependency graph.
type DepNode struct {
	Type string `json:"type"` // "node" | "gateway" | "listener" | "service" | "endpoint" | "route"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// DependencyEdge represents a directed dependency between two entities.
// From depends on To. If To fails, From is affected according to Strength.
type DependencyEdge struct {
	From     DepNode     `json:"from"`
	To       DepNode     `json:"to"`
	Strength DepStrength `json:"strength"`
	Impact   string      `json:"impact"`            // human-readable: "HTTPS 外部入口不可用"
	Affects  []string    `json:"affects,omitempty"` // which traffic types are affected
}
