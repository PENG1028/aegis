package apply

import (
	"context"
	"slices"
	"strings"
	"testing"

	"aegis/internal/hostdep/provider"
)

// TestSwitchModePlanCoversEveryTargetModeProvider pins a cross-package invariant
// that SwitchMode relies on but does not itself check.
//
// SwitchMode validates ConfigStager/ServiceController over targetProviderIDs,
// which comes from planProviderIDs(plan) — the plan's keys. It then starts
// services by iterating targetMode.ProviderIDs() with an UNCHECKED type
// assertion:
//
//	sc := w.registry.Get(id).(provider.ServiceController)
//
// That is safe only while the two sets are identical. They are, because
// topology.planMatchesMode requires a plan entry for every provider the mode
// names, and PlanForMode errors out otherwise — so SwitchMode returns before
// reaching the assertion. The invariant is held in internal/topology while the
// assertion that depends on it lives in internal/apply, with nothing binding
// them. A future planner change that emitted a partial plan would turn this into
// a nil-interface panic mid-switch, after services are already stopped.
//
// This test asserts the sets match for every implemented mode.
func TestSwitchModePlanCoversEveryTargetModeProvider(t *testing.T) {
	for _, mode := range provider.AllRuntimeModes() {
		if !mode.Implemented {
			continue
		}
		t.Run(mode.ID, func(t *testing.T) {
			w, _, _ := switchModeWorkflow(t)
			states := w.registry.List()

			plan, err := w.planner.PlanForMode("", states, mode)
			if err != nil {
				// A mode whose providers cannot be planned in this harness is not
				// a failure — SwitchMode would return this same error before
				// reaching the unchecked assertion.
				t.Skipf("mode %s not plannable in harness: %v", mode.ID, err)
			}

			planned := planProviderIDs(plan)
			for _, id := range mode.ProviderIDs() {
				if !slices.Contains(planned, id) {
					t.Errorf("mode %s names provider %q but the plan has no entry for it — SwitchMode would panic on the unchecked ServiceController assertion after stopping services",
						mode.ID, id)
				}
			}
			for _, id := range planned {
				if !slices.Contains(mode.ProviderIDs(), id) {
					t.Errorf("plan includes provider %q that mode %s does not name — it would be staged and stopped but never restarted",
						id, mode.ID)
				}
			}
		})
	}
}

// TestSwitchModeRequiresStagerAndControllerBeforeTouchingServices pins that the
// interface requirements are checked while the current mode is still serving.
//
// A provider missing ConfigStager or ServiceController must be rejected during
// planning/rendering — before Phase 2 stops anything. If that check moved after
// the stop loop, a misconfigured provider would take the gateway down and then
// fail, with rollback as the only path back.
func TestSwitchModeRequiresStagerAndControllerBeforeTouchingServices(t *testing.T) {
	w, caddy, haproxy := switchModeWorkflow(t)

	// A provider that cannot stage config: registered, healthy, but incapable.
	// Replacing haproxy means EdgeMux names a provider that fails the check.
	broken := &bareProvider{id: "haproxy", capabilities: haproxy.capabilities}
	registry := provider.NewRegistry()
	registry.Register(caddy)
	registry.Register(broken)
	w.registry = registry

	err := w.SwitchMode(context.Background(), provider.RuntimeModeEdgeMux.ID)
	if err == nil {
		t.Fatal("expected SwitchMode to refuse a provider that cannot stage config")
	}
	// caddy starts running; Stop() clears the flag. Still running means Phase 2
	// was never reached.
	if !caddy.running {
		t.Error("caddy was stopped before the capability check completed — the gateway went down for an error that was knowable up front")
	}
	if !strings.Contains(err.Error(), "stage") && !strings.Contains(err.Error(), "control its service") &&
		!strings.Contains(err.Error(), "unavailable") && !strings.Contains(err.Error(), "installed") {
		t.Errorf("error should name the missing capability, got: %v", err)
	}
}

// bareProvider implements only Provider — no ConfigStager, no ServiceController.
//
// Installed but not running, so DetectRuntimeMode still reports legacy as
// current and EdgeMux is a genuine target. Installed matters because
// PlanForMode requires every target-mode provider to be installed.
type bareProvider struct {
	id           string
	capabilities []provider.Capability
}

func (p *bareProvider) State() provider.ProviderState {
	return provider.ProviderState{
		ID: p.id, Name: p.id, Installed: true, Running: false,
		Capabilities: p.capabilities,
	}
}
func (p *bareProvider) Render(provider.Plan) ([]provider.ConfigFile, error) { return nil, nil }
func (p *bareProvider) Apply([]provider.ConfigFile) error                   { return nil }
func (p *bareProvider) Diagnose() provider.ProviderDiagnostic {
	return provider.ProviderDiagnostic{}
}
