// Package provider implements the Provider abstraction layer for Aegis.
//
// # ARCHITECTURE INDEX — Read this first
//
// This file defines the type system that underpins all gateway/provider logic.
// When adding a new provider or gateway type, start here.
//
// ## Concept hierarchy (top to bottom):
//
//   Layer 1 — GatewayType (5 fixed types, protocol-determined, closed set)
//     http_terminate | sni_passthrough | tcp_forward | udp_forward | transparent
//     Defined in: GatewayType, GatewayTypeDef
//
//   Layer 2 — Protocol (open set, what protocol the forwarded traffic uses)
//     http/1.1 | http/2 | http/3 | websocket | grpc | sse | tcp | udp | unix
//     Defined in: Protocol, ProtocolDef
//
//   Layer 3 — Provider (tool that implements gateway functionality)
//     caddy | haproxy_edge_mux | haproxy_tcp | aegis_tcp | aegis_udp
//     Interface defined in: provider.go → Provider
//     Registry defined in: registry.go
//
//   Layer 4 — ProviderCapabilities (what a specific provider CAN do)
//     Computed from: GatewayType × Protocol support × installed tools
//     Defined in: ProviderCapabilities
//
//   Layer 5 — UIHints (how the frontend SHOULD render this provider)
//     Custom panels, metrics, operations per provider
//     Defined in: ProviderUIHints
//
// ## Key relationships:
//
//   Provider  →  implements GatewayType  (a Caddy instance IS an http_terminate gateway)
//   Provider  →  supports Protocol set   (Caddy supports http, h2, ws, grpc; HAProxy SNI supports tcp only)
//   Gateway   →  has one GatewayType     (stored in gateways.type column)
//   Route     →  declares one Protocol   (stored in route options)
//   Listener  →  binds one port          (exclusive, one port = one owner)
//
// ## Port allocation:
//
//   Legacy mode (no HAProxy):  Caddy owns 80 + 443
//   EdgeMux mode (HAProxy):    Caddy owns 80 + 8443, HAProxy owns 443
//   See: port_policy.go for the dynamic allocation logic.
//
// ## Adding a new provider (e.g. Nginx):
//
//   1. Add a new Provider implementation in internal/provider/nginx/
//   2. Register in registry.go init()
//   3. Add frontend adapter in ui/src/adapters/nginx/
//   4. No other code changes needed — GatewayType and Protocol are fixed.

package provider

import "time"

// ============================================================================
// Layer 1: GatewayType — 5 fixed types, closed set
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
type GatewayTypeDef struct {
	Type        GatewayType
	Label       string // Human-readable label for UI

	// MatchKey defines what the gateway uses to distinguish traffic.
	MatchKey MatchKey // "host_path" | "sni" | "port" | "dest_addr"

	// ForwardKind defines what the gateway forwards.
	ForwardKind ForwardKind // "http_proxy" | "tcp_stream" | "udp_datagram"

	// TerminatesTLS is true if the gateway decrypts TLS and can read HTTP content.
	TerminatesTLS bool

	// UnderstandsHTTP is true if the gateway can read HTTP headers (Host, Path, etc.).
	// Only true for TypeHTTPTerm. SNI passthrough sees TLS, not HTTP.
	UnderstandsHTTP bool

	// Granularity describes how many routes can share one port.
	// domain+path: unlimited (Caddy on :80)
	// domain: one per SNI hostname (HAProxy on :443)
	// port: one route per port (TCP/UDP exposure)
	Granularity RoutingGranularity
}

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
	ForwardHTTPProxy   ForwardKind = "http_proxy"    // HTTP reverse proxy
	ForwardTCPStream   ForwardKind = "tcp_stream"    // raw TCP stream
	ForwardUDPDatagram ForwardKind = "udp_datagram"  // raw UDP datagrams
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
var GatewayTypeDefs = map[GatewayType]GatewayTypeDef{
	TypeHTTPTerm: {
		Type: TypeHTTPTerm, Label: "HTTP 终结网关",
		MatchKey: MatchHostPath, ForwardKind: ForwardHTTPProxy,
		TerminatesTLS: true, UnderstandsHTTP: true,
		Granularity: RouteByDomainPath,
	},
	TypeSNIPass: {
		Type: TypeSNIPass, Label: "TLS SNI 直通网关",
		MatchKey: MatchSNI, ForwardKind: ForwardTCPStream,
		TerminatesTLS: false, UnderstandsHTTP: false,
		Granularity: RouteByDomain,
	},
	TypeTCPForward: {
		Type: TypeTCPForward, Label: "TCP 端口网关",
		MatchKey: MatchPort, ForwardKind: ForwardTCPStream,
		TerminatesTLS: false, UnderstandsHTTP: false,
		Granularity: RouteByPort,
	},
	TypeUDPForward: {
		Type: TypeUDPForward, Label: "UDP 端口网关",
		MatchKey: MatchPort, ForwardKind: ForwardUDPDatagram,
		TerminatesTLS: false, UnderstandsHTTP: false,
		Granularity: RouteByPort,
	},
	TypeTransparent: {
		Type: TypeTransparent, Label: "透明劫持网关",
		MatchKey: MatchDestAddr, ForwardKind: ForwardTCPStream,
		TerminatesTLS: false, UnderstandsHTTP: false,
		Granularity: RouteByPort,
	},
}

