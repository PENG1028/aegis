package topology

import (
	"testing"

	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
)

func TestOrdinaryPlanDoesNotReplaceAutomaticTLSExecutor(t *testing.T) {
	intents := []RouteIntent{{
		Domain: "app.example.com", TLSBindingMode: route.TLSBindingProviderAuto, TLSExecutor: "executor-a",
	}}
	mode := provider.RuntimeMode{
		ID: "test", Providers: []provider.ProviderAtoms{{ProviderID: "executor-b"}},
	}
	available := []provider.ProviderState{{
		ID: "executor-b", Installed: true, Capabilities: []provider.Capability{provider.CapAutoCert},
	}}
	if err := validateExecutionAffinity(intents, available, mode, false); err == nil {
		t.Fatal("ordinary planning silently replaced the bound automatic TLS executor")
	}
	if err := validateExecutionAffinity(intents, available, mode, true); err != nil {
		t.Fatalf("explicit migration should recreate automatic TLS: %v", err)
	}
}

func TestOrdinaryPlanKeepsAvailableAutomaticTLSExecutor(t *testing.T) {
	intents := []RouteIntent{{
		Domain: "app.example.com", TLSBindingMode: route.TLSBindingProviderAuto, TLSExecutor: "executor-a",
	}}
	mode := provider.RuntimeMode{ID: "test", Providers: []provider.ProviderAtoms{{ProviderID: "executor-a"}}}
	available := []provider.ProviderState{{
		ID: "executor-a", Installed: true, Capabilities: []provider.Capability{provider.CapAutoCert},
	}}
	if err := validateExecutionAffinity(intents, available, mode, false); err != nil {
		t.Fatalf("bound executor was rejected: %v", err)
	}
}
