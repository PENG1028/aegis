package provider

import "testing"

func healthyState(id string) ProviderState {
	return ProviderState{ID: id, Status: "ready", Installed: true, Running: true}
}

func TestDetectRuntimeMode_PrefersRicherMode(t *testing.T) {
	// When BOTH Legacy AND EdgeMux are satisfied, EdgeMux should win because
	// it has more providers (2 vs 1). This verifies the "richer mode" design,
	// not just "first match wins".
	states := []ProviderState{
		healthyState("caddy"),
		healthyState("haproxy"),
	}
	got := DetectRuntimeMode(states)
	if got.ID != RuntimeModeEdgeMux.ID {
		t.Fatalf("both modes satisfied → expected EdgeMux (2 providers), got %s", got.ID)
	}
}

func TestDetectRuntimeMode_LegacyOnly(t *testing.T) {
	// Only Caddy → Legacy
	states := []ProviderState{
		healthyState("caddy"),
	}
	got := DetectRuntimeMode(states)
	if got.ID != RuntimeModeLegacy.ID {
		t.Fatalf("only Caddy → expected Legacy, got %s", got.ID)
	}
}

func TestDetectRuntimeMode_NoProviders(t *testing.T) {
	// No providers at all → fallback to Legacy
	got := DetectRuntimeMode(nil)
	if got.ID != RuntimeModeLegacy.ID {
		t.Fatalf("no providers → expected Legacy fallback, got %s", got.ID)
	}
}

func TestDetectRuntimeMode_EdgeMuxNotSatisfied(t *testing.T) {
	// Caddy healthy but HAProxy not installed → only Legacy satisfied
	states := []ProviderState{
		healthyState("caddy"),
		{ID: "haproxy", Status: "error", Installed: false, Running: false},
	}
	got := DetectRuntimeMode(states)
	if got.ID != RuntimeModeLegacy.ID {
		t.Fatalf("HAProxy unhealthy → expected Legacy, got %s", got.ID)
	}
}