// ============================================================================
// Layer 2: Protocol — the application protocol the traffic speaks
// ============================================================================

// Protocol identifies the application-layer protocol used between the gateway
// and the upstream endpoint. This is separate from GatewayType — the same
// HTTP-termination gateway (Caddy) can forward HTTP/1.1, WebSocket, gRPC, etc.
type Protocol string

const (
	ProtoHTTP1     Protocol = "http/1.1"
	ProtoHTTP2     Protocol = "http/2"      // h2, h2c
	ProtoHTTP3     Protocol = "http/3"      // QUIC (UDP transport)
	ProtoWebSocket Protocol = "websocket"    // HTTP Upgrade → bidirectional TCP pipe
	ProtoSSE       Protocol = "sse"         // Server-Sent Events (text/event-stream)
	ProtoGRPC      Protocol = "grpc"        // HTTP/2 + protobuf + trailers
	ProtoTCP       Protocol = "tcp"         // raw TCP stream
	ProtoUDP       Protocol = "udp"         // raw UDP datagram
	ProtoUnix      Protocol = "unix"        // Unix domain socket
)

// ProtocolDef describes how a protocol behaves at the transport level.
// This tells the gateway what special handling (if any) is needed.
type ProtocolDef struct {
	ID       Protocol
	Label    string
	Transport string // underlying transport: "tcp" | "udp" | "unix"

	// NeedsUpgrade is true for protocols that start as HTTP and then upgrade
	// to a different transport mode (WebSocket: Upgrade: websocket).
	NeedsUpgrade bool

	// Persistent is true for long-lived connections (WebSocket, gRPC, TCP streams).
	// Gateways should not apply short HTTP timeouts to persistent connections.
	Persistent bool

	// CustomHeaders are headers the gateway must set for this protocol to work.
	// e.g. WebSocket needs Upgrade + Connection headers.
	CustomHeaders map[string]string

	// IdleTimeout is how long an idle connection stays open.
	// 0 means no timeout (WebSocket).
	IdleTimeout time.Duration
}

