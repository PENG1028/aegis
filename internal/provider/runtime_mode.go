// Package provider — RuntimeMode: the single source of truth for deployment modes.
//
// RuntimeMode replaces three scattered mode definitions:
//  1. Frontend MODES[] hand-coded in ui/src/pages/fabric/Providers.tsx
//  2. Template hardcoded port numbers in internal/topology/templates/*.go
//  3. Shell-based CurrentPortPolicyMode() in discovery.go
//
// With RuntimeMode, all consumers (templates, Planner, API, frontend) reference
// the same canonical definitions. Changing a mode's port binding requires only
// one edit — the RuntimeMode variable in this file.
//
// ============================================================================
// Atom-first model (v1.8L-21)
// ============================================================================
//
// The old model used 5 compound "traffic columns" (HTTP, HTTPS, SNI, TCP, UDP)
// that conflated OSI layers — "HTTPS" meant L4 TCP listen + L5 TLS decrypt +
// L7 HTTP route, which made it impossible to answer "which provider handles
// TLS for this traffic?" without parsing a compound column name.
//
// The atom model decomposes traffic handling into 9 indivisible RuntimeAtoms
// organized by OSI layer:
//
//   L4 地基:  TCP Entry, UDP Entry        — port binding
//   L5 安全:  TLS Terminate, SNI Preread, QUIC Session — what happens to the stream
//   L7 应用:  HTTP Route, gRPC, WebSocket, CONNECT     — application protocol handling
//
// Each atom maps 1:1 to a Capability constant in capability.go.
// A Provider either handles an atom (with specific port/action/target) or doesn't.
//
// Compositions (e.g. "HTTPS Route" = TCP + TLS + HTTP) are defined separately
// and rendered as overlay cards — they are the user-facing "use case" view,
// not the canonical data model.

package provider

import "fmt"

// ============================================================================
// RuntimeAtom — an indivisible gateway capability, the atomic unit of the matrix
// ============================================================================

// RuntimeAtom is one atomic capability in the runtime binding matrix.
// Atoms are the columns of the matrix — each atom corresponds to exactly one
// concrete action a Provider can take on traffic. Compounds like "HTTPS Route"
// are Compositions of atoms, not atoms themselves.
type RuntimeAtom struct {
	Key   string `json:"key"`   // "tcp", "udp", "tls", "sni", "quic", "http", "grpc", "ws", "connect"
	Layer string `json:"layer"` // "L4", "L5", "L7"
	Label string `json:"label"` // "TCP Entry", "UDP Entry", ...
}

// AllAtomsInDisplayOrder returns all 9 atoms in layer-grouped display order:
// L4 first (地基), L5 second, L7 last. The frontend renders columns in this order.
func AllAtomsInDisplayOrder() []RuntimeAtom {
	return []RuntimeAtom{
		// L4 — 地基（端口绑定）
		{Key: "tcp", Layer: "L4", Label: "TCP Entry"},
		{Key: "udp", Layer: "L4", Label: "UDP Entry"},

		// L5 — 安全层（对流做什么）
		{Key: "tls", Layer: "L5", Label: "TLS Terminate"},
		{Key: "sni", Layer: "L5", Label: "SNI Preread"},
		{Key: "quic", Layer: "L5", Label: "QUIC/HTTP3"},

		// L7 — 应用层（协议处理）
		{Key: "http", Layer: "L7", Label: "HTTP Route"},
		{Key: "grpc", Layer: "L7", Label: "gRPC"},
		{Key: "ws", Layer: "L7", Label: "WebSocket"},
		{Key: "connect", Layer: "L7", Label: "CONNECT"},
	}
}

// CapabilityKey maps an atom key to the corresponding Capability constant.
// Used by the Planner to verify that a Provider actually declares the capability
// it claims in the RuntimeMode binding.
func (a RuntimeAtom) CapabilityKey() string {
	switch a.Key {
	case "tcp":
		return "listen_tcp"
	case "udp":
		return "listen_udp"
	case "tls":
		return "tls_terminate"
	case "sni":
		return "sni_preread"
	case "quic":
		return "http3"
	case "http":
		return "http1" // base HTTP capability; http2 is additive
	case "grpc":
		return "grpc"
	case "ws":
		return "websocket"
	case "connect":
		return "connect" // not yet in capability.go — reserved
	default:
		return ""
	}
}

// ============================================================================
// AtomSlot — one concrete port binding for one atom
// ============================================================================

