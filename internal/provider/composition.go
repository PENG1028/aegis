// Package provider — Composition registry: the single source of truth for binding capabilities.
//
// A Composition is a named, user-facing "final binding capability" —
// the thing a user actually selects when creating a route. For example,
// "HTTPS Route" means "I want to expose an HTTPS service on my domain."
//
// Each Composition decomposes into an ordered chain of RuntimeAtoms.
// Those atoms determine which Providers are needed and whether the
// composition is currently available in a given RuntimeMode.
//
// ============================================================================
// Single-source-of-truth rule
// ============================================================================
//
// Every piece of code that needs to know about compositions reads from this file:
//
//   Route.Composition → stored as CompKey (e.g. "https_route")
//   collectIntents() → Lookup(rt.Composition) → fills Transport/Port/TLSMode/AppProtocol
//   RequirementsOf() → Lookup(ri.Composition) → derives []Capability
//   RuntimeMode     → buildCompositions(defs, modeID) → UI renders
//   Frontend        → GET /api/system/runtime-mode → compositions array
//
// Adding a new binding capability:
//   1. Add a CompDef to the AllCompositions slice below
//   2. That's it — zero changes to Planner, templates, or frontend.

package provider

// ============================================================================
// CompKey — machine-readable composition identifier
// ============================================================================

// CompKey is the canonical identifier for a composition.
// Stored in Route.Composition and referenced everywhere.
type CompKey string

const (
	CompHTTPRoute      CompKey = "http_route"
	CompHTTPSRoute     CompKey = "https_route"
	CompTLSPassthrough CompKey = "tls_passthrough"
	CompHTTP3          CompKey = "http3"
	CompRawTCP         CompKey = "raw_tcp"
	CompRawUDP         CompKey = "raw_udp"
)

// AllCompKeys returns every defined composition key.
func AllCompKeys() []CompKey {
	return []CompKey{CompHTTPRoute, CompHTTPSRoute, CompTLSPassthrough, CompHTTP3, CompRawTCP, CompRawUDP}
}

// ============================================================================
// CompDef — the canonical definition of one binding capability
// ============================================================================

// CompDef defines everything the system needs to know about a composition.
type CompDef struct {
	// Identity
	Key         CompKey `json:"key"`         // "https_route"
	Name        string  `json:"name"`        // "HTTPS Route"
	Description string  `json:"description"` // one-line explanation for tooltips

	// Display
	Atoms []string `json:"atoms"` // ordered atom keys: ["tcp","tls","http"]
	Chain string   `json:"chain"` // "L4 → L5 → L7"

	// RouteIntent derivation — these fields drive the Planner
	Transport   string `json:"transport"`    // "tcp" | "udp"
	Port        int    `json:"port"`         // 80 | 443
	TLSMode     string `json:"tls_mode"`     // "none" | "terminate" | "passthrough"
	AppProtocol string `json:"app_protocol"` // "http" | "raw"
}

// Requirements returns the Capability constants needed to fulfill this composition.
func (d CompDef) Requirements() []Capability {
	var caps []Capability

	// L4 transport
	switch d.Transport {
	case "udp":
		caps = append(caps, CapListenUDP, CapUpstreamUDP)
	default:
		caps = append(caps, CapListenTCP, CapUpstreamTCP)
	}

	// L5 TLS mode
	switch d.TLSMode {
	case "terminate":
		caps = append(caps, CapTLSTerminate)
	case "passthrough":
		caps = append(caps, CapTLSPassthrough, CapSNIPreread)
	}

	// L7 application protocol
	switch d.AppProtocol {
	case "http":
		caps = append(caps, CapHTTP1, CapRouteHost)
		// HTTP/3 over UDP requires QUIC capability
		if d.Transport == "udp" {
			caps = append(caps, CapHTTP3)
		}
	}

	return caps
}

// IsTransparentForwardTarget returns true if this composition can serve as a
// forward entry for transparent proxy iptables interception.
//
// Not all compositions can: Raw TCP/UDP forward to specific ports, not through
// a shared HTTP router. TLS Passthrough routes by SNI, not by destination.
//
// Currently only HTTP-based compositions qualify — they provide a shared entry
// port (Caddy :80/:8443) that routes arbitrary traffic by Host header.
// When new compositions are added to the registry (e.g. future gRPC proxy),
// this method is the single place to declare whether they qualify.
func (d CompDef) IsTransparentForwardTarget() bool {
	// All compositions can serve as transparent proxy forward targets.
	// HTTP compositions route by Host header; raw TCP/UDP forward by port;
	// TLS passthrough routes by SNI.
	//
	// The transparent proxy status endpoint reads mode.Compositions (which
	// already has status computed by EvalAllCompositions). Only "available"
	// compositions appear as usable targets — the rest are shown as
	// "provider_missing" or "unsupported" for diagnosis.
	//
	// When a new composition is added to the registry, it automatically
	// appears here with zero code changes.
	return true
}