// ProtocolDefs maps each Protocol to its behavior definition.
// This is the single source of truth for protocol handling.
var ProtocolDefs = map[Protocol]ProtocolDef{
	ProtoHTTP1: {
		ID: ProtoHTTP1, Label: "HTTP/1.1",
		Transport: "tcp", Persistent: false, IdleTimeout: 30 * time.Second,
	},
	ProtoHTTP2: {
		ID: ProtoHTTP2, Label: "HTTP/2",
		Transport: "tcp", Persistent: true, IdleTimeout: 60 * time.Second,
	},
	ProtoHTTP3: {
		ID: ProtoHTTP3, Label: "HTTP/3 (QUIC)",
		Transport: "udp", Persistent: true, IdleTimeout: 60 * time.Second,
	},
	ProtoWebSocket: {
		ID: ProtoWebSocket, Label: "WebSocket",
		Transport: "tcp", NeedsUpgrade: true, Persistent: true, IdleTimeout: 0,
		CustomHeaders: map[string]string{"Upgrade": "websocket", "Connection": "Upgrade"},
	},
	ProtoSSE: {
		ID: ProtoSSE, Label: "SSE",
		Transport: "tcp", Persistent: true, IdleTimeout: 0,
	},
	ProtoGRPC: {
		ID: ProtoGRPC, Label: "gRPC",
		Transport: "tcp", Persistent: true, IdleTimeout: 60 * time.Second,
	},
	ProtoTCP: {
		ID: ProtoTCP, Label: "TCP 流",
		Transport: "tcp", Persistent: true, IdleTimeout: 0,
	},
	ProtoUDP: {
		ID: ProtoUDP, Label: "UDP 报",
		Transport: "udp", Persistent: false, IdleTimeout: 60 * time.Second,
	},
	ProtoUnix: {
		ID: ProtoUnix, Label: "Unix Socket",
		Transport: "unix", Persistent: true, IdleTimeout: 0,
	},
}

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
	Impact   string      `json:"impact"`             // human-readable: "HTTPS 外部入口不可用"
	Affects  []string    `json:"affects,omitempty"`  // which traffic types are affected
}

// ============================================================================
// Layer 4-5: Provider capabilities & UI hints (for discovery → UI rendering)
// ============================================================================

// ProviderCapabilities declares what a specific provider implementation CAN do.
// This is used by the capability matrix to answer "can this node do X?"
// Every Provider must implement a Capabilities() method returning this.
type ProviderCapabilities struct {
	// Identity
	ProviderID   string      `json:"provider_id"`   // "caddy", "haproxy_edge_mux", etc.
	ProviderName string      `json:"provider_name"`  // human-readable
	GatewayType  GatewayType `json:"gateway_type"`   // which of the 5 types

	// Match capabilities — what keys can this provider match on?
	MatchKeys []MatchKey `json:"match_keys"`

	// Protocol support — what protocols can this provider forward?
	Protocols []Protocol `json:"protocols"`

	// Feature flags
	AutoTLS          bool `json:"auto_tls"`           // automatic HTTPS (Let's Encrypt)
	LoadBalance      bool `json:"load_balance"`       // can distribute across multiple backends
	HealthCheck      bool `json:"health_check"`       // can actively health-check backends
	RateLimit        bool `json:"rate_limit"`         // can rate-limit requests
	ConfigImport     bool `json:"config_import"`      // can parse existing config into Aegis routes
	SNIPassthrough   bool `json:"sni_passthrough"`    // can route based on TLS SNI
	CanInstall       bool `json:"can_install"`        // can be installed via package manager
}

// ProviderUIHints tells the frontend how to render this provider.
// Each provider type gets different panels, metrics, and operations.
// The frontend adapter registry uses these hints to dynamically load components.
type ProviderUIHints struct {
	// ConfigSyntax is the syntax of the generated config file,
	// used by the frontend for syntax highlighting.
	// Values: "caddyfile" | "haproxy" | "nginx" | "json" | "yaml" | "plain"
	ConfigSyntax string `json:"config_syntax"`

	// ConfigMimeType for code highlighting.
	ConfigMimeType string `json:"config_mime_type"`

	// CustomPanels are provider-specific UI panels shown in the gateway detail page.
	// Each panel maps to a frontend component registered in the adapter registry.
	CustomPanels []PanelDef `json:"custom_panels,omitempty"`

	// DashboardMetrics are provider-specific metrics shown in the overview.
	DashboardMetrics []MetricDef `json:"dashboard_metrics,omitempty"`

	// CustomOperations are provider-specific actions (beyond the standard reload/validate).
	CustomOperations []OperationDef `json:"custom_operations,omitempty"`
}

// PanelDef describes a provider-specific detail panel for the UI.
type PanelDef struct {
	ID        string `json:"id"`        // frontend component name, e.g. "sni_routing_table"
	Label     string `json:"label"`     // display label, e.g. "SNI 路由表"
	Priority  int    `json:"priority"`  // sort order; lower = first
	MinWidth  int    `json:"min_width"` // minimum width in pixels
}

// MetricDef describes a provider-specific metric for the dashboard.
type MetricDef struct {
	Key    string `json:"key"`    // unique metric identifier
	Label  string `json:"label"`  // human-readable label
	Unit   string `json:"unit"`   // "conns", "req/s", "ms", "%", "certs"
	Source string `json:"source"` // how to get the value: "systemctl_status" | "admin_api" | "log_parse"
}

