package templates

import (
	"testing"

	"aegis/internal/hostdep/provider"
	"aegis/internal/topology"
)

func rawTCPIntent(domain string) topology.RouteIntent {
	return topology.RouteIntent{
		Domain:      domain,
		Port:        3306,
		Transport:   "tcp",
		AppProtocol: "raw",
		Composition: "raw_tcp",
	}
}

// TestSingleCaddyRejectsRawTCP is a regression test: a raw TCP route (SSH,
// MySQL, …) used to be silently rendered as an HTTP site on Caddy — the
// traffic went to the wrong protocol handler and the service "worked" for
// nobody. Templates that cannot carry raw traffic must fail loudly instead.
func TestSingleCaddyRejectsRawTCP(t *testing.T) {
	tpl := &SingleCaddy{}
	_, err := tpl.BuildPlan(
		[]topology.RouteIntent{rawTCPIntent("db.internal")},
		fakeStates(),
		provider.RuntimeMode{},
	)
	if err == nil {
		t.Fatal("single_caddy must reject raw TCP routes (it can only serve HTTP)")
	}
}

// TestHAProxyCaddyRejectsRawTCP mirrors the check for the split template.
func TestHAProxyCaddyRejectsRawTCP(t *testing.T) {
	tpl := &HAProxyCaddy{}
	_, err := tpl.BuildPlan(
		[]topology.RouteIntent{rawTCPIntent("db.internal")},
		fakeStates(),
		provider.RuntimeMode{},
	)
	if err == nil {
		t.Fatal("haproxy_caddy must reject raw TCP routes")
	}
}

// TestSingleHAProxyRejectsRawTCP: forcing raw TCP into SNI passthrough
// produces a black hole (plaintext traffic has no SNI to match).
func TestSingleHAProxyRejectsRawTCP(t *testing.T) {
	tpl := &SingleHAProxy{}
	_, err := tpl.BuildPlan(
		[]topology.RouteIntent{rawTCPIntent("db.internal")},
		fakeStates(),
		provider.RuntimeMode{},
	)
	if err == nil {
		t.Fatal("single_haproxy must reject raw TCP routes")
	}
}

// fakeStates provides provider states with the capabilities each template
// needs to get past provider lookup and reach the raw-intent check.
func fakeStates() []provider.ProviderState {
	return []provider.ProviderState{
		{
			ID:           "caddy",
			Name:         "caddy",
			Capabilities: []provider.Capability{provider.CapListenTCP, provider.CapTLSTerminate, provider.CapRouteHost, provider.CapHTTP1},
		},
		{
			ID:           "haproxy",
			Name:         "haproxy",
			Capabilities: []provider.Capability{provider.CapSNIPreread, provider.CapTLSPassthrough, provider.CapRawTCP, provider.CapListenTCP},
		},
	}
}
