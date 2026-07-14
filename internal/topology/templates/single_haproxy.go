package templates

import (
	"fmt"

	"aegis/internal/hostdep/provider"
	"aegis/internal/topology"
)

// SingleHAProxy handles all traffic through HAProxy alone.
// Best for TCP/TLS-heavy workloads where HTTP routing is not needed,
// or when Caddy is unavailable.
type SingleHAProxy struct{}

func (t *SingleHAProxy) Name() string { return "single_haproxy" }
func (t *SingleHAProxy) Description() string {
	return "Single HAProxy: :443 TLS SNI passthrough + TCP forwarding"
}

func (t *SingleHAProxy) RequiredCapabilities() []provider.Capability {
	return []provider.Capability{
		provider.CapListenTCP,
		provider.CapUpstreamTCP,
		provider.CapTLSPassthrough,
		provider.CapSNIPreread,
		provider.CapRawTCP,
		provider.CapHotReload,
		provider.CapValidateConfig,
	}
}

func (t *SingleHAProxy) BuildPlan(intents []topology.RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode) (*topology.TopologyPlan, error) {
	haproxy := findProvider(available, provider.CapSNIPreread, provider.CapTLSPassthrough)
	if haproxy == nil {
		return nil, fmt.Errorf("single_haproxy: no HAProxy provider available")
	}

	var routes []provider.RouteSpec
	for _, ri := range intents {
		rs := topology.RouteIntentToRouteSpec(ri)
		// HAProxy in single mode: all TLS traffic is SNI-passthrough
		if ri.TLSMode == "" || ri.TLSMode == "none" {
			rs.TLSMode = "passthrough"
		}
		rs.Match = provider.MatchSpec{SNI: ri.Domain} // SNI-based routing
		routes = append(routes, rs)
	}

	listeners := mode.ListenerSpecsFor("haproxy")

	plan := topology.BuildPlan(listeners, routes, nil)
	return &topology.TopologyPlan{
		Primary: topology.Solution{
			TemplateName: t.Name(),
			Level:        0,
			Description:  t.Description(),
			Providers:    []string{haproxy.ID},
		},
		Plans: map[string]provider.Plan{
			haproxy.ID: plan,
		},
		Warnings: []string{
			"single HAProxy mode — no HTTP host/path routing, no auto TLS certs",
		},
	}, nil
}
