package provider

import (
	"testing"
)

func TestAnalyseModeSwitch_EdgeMuxToLegacy(t *testing.T) {
	routes := []RouteSpec{
		{AppProtocol: "http", TLSMode: "terminate", Transport: "tcp", Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:3000"}},
		{AppProtocol: "http", TLSMode: "terminate", Transport: "tcp", Match: MatchSpec{Host: "b.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:4000"}},
		{AppProtocol: "raw", TLSMode: "passthrough", Transport: "tcp", Match: MatchSpec{SNI: "db.com"}, Upstream: UpstreamSpec{Target: "10.0.0.5:5432"}},
	}

	currentMode := RuntimeModeEdgeMux
	targetMode := RuntimeModeLegacy

	preview := AnalyseModeSwitch(routes, currentMode, targetMode)

	if preview.TotalRoutes != 3 {
		t.Errorf("expected 3 total routes, got %d", preview.TotalRoutes)
	}
	if preview.AffectedRoutes.Kept != 2 {
		t.Errorf("expected 2 kept routes, got %d", preview.AffectedRoutes.Kept)
	}
	if preview.AffectedRoutes.Unsupported != 1 {
		t.Errorf("expected 1 unsupported route (passthrough), got %d", preview.AffectedRoutes.Unsupported)
	}

	// Check route breakdown
	var hasHTTPS, hasPassthrough bool
	for _, b := range preview.RouteBreakdown {
		switch b.Key {
		case "https_route":
			hasHTTPS = true
			if !b.CurrentModeOK {
				t.Error("HTTPS Route should be supported in EdgeMux")
			}
			if !b.TargetModeOK {
				t.Error("HTTPS Route should be supported in Legacy")
			}
		case "tls_passthrough":
			hasPassthrough = true
			if !b.CurrentModeOK {
				t.Error("TLS Passthrough should be supported in EdgeMux")
			}
			if b.TargetModeOK {
				t.Error("TLS Passthrough should NOT be supported in Legacy")
			}
			if b.Reason == "" {
				t.Error("TLS Passthrough should have a reason in Legacy")
			}
		}
	}
	if !hasHTTPS {
		t.Error("expected HTTPS Route breakdown")
	}
	if !hasPassthrough {
		t.Error("expected TLS Passthrough breakdown")
	}

	// Check provider changes
	seenHAProxy := false
	for _, pc := range preview.ProviderChanges {
		if pc.ProviderID == "haproxy" {
			seenHAProxy = true
			if pc.Action != "stop" {
				t.Errorf("expected HAProxy to stop, got %s", pc.Action)
			}
		}
	}
	if !seenHAProxy {
		t.Error("expected HAProxy in provider changes")
	}

	// Check risks
	if len(preview.Risks) == 0 {
		t.Error("expected at least 1 risk")
	}
}

func TestAnalyseModeSwitch_LegacyToEdgeMux(t *testing.T) {
	routes := []RouteSpec{
		{AppProtocol: "http", TLSMode: "terminate", Transport: "tcp", Match: MatchSpec{Host: "a.com"}, Upstream: UpstreamSpec{Target: "10.0.0.1:3000"}},
	}

	preview := AnalyseModeSwitch(routes, RuntimeModeLegacy, RuntimeModeEdgeMux)
	if preview.TotalRoutes != 1 {
		t.Errorf("expected 1 route, got %d", preview.TotalRoutes)
	}
	if preview.AffectedRoutes.Kept != 1 {
		t.Errorf("expected 1 kept, got %d", preview.AffectedRoutes.Kept)
	}

	// HAProxy should need to start
	hasHAProxyStart := false
	for _, pc := range preview.ProviderChanges {
		if pc.ProviderID == "haproxy" && pc.Action == "start" {
			hasHAProxyStart = true
		}
	}
	if !hasHAProxyStart {
		t.Error("expected HAProxy to start when switching to EdgeMux")
	}
}
