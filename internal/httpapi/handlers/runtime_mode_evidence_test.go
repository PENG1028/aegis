package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aegis/internal/hostdep/provider"
)

// TestRuntimeModeEndpointReportsPortEvidence pins that the endpoint an operator
// reads the mode from also reports when that verdict disagrees with reality.
//
// DetectRuntimeMode decides from process liveness alone. A haproxy that systemd
// restarted after a switch back to legacy makes it answer edge_mux, and every
// later plan targets the abandoned mode. This endpoint is the one place a human
// sees the mode, so it pays for the socket probes.
func TestRuntimeModeEndpointReportsPortEvidence(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Name: "caddy", Installed: true, Running: true, Status: "ready", Ready: true,
		Capabilities: []provider.Capability{provider.CapAutoCert, provider.CapLoadCert},
	}})
	// Nothing listening. Injected rather than dialling for real so the result does
	// not depend on what happens to be bound on the machine running the test.
	h := &Handlers{ProvReg: registry, PortProbe: func(int) bool { return false }}

	rec := httptest.NewRecorder()
	h.RuntimeMode(rec, httptest.NewRequest(http.MethodGet, "/api/system/runtime-mode", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Current  map[string]any `json:"current"`
		Evidence struct {
			Mode       string `json:"mode"`
			Consistent bool   `json:"consistent"`
			Probed     bool   `json:"probed"`
			Mismatches []struct {
				Port     int    `json:"port"`
				Expected bool   `json:"expected_listening"`
				Actual   bool   `json:"actual_listening"`
				Detail   string `json:"detail"`
			} `json:"mismatches"`
		} `json:"evidence"`
		EvidenceWarning string `json:"evidence_warning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !body.Evidence.Probed {
		t.Error("endpoint should probe ports — it is the operator-facing read of the mode")
	}
	if body.Evidence.Mode == "" {
		t.Error("evidence should name the mode it cross-checked")
	}
	if body.Current == nil {
		t.Error("current mode must still be present — evidence is additive")
	}

	// The injected probe reports nothing listening, so the detected mode's ports
	// are absent and the evidence must say so rather than silently agreeing.
	if body.Evidence.Consistent {
		t.Error("with no gateway listening, evidence should not report consistency")
	}
	if body.EvidenceWarning == "" {
		t.Error("an inconsistency must surface a human-readable warning for the UI")
	}
	var sawAbsent bool
	for _, m := range body.Evidence.Mismatches {
		if m.Expected && !m.Actual {
			sawAbsent = true
			if m.Detail == "" {
				t.Error("mismatch detail should explain which port and why")
			}
		}
	}
	if !sawAbsent {
		t.Errorf("expected an expected-but-absent port mismatch; got %+v", body.Evidence.Mismatches)
	}
}

// TestRuntimeModeEvidenceWarningNamesTheMode keeps the warning actionable: the
// operator needs to know which mode was inferred and which port contradicts it,
// not just that something is wrong.
func TestRuntimeModeEvidenceWarningNamesTheMode(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Name: "caddy", Installed: true, Running: true, Status: "ready", Ready: true,
	}})

	// The leftover-listener scenario: only caddy is healthy so detection says
	// legacy, while legacy's own ports are bound AND an edge_mux-only port is
	// still held by a haproxy nobody stopped.
	legacy := portsOf(provider.RuntimeModeLegacy)
	var edgeOnly int
	for _, p := range portsOf(provider.RuntimeModeEdgeMux) {
		if !contains(legacy, p) {
			edgeOnly = p
			break
		}
	}
	if edgeOnly == 0 {
		t.Skip("edge_mux binds no port legacy does not")
	}
	h := &Handlers{ProvReg: registry, PortProbe: func(port int) bool {
		return contains(legacy, port) || port == edgeOnly
	}}

	rec := httptest.NewRecorder()
	h.RuntimeMode(rec, httptest.NewRequest(http.MethodGet, "/api/system/runtime-mode", nil))

	var body struct {
		EvidenceWarning string `json:"evidence_warning"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.EvidenceWarning == "" {
		t.Fatal("a port held by a mode that was switched away from must produce a warning")
	}
	if !strings.Contains(body.EvidenceWarning, "legacy") && !strings.Contains(body.EvidenceWarning, "edge_mux") {
		t.Errorf("warning should name the detected mode, got %q", body.EvidenceWarning)
	}
	if !strings.Contains(body.EvidenceWarning, ":") {
		t.Errorf("warning should name the port in conflict, got %q", body.EvidenceWarning)
	}
}

func portsOf(m provider.RuntimeMode) []int {
	var out []int
	for _, pa := range m.Providers {
		for _, slots := range pa.Bindings {
			for _, s := range slots {
				if s.Port > 0 && !contains(out, s.Port) {
					out = append(out, s.Port)
				}
			}
		}
	}
	return out
}

func contains(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