// OperationDef describes a provider-specific action.
type OperationDef struct {
	ID    string `json:"id"`    // operation identifier
	Label string `json:"label"` // display label
	Risk  string `json:"risk"`  // "low" | "medium" | "high" — for the risk evaluator
}

// ============================================================================
// Capability defaults for built-in providers
// ============================================================================

// CaddyCapabilities returns the capability declaration for the Caddy HTTP provider.
func CaddyCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "caddy", ProviderName: "Caddy", GatewayType: TypeHTTPTerm,
		MatchKeys:  []MatchKey{MatchHostPath},
		Protocols:  []Protocol{ProtoHTTP1, ProtoHTTP2, ProtoHTTP3, ProtoWebSocket, ProtoGRPC, ProtoSSE},
		AutoTLS:    true, LoadBalance: true, RateLimit: true, ConfigImport: true,
		CanInstall: true,
	}
}

// CaddyUIHints returns the UI hints for the Caddy HTTP provider.
func CaddyUIHints() ProviderUIHints {
	return ProviderUIHints{
		ConfigSyntax:   "caddyfile",
		ConfigMimeType: "text/x-caddyfile",
		CustomPanels: []PanelDef{
			{ID: "caddyfile_viewer", Label: "Caddyfile", Priority: 10, MinWidth: 400},
			{ID: "http_routes", Label: "HTTP 路由", Priority: 5, MinWidth: 300},
		},
		DashboardMetrics: []MetricDef{
			{Key: "auto_tls_certs", Label: "证书数量", Unit: "certs", Source: "admin_api"},
			{Key: "config_reloads", Label: "重载次数", Unit: "reloads", Source: "systemctl_status"},
		},
		CustomOperations: []OperationDef{
			{ID: "import_caddyfile", Label: "从 Caddyfile 导入", Risk: "low"},
		},
	}
}

// HAProxyEdgeCapabilities returns the capability declaration for HAProxy EdgeMux.
func HAProxyEdgeCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "haproxy_edge_mux", ProviderName: "HAProxy EdgeMux", GatewayType: TypeSNIPass,
		MatchKeys:      []MatchKey{MatchSNI},
		Protocols:      []Protocol{ProtoTCP}, // SNI passthrough only forwards TCP streams
		SNIPassthrough: true, LoadBalance: false, HealthCheck: true,
		CanInstall: true,
	}
}

// HAProxyEdgeUIHints returns the UI hints for HAProxy EdgeMux.
func HAProxyEdgeUIHints() ProviderUIHints {
	return ProviderUIHints{
		ConfigSyntax:   "haproxy",
		ConfigMimeType: "text/x-haproxy",
		CustomPanels: []PanelDef{
			{ID: "sni_routing_table", Label: "SNI 路由表", Priority: 5, MinWidth: 400},
			{ID: "backend_pools", Label: "后端池", Priority: 10, MinWidth: 300},
			{ID: "haproxy_config", Label: "haproxy.cfg", Priority: 15, MinWidth: 400},
		},
		DashboardMetrics: []MetricDef{
			{Key: "active_sessions", Label: "活跃会话", Unit: "sessions", Source: "admin_api"},
			{Key: "backend_health", Label: "后端健康", Unit: "%", Source: "admin_api"},
		},
		CustomOperations: []OperationDef{
			{ID: "view_stats", Label: "查看统计 (stats)", Risk: "low"},
			{ID: "sni_probe", Label: "SNI 探活", Risk: "low"},
		},
	}
}

// HAProxyTCPCapabilities returns the capability declaration for HAProxy TCP.
func HAProxyTCPCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "haproxy_tcp", ProviderName: "HAProxy TCP", GatewayType: TypeTCPForward,
		MatchKeys:  []MatchKey{MatchPort},
		Protocols:  []Protocol{ProtoTCP},
		LoadBalance: false, HealthCheck: true,
		CanInstall: false, // HAProxy TCP shares the same binary as EdgeMux
	}
}

