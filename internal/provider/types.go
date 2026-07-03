// Package provider — dimension 1 of the 3-dimension gateway architecture.
//
// This file defines the fixed type system: 5 GatewayTypes, port binding, and
// dependency edges. These types answer "what kind of gateway is this?" — they
// do NOT describe what a gateway CAN do (that's Capability in capability.go)
// or what it IS doing right now (that's ProviderState in state.go).
//
// ## File map (dimension 1 = provider package):
//
//   capability.go — 26 Capability constants (L3-L7), Layer(), IsIngress()
//   state.go      — ProviderState, Plan, ListenerSpec, RouteSpec, MatchSpec,
//                    UpstreamSpec, ConfigFile, ForwardTarget
//   types.go      — GatewayType (5 fixed types), PortPolicy, PortBinding,
//                    DependencyEdge (this file)
//   provider.go   — Provider interface (State / Diagnose / Render / Apply)
//   registry.go   — Provider registry (Register / Get / List)
//   discovery.go  — Runtime detection (DiscoverProviders / CurrentPortPolicyMode)
//   diagnostic.go — ProviderDiagnostic struct + diagnostic error codes
//   caddy.go      — CaddyProvider implementation
//   haproxy.go    — HAProxyProvider implementation (merged edge + tcp)
//   install.go    — apt-get install/uninstall helpers (used by dimension 3)
//
// ## The 3 dimensions (for orientation):
//
//   Dimension 1 (this package) — Provider adapter: wrap each middleware as a
//   unified interface. One middleware = one Provider.
//
//   Dimension 2 (internal/topology/) — Topology planner: turn user intent into
//   provider assignments. Chooses which providers handle which routes.
//
//   Dimension 3 (internal/lifecycle/) — Lifecycle manager: install, start, stop,
//   uninstall middleware. Writes ProviderState; dimensions 1 & 2 read it.
//
// ## Port allocation:
//
//   Legacy mode (no HAProxy):  Caddy owns 80 + 443
//   EdgeMux mode (HAProxy):    Caddy owns 80 + 8443, HAProxy owns 443
//   See: discovery.go CurrentPortPolicyMode() for the dynamic detection logic.
//
// ## Adding a new provider (e.g. Nginx):
//
//   1. Implement the Provider interface (see caddy.go for reference)
//   2. Register in registry.go
//   3. Add topology templates that include it (internal/topology/templates/)
//   4. Add lifecycle operations (internal/lifecycle/)
//   5. Add frontend adapters in ui/

package provider

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

// The old Protocol enum and ProviderCapabilities struct have been replaced by:
//   - capability.go: 26 fine-grained Capability constants (L3-L7)
//   - state.go: RouteSpec (5-dimension intent model: transport × tlsMode ×
//     appProtocol × match × upstream)
//   - state.go: ProviderState.Capabilities []Capability (per-instance declaration)

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

// NOTE: ProviderCapabilities, ProviderUIHints and static functions like
// CaddyCapabilities() / HAProxyEdgeCapabilities() have been removed.
// Each Provider now returns a []Capability slice (from capability.go) and
// ProviderState (from state.go) carries the Capabilities + Diagnostic.

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
