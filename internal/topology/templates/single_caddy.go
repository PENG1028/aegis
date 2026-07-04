package templates

import (
	"fmt"

	"aegis/internal/provider"
	"aegis/internal/topology"
)

// SingleCaddy handles all HTTP/HTTPS routes with Caddy alone.
// Best for simple setups: Caddy on :80 + :443, TLS termination, auto certs.
type SingleCaddy struct{}

func (t *SingleCaddy) Name() string        { return "single_caddy" }
func (t *SingleCaddy) Description() string { return "Single Caddy: :80 HTTP + :443 HTTPS termination" }

func (t *SingleCaddy) RequiredCapabilities() []provider.Capability {
	return []provider.Capability{
		provider.CapListenTCP,
		provider.CapUpstreamTCP,
		provider.CapTLSTerminate,
		provider.CapHTTP1,
		provider.CapRouteHost,
		provider.CapHotReload,
		provider.CapValidateConfig,
	}
}

func (t *SingleCaddy) BuildPlan(intents []topology.RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode) (*topology.TopologyPlan, error) {
	caddy := findProvider(available, provider.CapListenTCP, provider.CapTLSTerminate, provider.CapRouteHost)
	if caddy == nil {
		return nil, fmt.Errorf("single_caddy: no provider with HTTP termination capability")
	}

	// Build routes for Caddy
	var routes []provider.RouteSpec
	for _, ri := range intents {
		rs := topology.RouteIntentToRouteSpec(ri)
		// All routes go to Caddy — set TLS mode appropriately
		if ri.TLSMode == "" && ri.Port == 443 {
			rs.TLSMode = "terminate"
		}
		routes = append(routes, rs)
	}

	// Caddy listeners come from RuntimeMode — no more hardcoded ports
	listeners := mode.ListenerSpecsFor("caddy")

	plan := topology.BuildPlan(listeners, routes, nil)
	return &topology.TopologyPlan{
		Primary: topology.Solution{
			TemplateName: t.Name(),
			Level:        0,
			Description:  t.Description(),
			Providers:    []string{caddy.ID},
		},
		Plans: map[string]provider.Plan{
			caddy.ID: plan,
		},
	}, nil
}
