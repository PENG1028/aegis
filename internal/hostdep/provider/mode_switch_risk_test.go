package provider

import (
	"strings"
	"testing"
)

// TestModeSwitchRisksStateDowntimeUnconditionally pins that the preview never
// implies a zero-downtime switch.
//
// The risk list is what the UI turns into mandatory confirmation checkboxes, so
// it is the only place an operator is told what a switch costs. Phase 2 of
// apply.Workflow.SwitchMode stops every provider of the current mode before
// starting the target set — listeners are rebound, so an interruption is
// structural. Nothing in the previous risk list said so.
func TestModeSwitchRisksStateDowntimeUnconditionally(t *testing.T) {
	cases := []struct {
		name            string
		current, target RuntimeMode
		routes          []RouteSpec
	}{
		{"legacy_to_edge_mux", RuntimeModeLegacy, RuntimeModeEdgeMux, nil},
		{"edge_mux_to_legacy", RuntimeModeEdgeMux, RuntimeModeLegacy, nil},
		{
			"with_routes",
			RuntimeModeLegacy, RuntimeModeEdgeMux,
			[]RouteSpec{{
				AppProtocol: "http", TLSMode: "terminate", Transport: "tcp",
				Match:    MatchSpec{Host: "a.example.com"},
				Upstream: UpstreamSpec{Target: "10.0.0.1:3000"},
			}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preview := AnalyseModeSwitch(tc.routes, tc.current, tc.target)

			var found bool
			for _, risk := range preview.Risks {
				if strings.Contains(risk, "中断") && strings.Contains(risk, "端口") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("no risk mentions the interruption from rebinding listeners; risks = %#v", preview.Risks)
			}
		})
	}
}

// TestReconfigRiskDoesNotClaimHotReload guards the wording against the executor.
//
// A provider present in both modes gets action "reconfig", which read as "配置将
// 重新加载，可能有秒级连接中断". The executor does not reload it: the phase-2 stop
// loop covers every provider of the current mode, so caddy moving :443 → :8443
// is stopped and started like any other. Describing that as a hot reload
// understated the outage for the one provider most likely to be serving traffic.
func TestReconfigRiskDoesNotClaimHotReload(t *testing.T) {
	preview := AnalyseModeSwitch(nil, RuntimeModeLegacy, RuntimeModeEdgeMux)

	var sawReconfig bool
	for _, pc := range preview.ProviderChanges {
		if pc.Action == "reconfig" {
			sawReconfig = true
		}
	}
	if !sawReconfig {
		t.Fatal("legacy → edge_mux should reconfig caddy (present in both modes); harness assumption broken")
	}

	for _, risk := range preview.Risks {
		if strings.Contains(risk, "热重载") && !strings.Contains(risk, "不是热重载") {
			t.Errorf("risk claims a hot reload: %q", risk)
		}
		if strings.Contains(risk, "重新加载") && !strings.Contains(risk, "重启") {
			t.Errorf("risk describes a reload without saying the provider restarts: %q", risk)
		}
	}
}

// TestModeSwitchRiskCountCoversEveryProviderChange keeps the confirmation list
// from silently dropping a provider: each stop or reconfig must be represented,
// plus the unconditional downtime line.
func TestModeSwitchRiskCountCoversEveryProviderChange(t *testing.T) {
	preview := AnalyseModeSwitch(nil, RuntimeModeEdgeMux, RuntimeModeLegacy)

	relevant := 0
	for _, pc := range preview.ProviderChanges {
		if pc.Action == "stop" || pc.Action == "reconfig" {
			relevant++
		}
	}
	if len(preview.Risks) < relevant+1 {
		t.Errorf("risks = %d, want at least %d (one per stop/reconfig plus the downtime line); risks = %#v",
			len(preview.Risks), relevant+1, preview.Risks)
	}
}
