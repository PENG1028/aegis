package apply

import (
	"context"
	"testing"

	"aegis/internal/endpoint"
	"aegis/internal/fake"
	"aegis/internal/provider"
)

func TestFakeProvider(t *testing.T) {
	fp := fake.NewFakeProvider("caddy", "http")

	// State
	state := fp.State()
	if state.ID != "caddy" {
		t.Errorf("expected provider ID 'caddy', got %q", state.ID)
	}
	if state.Status != "ready" {
		t.Errorf("expected status 'ready', got %q", state.Status)
	}

	// Diagnose
	diag := fp.Diagnose()
	if diag.Provider != "caddy" {
		t.Errorf("expected provider 'caddy', got %q", diag.Provider)
	}

	// Render
	plan := provider.Plan{
		Routes: []provider.RouteSpec{
			{
				Match:    provider.MatchSpec{Host: "test.example.com"},
				Upstream: provider.UpstreamSpec{Target: "http://127.0.0.1:3001"},
			},
		},
	}
	configs, err := fp.Render(plan)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(configs) == 0 {
		t.Error("expected rendered output")
	}

	// Apply success
	err = fp.Apply(configs)
	if err != nil {
		t.Errorf("apply should succeed: %v", err)
	}

	// Apply failure (validation)
	fp.FailValidate = true
	fp.ValidateErr = "syntax error at line 1"
	err = fp.Apply(configs)
	if err == nil {
		t.Error("apply should fail with validation error")
	}
	fp.ResetErrors()

	// Apply failure (reload)
	fp.FailReload = true
	err = fp.Apply(configs)
	if err == nil {
		t.Error("apply should fail with reload error")
	}
	fp.ResetErrors()
}

// TestApplyPlanStructure verifies the ApplyPlan/ApplyWarning types are usable.
func TestApplyPlanStructure(t *testing.T) {
	plan := ApplyPlan{
		RouteCount:         1,
		ManagedDomainCount: 0,
		SkippedCount:       2,
		Warnings: []ApplyWarning{
			{Code: WarningServiceDisabled, Severity: "warning", Message: "test", Target: "svc_1"},
			{Code: WarningNoAvailableEndpoint, Severity: "critical", Message: "test", Target: "svc_2"},
		},
	}

	if plan.RouteCount != 1 {
		t.Error("route count mismatch")
	}
	if len(plan.Warnings) != 2 {
		t.Error("warnings count mismatch")
	}
}

func TestAddressNormalization(t *testing.T) {
	ctx := context.Background()
	_ = ctx

	tests := []struct {
		in  string
		out string
	}{
		{"127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"http://127.0.0.1:3001", "http://127.0.0.1:3001"},
		{"https://example.com:443", "https://example.com:443"},
	}

	for _, tt := range tests {
		normalized := endpoint.NormalizeAddress(tt.in)
		if normalized != tt.out {
			t.Errorf("NormalizeAddress(%q) = %q, want %q", tt.in, normalized, tt.out)
		}
	}
}
