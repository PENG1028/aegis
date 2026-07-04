package apply

import (
	"context"
	"testing"

	"aegis/internal/endpoint"
	"aegis/internal/proxy"
)

func TestFakeProxyAdapter(t *testing.T) {
	adapter := proxy.NewFakeAdapter()

	// Render
	cfg := proxy.GatewayConfig{
		Routes: []proxy.RouteConfig{
			{Domain: "test.example.com", UpstreamURL: "http://127.0.0.1:3001", Kind: "reverse_proxy"},
		},
	}
	result, err := adapter.Render(cfg)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected rendered output")
	}

	// Validate success
	err = adapter.Validate("/tmp/test")
	if err != nil {
		t.Errorf("validate should succeed: %v", err)
	}

	// Validate failure
	adapter.ValidateShouldFail = true
	err = adapter.Validate("/tmp/test")
	if err == nil {
		t.Error("validate should fail")
	}

	// Reload
	adapter.ValidateShouldFail = false
	err = adapter.Reload("")
	if err != nil {
		t.Errorf("reload should succeed: %v", err)
	}
	if adapter.ReloadCallCount != 1 {
		t.Errorf("reload count = %d, want 1", adapter.ReloadCallCount)
	}
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
