package listener

import (
	"fmt"
	"testing"

	"aegis/internal/hostdep/provider"
)

// TestListenerDefaultsAgreeWithRuntimeMode binds the two independent port
// allocation tables to each other.
//
// Port ownership is declared twice in this codebase:
//
//   - provider.RuntimeMode{}.Providers[].Bindings — the source the planner,
//     mode-switch preview, and config renderers read.
//   - listener.DefaultListeners() / EdgeMuxDefaults() — read by the listener
//     registry, the safety service, edgemux, trace, and two HTTP handlers.
//
// They agree today, but nothing enforces it. Changing a port in RuntimeMode
// without changing it here would leave the safety service validating against
// ports nobody binds, and the listener registry claiming ownership of a port the
// renderers never write. Both failures are silent: the config would be correct
// and the bookkeeping wrong, so the symptom appears later as a conflict report
// about a port that is actually fine, or no report about one that is not.
//
// This test does not merge the two tables — that is a refactor with real blast
// radius across seven call sites. It makes the divergence fail loudly instead.
func TestListenerDefaultsAgreeWithRuntimeMode(t *testing.T) {
	cases := []struct {
		name      string
		mode      provider.RuntimeMode
		listeners []Listener
	}{
		{"legacy", provider.RuntimeModeLegacy, DefaultListeners()},
		{"edge_mux", provider.RuntimeModeEdgeMux, EdgeMuxDefaults()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fromMode := portSet(modePorts(tc.mode))
			fromListeners := portSet(listenerPorts(tc.listeners))

			for p := range fromListeners {
				if !fromMode[p] {
					t.Errorf("listener defaults claim :%d but RuntimeMode %s binds nothing there — "+
						"the safety service would validate a port no renderer writes",
						p, tc.mode.ID)
				}
			}
			for p := range fromMode {
				if !fromListeners[p] {
					t.Errorf("RuntimeMode %s binds :%d but the listener registry does not know about it — "+
						"conflict detection would miss a port that is actually owned",
						tc.mode.ID, p)
				}
			}
		})
	}
}

// TestEveryListenerDefaultHasAPurpose guards the bookkeeping fields the conflict
// reporter renders. A listener with an empty purpose produces a conflict message
// that names a port and nothing else, which is not actionable.
func TestEveryListenerDefaultHasAPurpose(t *testing.T) {
	for _, set := range [][]Listener{DefaultListeners(), EdgeMuxDefaults()} {
		for _, l := range set {
			if l.Purpose == "" {
				t.Errorf("listener %s (:%d) has no purpose — conflict reports would be unactionable", l.ID, l.Port)
			}
			if l.Provider == "" {
				t.Errorf("listener %s (:%d) has no provider — ownership is unattributable", l.ID, l.Port)
			}
			if l.Port <= 0 || l.Port > 65535 {
				t.Errorf("listener %s has an invalid port %d", l.ID, l.Port)
			}
		}
	}
}

// TestListenerDefaultsHaveNoInternalPortCollision guards that a single mode's
// defaults do not claim the same bind address twice, which would make the
// registry's own conflict detection fire against itself on a clean install.
func TestListenerDefaultsHaveNoInternalPortCollision(t *testing.T) {
	for name, set := range map[string][]Listener{
		"legacy":   DefaultListeners(),
		"edge_mux": EdgeMuxDefaults(),
	} {
		seen := map[string]string{}
		for _, l := range set {
			key := fmt.Sprintf("%s:%d", l.BindIP, l.Port)
			if prev, dup := seen[key]; dup {
				t.Errorf("%s: %s and %s both claim %s", name, prev, l.ID, key)
			}
			seen[key] = l.ID
		}
	}
}

func modePorts(m provider.RuntimeMode) []int {
	var out []int
	for _, pa := range m.Providers {
		for _, slots := range pa.Bindings {
			for _, s := range slots {
				if s.Port > 0 {
					out = append(out, s.Port)
				}
			}
		}
	}
	return out
}

func listenerPorts(ls []Listener) []int {
	out := make([]int, 0, len(ls))
	for _, l := range ls {
		out = append(out, l.Port)
	}
	return out
}

func portSet(ports []int) map[int]bool {
	set := make(map[int]bool, len(ports))
	for _, p := range ports {
		set[p] = true
	}
	return set
}
