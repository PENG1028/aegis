package topology

import "aegis/internal/hostdep/provider"

// ============================================================================
// TopologyPlan — planner output
// ============================================================================

// TopologyPlan is the complete output of the topology Planner.
// It assigns routes to providers and describes alternative configurations.
type TopologyPlan struct {
	// Primary is the selected (best) solution.
	Primary Solution `json:"primary"`

	// Alternatives are fallback options, sorted by level (best first).
	// Empty if no alternatives exist.
	Alternatives []Solution `json:"alternatives,omitempty"`

	// ForwardTarget is set when transparent proxy is active on this node.
	// It tells the transparent proxy manager where to forward intercepted traffic.
	// nil if transparent proxy is not in use.
	ForwardTarget *provider.ForwardTarget `json:"forward_target,omitempty"`

	// Plans maps provider ID → Plan. Each Plan contains the listeners and
	// routes that provider should render.
	Plans map[string]provider.Plan `json:"plans"`

	// Warnings are non-blocking issues found during planning.
	Warnings []string `json:"warnings,omitempty"`
}

// Solution describes one way to route a set of intents using specific providers.
type Solution struct {
	// TemplateName identifies the topology template used.
	// e.g. "single_caddy", "haproxy_caddy", "dedicated_ports"
	TemplateName string `json:"template_name"`

	// Level is the fallback level (0 = best, 3 = impossible).
	Level int `json:"level"`

	// Description is a human-readable summary of this solution.
	Description string `json:"description"`

	// Providers is the list of provider IDs involved in this solution.
	Providers []string `json:"providers"`

	// Warnings are solution-specific issues.
	Warnings []string `json:"warnings,omitempty"`
}

// BuildPlan constructs a provider.Plan for a single provider from a set of
// route intents and listeners. This is a helper that templates use to build
// the Plans map in TopologyPlan.
func BuildPlan(listeners []provider.ListenerSpec, routes []provider.RouteSpec, forwardTarget *provider.ForwardTarget) provider.Plan {
	return provider.Plan{
		Listeners:     listeners,
		Routes:        routes,
		ForwardTarget: forwardTarget,
	}
}

// RouteIntentToRouteSpec converts a RouteIntent to a provider.RouteSpec.
// This is called by templates when building per-provider Plans.
func RouteIntentToRouteSpec(ri RouteIntent) provider.RouteSpec {
	return provider.RouteSpec{
		Transport:   ri.Transport,
		TLSMode:     ri.TLSMode,
		Match: provider.MatchSpec{
			Host: ri.Domain,
			Path: ri.Path,
		},
		AppProtocol:        ri.AppProtocol,
		HTTPVersion:        ri.HTTPVersion,
		Upstream: provider.UpstreamSpec{
			Type:   upstreamType(ri.Upstream, ri.Transport),
			Target: ri.Upstream,
		},
		CertID:             ri.CertID,
		CertPath:           ri.CertPath,
		KeyPath:            ri.KeyPath,
		ExtraHeaders:       ri.ExtraHeaders,
		StripPathPrefix:    ri.StripPathPrefix,
		MaintenanceEnabled: ri.MaintenanceEnabled,
		MaintenanceMessage: ri.MaintenanceMessage,
	}
}

// upstreamType infers the upstream type from the upstream address.
func upstreamType(upstream, transport string) string {
	if len(upstream) >= 7 && upstream[:7] == "unix://" {
		return "unix"
	}
	if len(upstream) >= 7 && upstream[:7] == "http://" {
		return "http"
	}
	if len(upstream) >= 8 && upstream[:8] == "https://" {
		return "http"
	}
	if transport == "udp" {
		return "udp"
	}
	return "tcp"
}
