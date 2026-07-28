package handlers

import (
	"errors"
	"testing"

	"aegis/internal/apply"
	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
)

func TestModeSwitchRollbackStatus(t *testing.T) {
	if got := modeSwitchRollbackStatus(errors.New("preflight failed")); got != apply.RollbackNotRequired {
		t.Fatalf("preflight rollback status = %q", got)
	}
	for _, status := range []apply.RollbackStatus{apply.RollbackComplete, apply.RollbackIncomplete} {
		err := &apply.ModeSwitchError{Reason: "test", RollbackStatus: status, RollbackFailures: []string{"failure"}}
		if got := modeSwitchRollbackStatus(err); got != status {
			t.Fatalf("typed rollback status = %q, want %q", got, status)
		}
	}
}

type modeTLSProvider struct{ state provider.ProviderState }

func (p *modeTLSProvider) State() provider.ProviderState { return p.state }
func (p *modeTLSProvider) Diagnose() provider.ProviderDiagnostic {
	return provider.ProviderDiagnostic{}
}
func (p *modeTLSProvider) Render(provider.Plan) ([]provider.ConfigFile, error) { return nil, nil }
func (p *modeTLSProvider) Apply([]provider.ConfigFile) error                   { return nil }

func TestRouteExecutionForModePreservesThenRecreatesAutomaticTLS(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Installed: true,
		Capabilities: []provider.Capability{provider.CapAutoCert, provider.CapLoadCert},
	}})
	rt := route.Route{
		Composition: "https_route", SourceProvider: "caddy",
		TLSBindingMode: route.TLSBindingProviderAuto, TLSProvider: "caddy",
	}
	if !routeSupportedInMode(rt, provider.RuntimeModeLegacy, registry) {
		t.Fatal("declared Caddy auto-TLS route was rejected in legacy mode")
	}
	rt.TLSProvider = "haproxy"
	target, semantics := routeExecutionForMode(rt, provider.RuntimeModeLegacy, registry)
	if target != "caddy" {
		t.Fatalf("automatic TLS target = %q, want caddy", target)
	}
	if semantics.StateClass != provider.StateProviderManaged || semantics.Migration != provider.MigrationRecreate {
		t.Fatalf("unexpected automatic TLS semantics: %+v", semantics)
	}
}

func TestRouteSupportedInModeRequiresInstalledProvider(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Installed: false, Capabilities: []provider.Capability{provider.CapLoadCert},
	}})
	rt := route.Route{
		Composition: "https_route", SourceProvider: "caddy",
		TLSBindingMode: route.TLSBindingCertificate,
	}
	if routeSupportedInMode(rt, provider.RuntimeModeLegacy, registry) {
		t.Fatal("uninstalled certificate provider was accepted")
	}
}

func TestRouteExecutionForModeReloadsPortableCertificateAsset(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "asset-loader", Installed: true, Capabilities: []provider.Capability{provider.CapLoadCert},
	}})
	mode := provider.RuntimeMode{
		ID: "asset-mode", Implemented: true,
		Providers:    []provider.ProviderAtoms{{ProviderID: "asset-loader"}},
		Compositions: []provider.Composition{{Name: "HTTPS Route", Atoms: []string{"tcp", "tls", "http"}}},
	}
	rt := route.Route{
		Composition: "https_route", SourceProvider: "old-executor",
		TLSBindingMode: route.TLSBindingCertificate,
	}
	target, semantics := routeExecutionForMode(rt, mode, registry)
	if target != "asset-loader" {
		t.Fatalf("certificate target = %q, want asset-loader", target)
	}
	if semantics.StateClass != provider.StatePortableAsset || semantics.Migration != provider.MigrationReloadAsset {
		t.Fatalf("unexpected certificate semantics: %+v", semantics)
	}
}

func TestRouteExecutionForModeUsesPassthroughCapability(t *testing.T) {
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "sni-executor", Installed: true,
		Capabilities: []provider.Capability{provider.CapTLSPassthrough},
	}})
	mode := provider.RuntimeMode{
		ID: "sni-mode", Implemented: true,
		Providers:    []provider.ProviderAtoms{{ProviderID: "sni-executor"}},
		Compositions: []provider.Composition{{Name: "TLS Passthrough", Atoms: []string{"tcp", "sni"}}},
	}
	rt := route.Route{Composition: "tls_passthrough", SourceProvider: "old-executor", TLSBindingMode: route.TLSBindingOff}
	target, semantics := routeExecutionForMode(rt, mode, registry)
	if target != "sni-executor" || semantics.Migration != provider.MigrationRerender {
		t.Fatalf("unexpected passthrough migration: target=%q semantics=%+v", target, semantics)
	}
}