// HAProxyTCPUIHints returns the UI hints for HAProxy TCP.
func HAProxyTCPUIHints() ProviderUIHints {
	return ProviderUIHints{
		ConfigSyntax:   "haproxy",
		ConfigMimeType: "text/x-haproxy",
		CustomPanels: []PanelDef{
			{ID: "tcp_forward_rules", Label: "TCP 转发规则", Priority: 5, MinWidth: 400},
		},
		DashboardMetrics: []MetricDef{
			{Key: "tcp_connections", Label: "TCP 连接数", Unit: "conns", Source: "admin_api"},
		},
	}
}

// AegisTCPCapabilities returns the capability declaration for Aegis TCP Manager.
func AegisTCPCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "aegis_tcp", ProviderName: "Aegis TCP Manager", GatewayType: TypeTCPForward,
		MatchKeys:  []MatchKey{MatchPort},
		Protocols:  []Protocol{ProtoTCP, ProtoUnix},
		LoadBalance: false, HealthCheck: false,
		CanInstall: false, // built-in, no external binary needed
	}
}

// AegisUDPCapabilities returns the capability declaration for Aegis UDP Manager.
func AegisUDPCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "aegis_udp", ProviderName: "Aegis UDP Manager", GatewayType: TypeUDPForward,
		MatchKeys:  []MatchKey{MatchPort},
		Protocols:  []Protocol{ProtoUDP, ProtoUnix},
		LoadBalance: false, HealthCheck: false,
		CanInstall: false, // built-in
	}
}

// AegisTransparentCapabilities returns the capability declaration for transparent proxy.
func AegisTransparentCapabilities() ProviderCapabilities {
	return ProviderCapabilities{
		ProviderID: "aegis_transparent", ProviderName: "Aegis 透明代理", GatewayType: TypeTransparent,
		MatchKeys:  []MatchKey{MatchDestAddr},
		Protocols:  []Protocol{ProtoTCP},
		LoadBalance: false, HealthCheck: false,
		CanInstall: false, // built-in, but requires iptables + Linux
	}
}

// ============================================================================
// Port binding model — the physical constraint that drives capability computation
// ============================================================================

// PortBinding represents a port allocation on a node.
// Each port can have exactly one owner.
type PortBinding struct {
	Port     int    `json:"port"`
	Owner    string `json:"owner"`    // provider ID or gateway ID
	Protocol string `json:"protocol"` // "tcp" | "udp"
	Purpose  string `json:"purpose"`  // "http" | "https" | "tls_sni_mux" | "tcp_exposure" | "udp_exposure" | "internal_https"
	Status   string `json:"status"`   // "active" | "planned" | "conflict"
}

// PortPolicy represents the port allocation strategy for a node.
// There are two modes: legacy (Caddy only) and edge_mux (Caddy + HAProxy).
type PortPolicy struct {
	Mode     string        `json:"mode"`     // "legacy" | "edge_mux"
	Bindings []PortBinding `json:"bindings"`
}

// DefaultLegacyPortPolicy returns the port allocation when only Caddy is installed.
// Caddy owns both :80 (HTTP) and :443 (HTTPS with TLS termination).
func DefaultLegacyPortPolicy() PortPolicy {
	return PortPolicy{
		Mode: "legacy",
		Bindings: []PortBinding{
			{Port: 80, Owner: "caddy", Protocol: "tcp", Purpose: "http", Status: "active"},
			{Port: 443, Owner: "caddy", Protocol: "tcp", Purpose: "https", Status: "active"},
		},
	}
}

// DefaultEdgeMuxPortPolicy returns the port allocation when HAProxy is installed.
// HAProxy owns :443 (TLS SNI passthrough), Caddy owns :80 (HTTP) and :8443 (internal HTTPS).
func DefaultEdgeMuxPortPolicy() PortPolicy {
	return PortPolicy{
		Mode: "edge_mux",
		Bindings: []PortBinding{
			{Port: 80, Owner: "caddy", Protocol: "tcp", Purpose: "http", Status: "active"},
			{Port: 443, Owner: "haproxy_edge_mux", Protocol: "tcp", Purpose: "tls_sni_mux", Status: "active"},
			{Port: 8443, Owner: "caddy", Protocol: "tcp", Purpose: "internal_https", Status: "active"},
		},
	}
}
