package noderuntime

import (
	"fmt"

	"aegis/internal/provider"
)

// CaddyfileApplier renders and applies a Caddyfile from a routing table.
// v1.8L: delegates to provider.CaddyProvider instead of the old proxy.ProxyAdapter.
type CaddyfileApplier interface {
	Apply(entries []RoutingTableEntry) error
}

// caddyApplier uses the new Provider interface to render + apply Caddy config.
type caddyApplier struct {
	provReg *provider.Registry
}

// NewCaddyApplier creates a Caddyfile applier backed by the provider registry.
func NewCaddyApplier(provReg *provider.Registry) CaddyfileApplier {
	return &caddyApplier{provReg: provReg}
}

// Apply converts routing table entries to a Plan, renders a Caddyfile via
// CaddyProvider, and applies the config (validate → backup → write → reload).
func (a *caddyApplier) Apply(entries []RoutingTableEntry) error {
	caddy := a.provReg.FindByCapability(provider.CapAutoCert)
	if caddy == nil {
		return fmt.Errorf("no provider with auto-cert capability registered (expected Caddy)")
	}

	routes := routingTableToRouteSpecs(entries)
	if len(routes) == 0 {
		return fmt.Errorf("no available routes to apply")
	}

	listeners := []provider.ListenerSpec{
		{Port: 80, Protocol: "tcp", Purpose: "http"},
		{Port: 443, Protocol: "tcp", Purpose: "https"},
	}
	plan := provider.Plan{Listeners: listeners, Routes: routes}

	configs, err := caddy.Render(plan)
	if err != nil {
		return fmt.Errorf("render caddyfile: %w", err)
	}

	return caddy.Apply(configs)
}

// routingTableToRouteSpecs converts routing table entries to provider.RouteSpec.
func routingTableToRouteSpecs(entries []RoutingTableEntry) []provider.RouteSpec {
	var routes []provider.RouteSpec
	for _, entry := range entries {
		if entry.Status != "available" {
			continue
		}

		var upstream string
		var extraHeaders map[string]string

		if entry.TargetNodeID == entry.FromNodeID || entry.TargetNodeID == "" {
			if entry.TargetLocalHost == "" || entry.TargetLocalPort == 0 {
				continue
			}
			upstream = fmt.Sprintf("http://%s:%d", entry.TargetLocalHost, entry.TargetLocalPort)
		} else {
			if len(entry.Candidates) == 0 {
				continue
			}
			best := entry.Candidates[0]
			upstream = best.GatewayURL

			if best.GatewayLinkID != "" {
				extraHeaders = map[string]string{
					"X-Aegis-Gateway-Link": best.GatewayLinkID,
				}
			}
		}

		routes = append(routes, provider.RouteSpec{
			Transport:    "tcp",
			TLSMode:      "terminate",
			AppProtocol:  "http",
			Match:        provider.MatchSpec{Host: entry.Domain},
			Upstream:     provider.UpstreamSpec{Type: "http", Target: upstream},
			ExtraHeaders: extraHeaders,
		})
	}
	return routes
}