// AtomSlot describes one concrete port binding or protocol route for a
// specific atom. An atom can have multiple slots — e.g. Caddy's TCP atom
// has two slots: :80 (for HTTP/gRPC) and :443 (for TLS→L7).
//
// For L4/L5 atoms, every slot has a Port and Protocol (it binds a port).
// For L7 atoms, slots have Port=0 — they describe which protocols are
// routed, referencing ports bound by L4/L5 atoms.
type AtomSlot struct {
	// Port is the concrete port number. 0 for L7 atoms that don't bind ports.
	Port int `json:"port"`

	// Protocol is "tcp" or "udp". Empty for L7 atoms.
	Protocol string `json:"protocol,omitempty"`

	// ListeningAt is a human-readable address: "public :80/tcp", "internal :8443/tcp".
	ListeningAt string `json:"listening_at,omitempty"`

	// Action is what the provider does: "listen", "decrypt", "preread",
	// "terminate", "route", "upgrade", "tunnel".
	Action string `json:"action"`

	// Target is where traffic goes: "EP", "Caddy :8443", "node relay".
	Target string `json:"target"`

	// Required is false for optional slots (e.g. QUIC on :443/udp).
	Required bool `json:"required"`

	// Note provides extra context: "HTTP/gRPC", "after TLS decrypt".
	Note string `json:"note,omitempty"`

	// Purpose maps to ListenerSpec.Purpose for template compatibility.
	// Values: "http", "https", "internal_https", "tls_sni_mux",
	// "tcp_exposure", "udp_exposure".
	Purpose string `json:"purpose,omitempty"`
}

// ============================================================================
// ProviderAtoms — one row in the binding matrix
// ============================================================================

// ProviderAtoms describes one provider's complete set of atom bindings
// across all atoms in a given RuntimeMode. Each atom key maps to a slice
// of AtomSlots — zero slots means this provider does not handle that atom.
type ProviderAtoms struct {
	ProviderID string                `json:"provider_id"`
	Bindings   map[string][]AtomSlot `json:"bindings"` // atom key → slots
}

// listenerSpecs derives the []ListenerSpec this provider needs for this role.
// Templates call this via RuntimeMode.ListenerSpecsFor() — the signature is
// unchanged so templates don't need modification.
func (p ProviderAtoms) listenerSpecs() []ListenerSpec {
	seen := make(map[string]bool) // "port/protocol" → dedupe
	var specs []ListenerSpec
	for _, slots := range p.Bindings {
		for _, s := range slots {
			if s.Port == 0 || s.Protocol == "" {
				continue // L7 atoms — no port binding
			}
			key := fmt.Sprintf("%d/%s", s.Port, s.Protocol)
			if seen[key] {
				continue
			}
			seen[key] = true
			specs = append(specs, ListenerSpec{
				Port:     s.Port,
				Protocol: s.Protocol,
				Purpose:  s.Purpose,
			})
		}
	}
	return specs
}

// ============================================================================
// Composition — a named chain of atoms (the "use case" view)
// ============================================================================

// Composition is a named, ordered chain of atoms that together form a
// complete traffic-handling pipeline. For example, "HTTPS Route" =
// [tcp, tls, http] — L4 listen → L5 decrypt → L7 route.
//
// Compositions are rendered as clickable cards above the atom matrix.
// Clicking a composition highlights its constituent atom columns.
type Composition struct {
	Name  string   `json:"name"`  // "HTTPS Route"
	Atoms []string `json:"atoms"` // ["tcp", "tls", "http"] — ordered
	Chain string   `json:"chain"` // "L4 → L5 → L7" — human-readable
}

// ============================================================================
// RuntimeMode — the canonical mode definition
// ============================================================================

// RuntimeMode is the single source of truth for a deployment mode.
// It defines the atom columns, which providers participate, how each atom
// is handled (via AtomSlots), and which compositions are available.
type RuntimeMode struct {
	// ID is the machine-readable identifier: "legacy", "edge_mux", etc.
	ID string `json:"id"`

	// Label is the human-readable display name.
	Label string `json:"label"`

	// Description explains the mode in one line.
	Description string `json:"description"`

	// Implemented is false for future/placeholder modes.
	Implemented bool `json:"implemented"`

	// Atoms is the ordered list of atom columns for this mode.
	Atoms []RuntimeAtom `json:"atoms"`

	// Compositions is the list of named atom chains available in this mode.
	// A composition with empty Atoms means "not available in this mode".
	Compositions []Composition `json:"compositions"`

	// Providers assigns each participating provider its atom bindings.
	Providers []ProviderAtoms `json:"providers"`
}

