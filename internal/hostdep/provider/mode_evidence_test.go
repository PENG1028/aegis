package provider

import (
	"slices"
	"testing"
)

// healthyStates builds provider states that DetectRuntimeMode treats as
// participating.
func healthyStates(ids ...string) []ProviderState {
	out := make([]ProviderState, 0, len(ids))
	for _, id := range ids {
		out = append(out, ProviderState{
			ID: id, Name: id, Installed: true, Running: true, Status: "ready", Ready: true,
		})
	}
	return out
}

// probeSet returns a PortProbe where exactly the listed ports are listening.
func probeSet(ports ...int) PortProbe {
	return func(port int) bool { return slices.Contains(ports, port) }
}

// modePorts collects every non-zero port a mode binds.
func modePorts(m RuntimeMode) []int {
	var out []int
	for _, pa := range m.Providers {
		for _, slots := range pa.Bindings {
			for _, s := range slots {
				if s.Port > 0 && !slices.Contains(out, s.Port) {
					out = append(out, s.Port)
				}
			}
		}
	}
	return out
}

// TestEvidenceConsistentWhenPortsMatchDetectedMode is the baseline: liveness and
// ports agree, so nothing is reported.
func TestEvidenceConsistentWhenPortsMatchDetectedMode(t *testing.T) {
	for _, mode := range AllRuntimeModes() {
		if !mode.Implemented {
			continue
		}
		t.Run(mode.ID, func(t *testing.T) {
			states := healthyStates(mode.ProviderIDs()...)
			detected, ev := DetectRuntimeModeWithEvidence(states, probeSet(modePorts(mode)...))
			if detected.ID != mode.ID {
				t.Skipf("harness: detection picked %s for %s's provider set", detected.ID, mode.ID)
			}
			if !ev.Consistent {
				t.Errorf("expected consistent evidence, got mismatches: %#v", ev.Mismatches)
			}
			if ev.Summary() != "" {
				t.Errorf("consistent evidence should have an empty summary, got %q", ev.Summary())
			}
		})
	}
}

// TestEvidenceCatchesLeftoverListenerFromOtherMode covers the failure this whole
// file exists for.
//
// After edge_mux → legacy, a haproxy that systemd restarted keeps :443. Liveness
// detection then reports edge_mux again, because both providers look healthy, and
// every later plan targets a mode the operator already left. DetectDrift compares
// route sets only, so nothing else notices.
func TestEvidenceCatchesLeftoverListenerFromOtherMode(t *testing.T) {
	// Legacy is detected (only caddy healthy), but edge_mux's internal port is
	// occupied — the leftover signature.
	legacyPorts := modePorts(RuntimeModeLegacy)
	var edgeOnly int
	for _, p := range modePorts(RuntimeModeEdgeMux) {
		if !slices.Contains(legacyPorts, p) {
			edgeOnly = p
			break
		}
	}
	if edgeOnly == 0 {
		t.Skip("edge_mux binds no port that legacy does not; nothing to detect")
	}

	states := healthyStates("caddy")
	detected, ev := DetectRuntimeModeWithEvidence(states, probeSet(append(legacyPorts, edgeOnly)...))
	if detected.ID != RuntimeModeLegacy.ID {
		t.Fatalf("expected legacy from a caddy-only state set, got %s", detected.ID)
	}
	if ev.Consistent {
		t.Fatalf("a listener on :%d should contradict legacy", edgeOnly)
	}

	var found bool
	for _, m := range ev.Mismatches {
		if m.Port == edgeOnly && m.Actual && !m.Expected {
			found = true
		}
	}
	if !found {
		t.Errorf("no mismatch reported for the occupied port :%d; got %#v", edgeOnly, ev.Mismatches)
	}
	if ev.Summary() == "" {
		t.Error("inconsistent evidence must produce an operator-facing summary")
	}
}

// TestEvidenceCatchesMissingListener covers the inverse: the mode is believed
// active but its port is dead, so traffic is going nowhere.
func TestEvidenceCatchesMissingListener(t *testing.T) {
	states := healthyStates("caddy")
	detected, ev := DetectRuntimeModeWithEvidence(states, probeSet() /* nothing listening */)
	if detected.ID != RuntimeModeLegacy.ID {
		t.Fatalf("expected legacy, got %s", detected.ID)
	}
	if ev.Consistent {
		t.Fatal("no listeners at all must contradict any detected mode")
	}
	for _, m := range ev.Mismatches {
		if m.Expected && !m.Actual {
			return
		}
	}
	t.Errorf("expected an expected-but-absent mismatch; got %#v", ev.Mismatches)
}

// TestNilProbeYieldsNoVerdict pins that absence of evidence is not treated as
// counter-evidence — callers that cannot probe must not see a false conflict.
func TestNilProbeYieldsNoVerdict(t *testing.T) {
	_, ev := DetectRuntimeModeWithEvidence(healthyStates("caddy"), nil)
	if ev.Probed {
		t.Error("Probed should be false with a nil probe")
	}
	if !ev.Consistent {
		t.Error("a nil probe must not report an inconsistency")
	}
	if len(ev.Mismatches) != 0 {
		t.Errorf("expected no mismatches, got %#v", ev.Mismatches)
	}
}

// TestEvidenceDoesNotChangeTheVerdict guards the separation of concerns: evidence
// reports, it never overrides. DetectRuntimeMode remains the single source of the
// mode decision, so the planner and the API cannot disagree about which mode is
// current.
func TestEvidenceDoesNotChangeTheVerdict(t *testing.T) {
	states := healthyStates("caddy", "haproxy")
	pure := DetectRuntimeMode(states)

	for _, probe := range []PortProbe{
		probeSet(),
		probeSet(80),
		probeSet(modePorts(RuntimeModeLegacy)...),
		probeSet(modePorts(RuntimeModeEdgeMux)...),
	} {
		got, _ := DetectRuntimeModeWithEvidence(states, probe)
		if got.ID != pure.ID {
			t.Errorf("evidence changed the verdict: %s vs pure %s", got.ID, pure.ID)
		}
	}
}