// ============================================================================
// Registry — the canonical set of all compositions
// ============================================================================

// AllCompositions returns every defined composition in display order.
// This is the single source of truth. Every consumer reads from here.
func AllCompositions() []CompDef {
	return []CompDef{
		{
			Key: CompHTTPRoute, Name: "HTTP Route",
			Description: "明文 HTTP 服务，绑域名即可访问",
			Atoms:       []string{"tcp", "http"}, Chain: "L4 → L7",
			Transport: "tcp", Port: 80, TLSMode: "none", AppProtocol: "http",
		},
		{
			Key: CompHTTPSRoute, Name: "HTTPS Route",
			Description: "TLS 加密 HTTP 服务，自动证书管理",
			Atoms:       []string{"tcp", "tls", "http"}, Chain: "L4 → L5 → L7",
			Transport: "tcp", Port: 443, TLSMode: "terminate", AppProtocol: "http",
		},
		{
			Key: CompTLSPassthrough, Name: "TLS Passthrough",
			Description: "TLS 直通，不解密，由后端服务自己处理 TLS",
			Atoms:       []string{"tcp", "sni"}, Chain: "L4 → L5",
			Transport: "tcp", Port: 443, TLSMode: "passthrough", AppProtocol: "raw",
		},
		{
			Key: CompHTTP3, Name: "HTTP/3",
			Description: "QUIC / HTTP/3 服务，UDP 传输",
			Atoms:       []string{"udp", "quic", "http"}, Chain: "L4 → L5 → L7",
			Transport: "udp", Port: 443, TLSMode: "terminate", AppProtocol: "http",
		},
		{
			Key: CompRawTCP, Name: "Raw TCP Forward",
			Description: "原始 TCP 端口转发，用于 MySQL/SSH/自定义 TCP 协议",
			Atoms:       []string{"tcp"}, Chain: "L4",
			Transport: "tcp", Port: 0, TLSMode: "none", AppProtocol: "raw",
		},
		{
			Key: CompRawUDP, Name: "Raw UDP Forward",
			Description: "原始 UDP 数据报转发，用于 DNS/游戏/自定义 UDP 协议",
			Atoms:       []string{"udp"}, Chain: "L4",
			Transport: "udp", Port: 0, TLSMode: "none", AppProtocol: "raw",
		},
	}
}

// LookupCompByName returns the CompDef for a given human-readable name, or nil.
func LookupCompByName(name string) *CompDef {
	for _, d := range AllCompositions() {
		if d.Name == name {
			return &d
		}
	}
	return nil
}

// LookupComp returns the CompDef for a given key, or nil if not found.
func LookupComp(key CompKey) *CompDef {
	for _, d := range AllCompositions() {
		if d.Key == key {
			return &d
		}
	}
	return nil
}

// CompKeySupported returns true if the given composition is supported in the
// given mode (i.e., all its atoms can be satisfied by the mode's providers).
// A composition is "supported" if at least one provider provides each atom.
func CompKeySupported(key CompKey, mode RuntimeMode) bool {
	def := LookupComp(key)
	if def == nil || len(def.Atoms) == 0 {
		return false
	}
	for _, atomKey := range def.Atoms {
		if !mode.hasAtomBinding(atomKey) {
			return false
		}
	}
	return true
}

// hasAtomBinding returns true if any provider in this mode has a binding
// (non-empty slots) for the given atom key.
func (m RuntimeMode) hasAtomBinding(atomKey string) bool {
	for _, p := range m.Providers {
		slots, ok := p.Bindings[atomKey]
		if ok && len(slots) > 0 {
			return true
		}
	}
	return false
}

// ============================================================================
// Build compositions for a RuntimeMode from the registry
// ============================================================================

// buildCompositions generates the mode-specific Composition list from the
// canonical CompDef registry. A composition with no supported atoms in this
// mode gets atoms=nil (displayed as grey/unavailable in the frontend).
func buildCompositions(modeID string) []Composition {
	var out []Composition
	for _, def := range AllCompositions() {
		// For now, all compositions use their canonical atoms.
		// In the future, a mode can override the atom chain (e.g. EdgeMux
		// HTTPS Route adds an SNI step between TCP and TLS).
		atoms := def.Atoms
		chain := def.Chain
		out = append(out, Composition{
			Name:  def.Name,
			Atoms: atoms,
			Chain: chain,
		})
	}
	return out
}