// ListenerSpecsFor returns the listener specs for a given provider ID in this mode.
// Templates call this to get port bindings without hardcoding numbers.
// Signature and return value unchanged from v1.8L-20 — templates are unaffected.
func (m RuntimeMode) ListenerSpecsFor(providerID string) []ListenerSpec {
	for _, p := range m.Providers {
		if p.ProviderID == providerID {
			return p.listenerSpecs()
		}
	}
	return nil
}

// PortFor returns the port a provider uses for a given atom key.
func (m RuntimeMode) PortFor(providerID string, atomKey string) (int, bool) {
	for _, p := range m.Providers {
		if p.ProviderID == providerID {
			if slots, ok := p.Bindings[atomKey]; ok && len(slots) > 0 {
				return slots[0].Port, true
			}
		}
	}
	return 0, false
}

// ProviderIDs returns all provider IDs that participate in this mode.
func (m RuntimeMode) ProviderIDs() []string {
	ids := make([]string, len(m.Providers))
	for i, p := range m.Providers {
		ids[i] = p.ProviderID
	}
	return ids
}

// ============================================================================
// Canonical mode definitions — the single source of truth
// ============================================================================

// RuntimeModeLegacy — Caddy solo on :80 + :443.
// Caddy terminates TLS, routes by Host/Path, handles auto-cert.
// No HAProxy. No SNI passthrough. No raw TCP forwarding on shared ports.
var RuntimeModeLegacy = RuntimeMode{
	ID:          "legacy",
	Label:       "Legacy",
	Description: "Caddy 直接暴露 :80 + :443，TLS 终止后路由到 EP",
	Implemented: true,
	Atoms:       AllAtomsInDisplayOrder(),
	Compositions: buildCompositions("legacy"),
	Providers: []ProviderAtoms{
		{
			ProviderID: "caddy",
			Bindings: map[string][]AtomSlot{
				// ── L4 地基 ──
				"tcp": {
					{Port: 80, Protocol: "tcp", ListeningAt: "public :80/tcp", Action: "listen", Target: "EP", Required: true, Note: "HTTP/gRPC", Purpose: "http"},
					{Port: 443, Protocol: "tcp", ListeningAt: "public :443/tcp", Action: "listen", Target: "L7", Required: true, Note: "TLS decrypt → L7", Purpose: "https"},
				},
				"udp": {
					{Port: 443, Protocol: "udp", ListeningAt: "public :443/udp", Action: "listen", Target: "EP", Required: false, Note: "QUIC/HTTP3", Purpose: "udp_exposure"},
				},
				// ── L5 安全 ──
				"tls": {
					{Port: 443, Protocol: "tcp", ListeningAt: "public :443/tcp", Action: "decrypt", Target: "L7", Required: true, Note: "TLS → HTTP/gRPC", Purpose: "https"},
				},
				"sni":  nil, // Caddy does not do SNI preread in Legacy
				"quic": {
					{Port: 443, Protocol: "udp", ListeningAt: "public :443/udp", Action: "terminate", Target: "EP", Required: false, Note: "HTTP/3 over QUIC", Purpose: "udp_exposure"},
				},
				// ── L7 应用 ──
				"http": {
					{Action: "route", Target: "EP", Required: true, Note: "on :80 (plain) + :443 (after TLS)"},
				},
				"grpc": {
					{Action: "route", Target: "EP", Required: false, Note: "on :80 (h2c) + :443 (h2)"},
				},
				"ws": {
					{Action: "upgrade", Target: "EP", Required: false, Note: "on :80 + :443"},
				},
				"connect": nil, // Caddy does not support CONNECT tunnel
			},
		},
	},
}

