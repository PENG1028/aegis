package templates

import (
	"fmt"

	"aegis/internal/hostdep/provider"
	"aegis/internal/topology"
)

// HAProxyCaddy splits traffic: HAProxy handles TLS SNI passthrough on :443,
// Caddy handles HTTP on :80 and internal HTTPS on :8443.
// Used when both TLS-terminated HTTP and TLS-passthrough TCP are needed.
type HAProxyCaddy struct{}

func (t *HAProxyCaddy) Name() string { return "haproxy_caddy" }
func (t *HAProxyCaddy) Description() string {
	return "HAProxy + Caddy: HAProxy :443 SNI → Caddy :8443, Caddy :80 HTTP"
}

func (t *HAProxyCaddy) RequiredCapabilities() []provider.Capability {
	return []provider.Capability{
		// HAProxy capabilities
		provider.CapListenTCP,
		provider.CapUpstreamTCP,
		provider.CapTLSPassthrough,
		provider.CapSNIPreread,
		provider.CapRawTCP,
		// Caddy capabilities
		provider.CapTLSTerminate,
		provider.CapHTTP1,
		provider.CapRouteHost,
		provider.CapAutoCert,
		provider.CapHotReload,
	}
}

func (t *HAProxyCaddy) BuildPlan(intents []topology.RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode) (*topology.TopologyPlan, error) {
	haproxy := findProvider(available, provider.CapSNIPreread, provider.CapTLSPassthrough)
	caddy := findProvider(available, provider.CapTLSTerminate, provider.CapRouteHost)

	if haproxy == nil || caddy == nil {
		return nil, fmt.Errorf("haproxy_caddy: need both HAProxy (SNI) and Caddy (HTTP termination)")
	}

	// Split intents: passthrough → HAProxy, terminate → Caddy
	var haproxyRoutes []provider.RouteSpec
	var caddyRoutes []provider.RouteSpec

	for _, ri := range intents {
		rs := topology.RouteIntentToRouteSpec(ri)
		if ri.TLSMode == "passthrough" {
			// HAProxy SNI passthrough: match by SNI, forward to target
			rs.Match = provider.MatchSpec{SNI: ri.Domain}
			haproxyRoutes = append(haproxyRoutes, rs)
		} else {
			// Caddy HTTP termination
			if ri.TLSMode == "" {
				rs.TLSMode = "terminate"
			}
			caddyRoutes = append(caddyRoutes, rs)
		}
	}

	// Listener specs come from RuntimeMode — no hardcoded port numbers
	haproxyListeners := mode.ListenerSpecsFor("haproxy")
	caddyListeners := mode.ListenerSpecsFor("caddy")

	plans := map[string]provider.Plan{
		haproxy.ID: topology.BuildPlan(haproxyListeners, haproxyRoutes, nil),
		caddy.ID:   topology.BuildPlan(caddyListeners, caddyRoutes, nil),
	}

	return &topology.TopologyPlan{
		Primary: topology.Solution{
			TemplateName: t.Name(),
			Level:        0,
			Description:  t.Description(),
			Providers:    []string{haproxy.ID, caddy.ID},
		},
		Plans: plans,
	}, nil
}
