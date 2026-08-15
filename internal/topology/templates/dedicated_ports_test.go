package templates

import (
	"testing"

	"aegis/internal/hostdep/provider"
	"aegis/internal/topology"
)

// TestDedicatedPortsSkipsOccupiedPorts is a regression test: the dedicated
// port allocator started at 8080 and never checked what the mode's
// providers already bind, so a raw TCP route could claim a port that an
// HTTP listener already uses — the proxy would fail to start.
func TestDedicatedPortsSkipsOccupiedPorts(t *testing.T) {
	// Mode where the HTTP provider already occupies 8080/8081.
	mode := provider.RuntimeMode{
		ID: "test",
		Providers: []provider.ProviderAtoms{
			{
				ProviderID: "caddy",
				Bindings: map[string][]provider.AtomSlot{
					"http":  {{Port: 8080}},
					"https": {{Port: 8081}},
				},
			},
		},
	}

	tpl := &DedicatedPorts{}
	plan, err := tpl.BuildPlan(
		[]topology.RouteIntent{rawTCPIntent("db1"), rawTCPIntent("db2")},
		fakeStates(),
		mode,
	)
	if err != nil {
		t.Fatal(err)
	}

	// Collect allocated TCP listener ports.
	var ports []int
	for _, p := range plan.Plans {
		for _, l := range p.Listeners {
			ports = append(ports, l.Port)
		}
	}
	if len(ports) != 2 {
		t.Fatalf("allocated %d ports, want 2: %v", len(ports), ports)
	}
	for _, p := range ports {
		if p == 8080 || p == 8081 {
			t.Fatalf("allocated port %d collides with an occupied mode port: %v", p, ports)
		}
	}
	if ports[0] != 8082 || ports[1] != 8083 {
		t.Fatalf("ports = %v, want [8082 8083] (skip occupied 8080/8081)", ports)
	}
}

// TestDedicatedPortsStableAcrossRuns: the same input must produce the same
// ports every run (route churn must not reallocate existing services).
func TestDedicatedPortsStableAcrossRuns(t *testing.T) {
	tpl := &DedicatedPorts{}
	intents := []topology.RouteIntent{rawTCPIntent("db1"), rawTCPIntent("db2")}
	mode := provider.RuntimeMode{ID: "test"}

	plan1, err := tpl.BuildPlan(intents, fakeStates(), mode)
	if err != nil {
		t.Fatal(err)
	}
	plan2, err := tpl.BuildPlan(intents, fakeStates(), mode)
	if err != nil {
		t.Fatal(err)
	}

	ports1 := collectTCPPorts(plan1)
	ports2 := collectTCPPorts(plan2)
	if len(ports1) != len(ports2) {
		t.Fatalf("port counts differ across runs: %v vs %v", ports1, ports2)
	}
	for i := range ports1 {
		if ports1[i] != ports2[i] {
			t.Fatalf("ports differ across runs: %v vs %v (route churn would break clients)", ports1, ports2)
		}
	}
}

func collectTCPPorts(plan *topology.TopologyPlan) []int {
	var ports []int
	for _, p := range plan.Plans {
		for _, l := range p.Listeners {
			if l.Protocol == "tcp" {
				ports = append(ports, l.Port)
			}
		}
	}
	return ports
}
