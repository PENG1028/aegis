package templates

import (
	"fmt"

	"aegis/internal/provider"
	"aegis/internal/topology"
)

// DedicatedPorts is a Level 2 fallback: TCP traffic goes on dedicated ports
// when SNI preread is not available. Each TCP service gets its own port.
type DedicatedPorts struct{}

func (t *DedicatedPorts) Name() string { return "dedicated_ports" }
func (t *DedicatedPorts) Description() string {
	return "Dedicated Ports: TCP services on separate ports (no SNI multiplexing)"
}

func (t *DedicatedPorts) RequiredCapabilities() []provider.Capability {
	return []provider.Capability{
		provider.CapListenTCP,
		provider.CapUpstreamTCP,
		provider.CapRawTCP,
		provider.CapHTTP1,
		provider.CapRouteHost,
	}
}

func (t *DedicatedPorts) BuildPlan(intents []topology.RouteIntent, available []provider.ProviderState, mode provider.RuntimeMode) (*topology.TopologyPlan, error) {
	// Use Caddy for HTTP if available, otherwise use HAProxy for raw TCP
	httpProvider := findProvider(available, provider.CapTLSTerminate, provider.CapRouteHost)
	tcpProvider := findProvider(available, provider.CapListenTCP, provider.CapRawTCP)

	if httpProvider == nil && tcpProvider == nil {
		return nil, fmt.Errorf("dedicated_ports: no suitable provider available")
	}

	plans := make(map[string]provider.Plan)
	var providers []string

	// Separate HTTP and raw TCP intents
	var httpRoutes []provider.RouteSpec
	var tcpRoutes []provider.RouteSpec
	var httpListeners []provider.ListenerSpec
	var tcpListeners []provider.ListenerSpec
	nextPort := 8080 // start of dedicated port range

	for _, ri := range intents {
		rs := topology.RouteIntentToRouteSpec(ri)
		if ri.AppProtocol == "raw" || ri.TLSMode == "passthrough" {
			// Raw TCP: assign a dedicated port
			rs.Match = provider.MatchSpec{Port: nextPort}
			tcpRoutes = append(tcpRoutes, rs)
			tcpListeners = append(tcpListeners, provider.ListenerSpec{
				Port: nextPort, Protocol: "tcp", Purpose: "tcp_exposure",
			})
			nextPort++
		} else if httpProvider != nil {
			httpRoutes = append(httpRoutes, rs)
		}
	}

	// HTTP routes — listeners come from RuntimeMode
	if len(httpRoutes) > 0 && httpProvider != nil {
		httpListeners = append(httpListeners, mode.ListenerSpecsFor(httpProvider.ID)...)
		plans[httpProvider.ID] = topology.BuildPlan(httpListeners, httpRoutes, nil)
		providers = append(providers, httpProvider.ID)
	}

	// TCP routes get dedicated ports
	if len(tcpRoutes) > 0 && tcpProvider != nil {
		plans[tcpProvider.ID] = topology.BuildPlan(tcpListeners, tcpRoutes, nil)
		if tcpProvider.ID != (httpProvider.ID) { // avoid duplicate
			providers = append(providers, tcpProvider.ID)
		} else if !contains(providers, tcpProvider.ID) {
			providers = append(providers, tcpProvider.ID)
		}
	}

	warnings := []string{
		"dedicated ports mode — no SNI multiplexing on :443",
		"TCP services require unique ports; this does not scale to many services",
	}

	return &topology.TopologyPlan{
		Primary: topology.Solution{
			TemplateName: t.Name(),
			Level:        2, // degraded: no SNI multiplexing
			Description:  t.Description(),
			Providers:    providers,
			Warnings:     warnings,
		},
		Plans:    plans,
		Warnings: warnings,
	}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
