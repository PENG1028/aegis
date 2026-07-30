package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/hostdep/provider"
)

func listProviders(t *testing.T, states ...provider.ProviderState) []map[string]any {
	t.Helper()
	reg := provider.NewRegistry()
	for _, st := range states {
		reg.Register(&modeTLSProvider{state: st})
	}
	h := &Handlers{ProvReg: reg}
	rec := httptest.NewRecorder()
	h.ListProviders(rec, httptest.NewRequest(http.MethodGet, "/api/admin/v1/providers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Providers
}

// TestCapabilityStatusFieldNames pins the JSON keys on capability_statuses.
//
// An r7 verification report filed these entries as broken, having read a "state"
// field that does not exist and getting null back. The keys are key, availability,
// and semantics — ui/src/lib/provider-capability.ts reads availability, and a rename
// here would make capabilityIsReady() silently return false for every capability,
// greying out every action in the UI with no error anywhere.
func TestCapabilityStatusFieldNames(t *testing.T) {
	providers := listProviders(t, provider.ProviderState{
		ID: "caddy", Name: "Caddy", Installed: true, Running: true, Status: "ready",
		Capabilities: []provider.Capability{provider.CapAutoCert, provider.CapLoadCert},
	})
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}

	raw, ok := providers[0]["capability_statuses"].([]any)
	if !ok {
		t.Fatalf("capability_statuses missing or not an array: %#v", providers[0]["capability_statuses"])
	}
	if len(raw) != 2 {
		t.Fatalf("expected one entry per declared capability, got %d", len(raw))
	}

	entry, ok := raw[0].(map[string]any)
	if !ok {
		t.Fatalf("entry is not an object: %#v", raw[0])
	}
	for _, field := range []string{"key", "availability", "semantics"} {
		if _, present := entry[field]; !present {
			t.Errorf("field %q absent — a consumer reading it gets null and cannot tell "+
				"that from a real value; keys present: %v", field, keysOf(entry))
		}
	}
	if _, unexpected := entry["state"]; unexpected {
		t.Error(`a "state" field appeared; consumers read "availability" — having both invites reading the wrong one`)
	}
}

// TestCapabilityAvailabilityIsNeverBlank covers every branch of the fallback. A blank
// availability would fail the UI's `=== 'ready'` check and disable actions for a
// provider that is actually healthy, which is indistinguishable from a real outage.
func TestCapabilityAvailabilityIsNeverBlank(t *testing.T) {
	cases := map[string]struct {
		state provider.ProviderState
		want  string
	}{
		"explicit status wins": {
			provider.ProviderState{ID: "a", Installed: true, Running: true, Status: "ready",
				Capabilities: []provider.Capability{provider.CapAutoCert}}, "ready"},
		"blank status, healthy": {
			provider.ProviderState{ID: "b", Installed: true, Running: true,
				Capabilities: []provider.Capability{provider.CapAutoCert}}, "ready"},
		"blank status, not installed": {
			provider.ProviderState{ID: "c", Installed: false,
				Capabilities: []provider.Capability{provider.CapAutoCert}}, "unavailable"},
		"blank status, installed but stopped": {
			provider.ProviderState{ID: "d", Installed: true, Running: false,
				Capabilities: []provider.Capability{provider.CapAutoCert}}, "degraded"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			providers := listProviders(t, tc.state)
			raw := providers[0]["capability_statuses"].([]any)
			entry := raw[0].(map[string]any)
			got, _ := entry["availability"].(string)
			if got == "" {
				t.Fatal("availability is blank — the UI would disable actions for this provider")
			}
			if got != tc.want {
				t.Errorf("availability = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestProviderWithNoCapabilitiesYieldsEmptyArray pins that the field is an empty
// array rather than null. A JSON null breaks `?.find(...)` chains differently than an
// empty array does, and this is the shape a provider that failed detection produces.
func TestProviderWithNoCapabilitiesYieldsEmptyArray(t *testing.T) {
	providers := listProviders(t, provider.ProviderState{
		ID: "bare", Name: "bare", Installed: true, Running: true, Status: "ready",
	})
	raw, ok := providers[0]["capability_statuses"].([]any)
	if !ok {
		t.Fatalf("capability_statuses should be an array even with no capabilities, got %#v",
			providers[0]["capability_statuses"])
	}
	if len(raw) != 0 {
		t.Errorf("expected an empty array, got %d entries", len(raw))
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
