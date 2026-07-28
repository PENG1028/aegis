package handlers

import (
	"testing"

	"aegis/internal/hostdep/provider"
)

func TestProviderCapabilityInstancesCombineRuntimeAndLifecycleState(t *testing.T) {
	instances := providerCapabilityInstances(provider.ProviderState{
		ID: "executor-a", Installed: true, Running: true,
		Capabilities: []provider.Capability{provider.CapAutoCert, provider.CapLoadCert},
	})
	if len(instances) != 2 || instances[0].Availability != "ready" {
		t.Fatalf("unexpected capability instances: %+v", instances)
	}
	if instances[0].Semantics.StateClass != provider.StateProviderManaged {
		t.Fatalf("automatic TLS lifecycle state missing: %+v", instances[0])
	}
	if instances[1].Semantics.StateClass != provider.StatePortableAsset {
		t.Fatalf("portable asset lifecycle state missing: %+v", instances[1])
	}
}