// RuntimeModeEdgeMux — HAProxy :443 SNI + Caddy :80 HTTP + Caddy :8443 internal TLS.
// HAProxy reads SNI from ClientHello: if the domain needs TLS termination, it forwards
// to Caddy :8443; if the domain is a passthrough route, it forwards directly to the EP.
// Caddy handles HTTP/1.1-2 on :80 and internal TLS termination on :8443.
var RuntimeModeEdgeMux = RuntimeMode{
	ID:          "edge_mux",
	Label:       "EdgeMux",
	Description: "HAProxy :443 SNI → Caddy :8443 TLS 终止 + Caddy :80 HTTP",
	Implemented: true,
	Atoms:       AllAtomsInDisplayOrder(),
	Compositions: buildCompositions("edge_mux"),
	Providers: []ProviderAtoms{
		{
			ProviderID: "caddy",
			Bindings: map[string][]AtomSlot{
				// ── L4 地基 ──
				"tcp": {
					{Port: 80, Protocol: "tcp", ListeningAt: "public :80/tcp", Action: "listen", Target: "EP", Required: true, Note: "HTTP/gRPC", Purpose: "http"},
					{Port: 8443, Protocol: "tcp", ListeningAt: "internal :8443/tcp", Action: "listen", Target: "L7", Required: true, Note: "from HAProxy SNI", Purpose: "internal_https"},
				},
				"udp": {
					{Port: 443, Protocol: "udp", ListeningAt: "public :443/udp", Action: "listen", Target: "EP", Required: false, Note: "QUIC/HTTP3", Purpose: "udp_exposure"},
				},
				// ── L5 安全 ──
				"tls": {
					{Port: 8443, Protocol: "tcp", ListeningAt: "internal :8443/tcp", Action: "decrypt", Target: "L7", Required: true, Note: "TLS → HTTP/gRPC", Purpose: "internal_https"},
				},
				"sni": nil, // Caddy does not do SNI — HAProxy does
				"quic": {
					{Port: 443, Protocol: "udp", ListeningAt: "public :443/udp", Action: "terminate", Target: "EP", Required: false, Note: "HTTP/3 over QUIC", Purpose: "udp_exposure"},
				},
				// ── L7 应用 ──
				"http": {
					{Action: "route", Target: "EP", Required: true, Note: "on :80 (plain) + :8443 (after TLS)"},
				},
				"grpc": {
					{Action: "route", Target: "EP", Required: false, Note: "on :80 + :8443"},
				},
				"ws": {
					{Action: "upgrade", Target: "EP", Required: false, Note: "on :80 + :8443"},
				},
				"connect": nil,
			},
		},
		{
			ProviderID: "haproxy",
			Bindings: map[string][]AtomSlot{
				// ── L4 地基 ──
				"tcp": {
					{Port: 443, Protocol: "tcp", ListeningAt: "public :443/tcp", Action: "listen", Target: "SNI", Required: true, Note: "SNI → Caddy:8443 / EP", Purpose: "tls_sni_mux"},
				},
				"udp": nil,
				// ── L5 安全 ──
				"tls":  nil, // HAProxy does not terminate TLS in EdgeMux
				"sni": {
					{Port: 443, Protocol: "tcp", ListeningAt: "public :443/tcp", Action: "preread", Target: "Caddy :8443 / EP", Required: true, Note: "TLS SNI 分流", Purpose: "tls_sni_mux"},
				},
				"quic": nil,
				// ── L7 应用 ──
				"http":    nil,
				"grpc":    nil,
				"ws":      nil,
				"connect": nil,
			},
		},
	},
}

// AllRuntimeModes returns every defined mode. The API endpoint returns this
// list so the frontend can populate the mode switcher without hand-coded data.
// Only modes with Implemented=true are actionable; the rest are greyed out.
func AllRuntimeModes() []RuntimeMode {
	return []RuntimeMode{
		RuntimeModeLegacy,
		RuntimeModeEdgeMux,
	}
}

// ============================================================================
// Mode detection — replaces shell-based CurrentPortPolicyMode()
// ============================================================================

// DetectRuntimeMode determines the active mode by checking which RuntimeMode's
// required providers are fully healthy (installed + running + no errors).
//
// Modes are tried in priority order (most feature-rich first). The first mode
// whose participating providers are all healthy wins. If no mode is fully
// satisfied, Legacy is the default fallback.
//
// This replaces the old CurrentPortPolicyMode() which used exec.LookPath +
// systemctl is-active — a separate detection path from the Planner's template
// matching. Now both the Planner and the API use the same function.
func DetectRuntimeMode(states []ProviderState) RuntimeMode {
	for _, mode := range AllRuntimeModes() {
		if !mode.Implemented {
			continue
		}
		if mode.isSatisfiedBy(states) {
			return mode
		}
	}
	// Fallback: return Legacy (always works if Caddy is installed)
	return RuntimeModeLegacy
}

// isSatisfiedBy returns true if all providers in this mode have a healthy state.
func (m RuntimeMode) isSatisfiedBy(states []ProviderState) bool {
	for _, p := range m.Providers {
		found := false
		for _, s := range states {
			if s.ID == p.ProviderID && s.Healthy() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
