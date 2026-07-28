package provider

import "testing"

func TestTLSCapabilitySemantics(t *testing.T) {
	automatic := SemanticsOf(CapAutoCert)
	if automatic.StateClass != StateProviderManaged || automatic.Affinity != AffinityExecutorSticky || automatic.Migration != MigrationRecreate {
		t.Fatalf("unexpected automatic TLS semantics: %+v", automatic)
	}
	asset := SemanticsOf(CapLoadCert)
	if asset.StateClass != StatePortableAsset || asset.Affinity != AffinityNone || asset.Migration != MigrationReloadAsset {
		t.Fatalf("unexpected certificate asset semantics: %+v", asset)
	}
	route := SemanticsOf(CapRouteHost)
	if route.StateClass != StateDeclarative || route.Migration != MigrationRerender {
		t.Fatalf("unexpected route semantics: %+v", route)
	}
}
