// Package topology — dimension 2: topology combination decisions.
//
// This package answers: "given the user's traffic requirements and the currently
// available middleware, what combination of providers should handle this traffic?"
//
// Input:  GatewayIntent (user requirements) + []provider.ProviderState (available)
// Output: TopologyPlan (per-provider Plans + ForwardTarget + alternatives)
//
// This package does NOT:
//   - Generate middleware config (that's dimension 1: provider.Render)
//   - Install/start/stop middleware (that's dimension 3: lifecycle.Manager)
//   - Execute iptables rules (that's transparent.Manager)
package topology

import "aegis/internal/provider"

// ============================================================================
// GatewayIntent — user's traffic routing requirement
// ============================================================================

// GatewayIntent is a collection of route intents that must be served together.
// It answers: "here's what I want the gateway to do." The Planner translates
// this into per-provider Plans.
type GatewayIntent struct {
	// Routes is the list of traffic routing requirements.
	Routes []RouteIntent `json:"routes"`

	// TransparentProxy, if true, means iptables-based transparent interception
	// is active on this node. The Planner must set ForwardTarget in the output.
	TransparentProxy bool `json:"transparent_proxy,omitempty"`
}

// RouteIntent describes a single traffic routing requirement using the
// 5-dimension model: transport × tlsMode × appProtocol × match × upstream.
//
// This is what the USER declares. It is middleware-agnostic — the Planner
// decides which Provider(s) can fulfill it.
type RouteIntent struct {
	// Domain is the primary routing key: "api.example.com"
	Domain string `json:"domain"`

	// Composition is the binding capability key: "https_route", "http_route", etc.
	// v1.8L-22 — replaces the individual Transport/Port/TLSMode derivation.
	Composition string `json:"composition"`

	// Port the traffic arrives on. Typically 80 or 443.
	Port int `json:"port"`

	// Transport layer protocol.
	// "tcp" — most HTTP/HTTPS/TCP traffic
	// "udp" — HTTP/3 QUIC, raw UDP forwarding
	Transport string `json:"transport"` // "tcp" | "udp"

	// TLS mode for incoming traffic.
	// "none"        — plain HTTP or raw TCP, no TLS
	// "terminate"   — decrypt TLS at the edge (read HTTP content)
	// "passthrough" — forward TLS without decrypting (SNI-based routing)
	TLSMode string `json:"tls_mode"` // "none" | "terminate" | "passthrough"

	// Path is an optional URL path prefix for HTTP routing.
	// Only meaningful when TLSMode is "terminate".
	Path string `json:"path,omitempty"`

	// AppProtocol is the application-layer protocol.
	// "http" | "grpc" | "websocket" | "raw" | "sse"
	AppProtocol string `json:"app_protocol"`

	// HTTPVersion specifies the HTTP version when AppProtocol is "http".
	// "h1" | "h2" | "h3". Empty means "h1" by default.
	HTTPVersion string `json:"http_version,omitempty"`

	// Upstream is where to forward matched traffic.
	// Format depends on upstream type:
	//   "host:port"        → TCP upstream (default)
	//   "unix:///path/sock" → Unix socket
	//   "http://host:port"  → HTTP upstream
	Upstream string `json:"upstream"`

	// ExtraHeaders are injected into forwarded requests.
	// Set by the Planner during Gateway Link resolution.
	ExtraHeaders map[string]string `json:"extra_headers,omitempty"`

	// StripPathPrefix removes the matched path prefix before forwarding.
	StripPathPrefix bool `json:"strip_path_prefix,omitempty"`

	// MaintenanceEnabled puts the route in maintenance mode (503 response).
	MaintenanceEnabled bool   `json:"maintenance_enabled,omitempty"`
	MaintenanceMessage string `json:"maintenance_message,omitempty"`

	// ─── Internal fields (set by Planner, not part of public API) ───

	// gatewayLinkID is the cross-machine link to resolve (internal use).
	gatewayLinkID string

	// serviceID is the service to resolve endpoint for (internal use).
	serviceID string
}

// RequirementsOf extracts the Capabilities required to serve this intent.
// v1.8L-22: reads from the composition registry instead of switch/case.
func (ri RouteIntent) RequirementsOf() []provider.Capability {
	// Primary path: look up the composition definition
	if ri.Composition != "" {
		if def := provider.LookupComp(provider.CompKey(ri.Composition)); def != nil {
			return def.Requirements()
		}
	}

	// Fallback for routes without a composition set (backward compat)
	var caps []provider.Capability
	if ri.Transport == "udp" {
		caps = append(caps, provider.CapListenUDP, provider.CapUpstreamUDP)
	} else {
		caps = append(caps, provider.CapListenTCP, provider.CapUpstreamTCP)
	}
	switch ri.TLSMode {
	case "terminate":
		caps = append(caps, provider.CapTLSTerminate)
	case "passthrough":
		caps = append(caps, provider.CapTLSPassthrough, provider.CapSNIPreread)
	default:
		// "none" or unrecognized — no TLS capabilities needed
	}
	switch ri.AppProtocol {
	case "http":
		caps = append(caps, provider.CapHTTP1, provider.CapRouteHost)
		switch ri.HTTPVersion {
		case "h2":
			caps = append(caps, provider.CapHTTP2)
		case "h3":
			caps = append(caps, provider.CapHTTP3)
		default:
			// "h1" is implicit with CapHTTP1
		}
		if ri.Path != "" {
			caps = append(caps, provider.CapRoutePath)
		}
	case "grpc":
		caps = append(caps, provider.CapGRPC, provider.CapRouteHost)
	case "websocket":
		caps = append(caps, provider.CapWebSocket, provider.CapRouteHost)
	default:
		// "raw", "sse", or unrecognized — no additional L7 protocol caps
	}
	return caps
}
