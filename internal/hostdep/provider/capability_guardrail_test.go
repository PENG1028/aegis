package provider

import (
	"slices"
	"testing"
)

// unimplementedCapabilities are keys that exist in the enumeration so semantics
// and onboarding docs can reference them, but that no renderer emits. Declaring
// one on a provider makes ProviderState.HasCapability and the UI's
// capabilityIsReady report a capability that cannot execute.
//
// Removing an entry from this list is how a capability graduates: do it in the
// same change that renders it, and TestNoProviderDeclaresUnimplementedCapability
// stops guarding it.
var unimplementedCapabilities = []Capability{
	CapMTLSTerminate,
	CapTLSMasquerade,
}

// TestNoProviderDeclaresUnimplementedCapability keeps the declared set honest.
//
// CapMTLSTerminate (haproxy) and CapTLSMasquerade (caddy) were both declared
// while the renderers emitted nothing for them — no ca-file, no verify
// directive, no masquerade block. Nothing broke, because no Composition can
// reach either capability, so the gap sat latent behind a capability list that
// read as complete.
func TestNoProviderDeclaresUnimplementedCapability(t *testing.T) {
	for _, p := range []struct {
		id    string
		decls []Capability
	}{
		{"caddy", caddyCapabilities()},
		{"haproxy", haproxyCapabilities()},
	} {
		for _, unimpl := range unimplementedCapabilities {
			if slices.Contains(p.decls, unimpl) {
				t.Errorf("provider %s declares %q, which no renderer emits — either render it (and drop it from unimplementedCapabilities) or remove the declaration",
					p.id, unimpl)
			}
		}
	}
}

// TestStatefulCapabilitiesHaveExplicitSemantics is the trap-door guard for
// SemanticsOf.
//
// Its default arm returns declarative/none/re_render, which is correct for the
// many capabilities that are pure config. It is wrong, and silently so, for any
// capability that owns state: a mode-switch preview would promise a plain config
// re-render and omit the asset reload or state recreation the target executor
// needs. CapMTLSTerminate sat in that default while consuming a client-CA
// bundle. A new stateful capability would land there too.
//
// The list below is the claim "these carry state"; SemanticsOf must agree.
func TestStatefulCapabilitiesHaveExplicitSemantics(t *testing.T) {
	stateful := map[Capability]StateClass{
		CapAutoCert:       StateProviderManaged,
		CapLoadCert:       StatePortableAsset,
		CapMTLSTerminate:  StatePortableAsset,
		CapListenTCP:      StateRuntime,
		CapListenUDP:      StateRuntime,
		CapHotReload:      StateRuntime,
		CapValidateConfig: StateRuntime,
	}

	for capability, wantClass := range stateful {
		got := SemanticsOf(capability)
		if got.StateClass != wantClass {
			t.Errorf("%s: state_class = %q, want %q — a stateful capability falling to the declarative default makes mode-switch previews under-promise migration work",
				capability, got.StateClass, wantClass)
		}
		if got.Migration == MigrationRerender && wantClass != StateDeclarative {
			t.Errorf("%s: migration = re_render but state_class = %q; stateful capabilities need reload_asset, recreate, restart or unsupported",
				capability, got.StateClass)
		}
	}
}

// TestPortableAssetCapabilitiesReloadRatherThanRecreate pins the distinction the
// cert lifecycle depends on: an asset on disk moves between executors, so it
// reloads; provider-owned state cannot move, so it is recreated and its binding
// stays executor-sticky.
func TestPortableAssetCapabilitiesReloadRatherThanRecreate(t *testing.T) {
	for _, capability := range []Capability{CapLoadCert, CapMTLSTerminate} {
		s := SemanticsOf(capability)
		if s.Migration != MigrationReloadAsset {
			t.Errorf("%s: migration = %q, want reload_asset", capability, s.Migration)
		}
		if s.Affinity != AffinityNone {
			t.Errorf("%s: affinity = %q, want none — a file on disk is not tied to one executor", capability, s.Affinity)
		}
	}

	auto := SemanticsOf(CapAutoCert)
	if auto.Migration != MigrationRecreate {
		t.Errorf("auto_cert: migration = %q, want recreate", auto.Migration)
	}
	if auto.Affinity != AffinityExecutorSticky {
		t.Errorf("auto_cert: affinity = %q, want executor_sticky — ACME state lives inside one executor", auto.Affinity)
	}
}

// TestRuntimeModeProvidersAreConstructible guards the mode↔provider seam: every
// provider ID named in a RuntimeMode's atom bindings must be a provider the
// registry can actually build. A typo or a renamed provider would otherwise
// surface as an empty target executor during a mode switch preview.
func TestRuntimeModeProvidersAreConstructible(t *testing.T) {
	known := []string{"caddy", "haproxy"}

	for _, mode := range AllRuntimeModes() {
		if !mode.Implemented {
			continue
		}
		if len(mode.Providers) == 0 {
			t.Errorf("mode %s is Implemented but binds no providers", mode.ID)
		}
		for _, pa := range mode.Providers {
			if !slices.Contains(known, pa.ProviderID) {
				t.Errorf("mode %s binds unknown provider %q — add it to the registry and to this test's known list",
					mode.ID, pa.ProviderID)
			}
		}
		if !slices.Contains(mode.ProviderIDs(), mode.Providers[0].ProviderID) {
			t.Errorf("mode %s: ProviderIDs() disagrees with Providers", mode.ID)
		}
	}
}

// TestEveryCapabilityKeyInEnumerationParses ensures the display enumeration and
// the typed constants cannot drift: the capability matrix UI renders rows from
// AllCapabilities, while gating uses the constants.
func TestEveryCapabilityKeyInEnumerationParses(t *testing.T) {
	declared := append(caddyCapabilities(), haproxyCapabilities()...)
	enumerated := make(map[string]bool)
	for _, def := range AllCapabilities() {
		enumerated[def.Key] = true
	}

	for _, capability := range declared {
		if !enumerated[string(capability)] {
			t.Errorf("provider declares %q but AllCapabilities has no row for it — the capability matrix UI would silently omit it",
				capability)
		}
	}
}
