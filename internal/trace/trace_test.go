package trace

import (
	"net"
	"testing"
	"time"

	"aegis/internal/hostdep/provider"
)

func TestAccessPathTrace_ModelFields(t *testing.T) {
	tr := &AccessPathTrace{
		Input:       "example.com",
		InputType:   "domain",
		TraceStatus: StatusComplete,
		Steps: []TraceStep{
			{Order: 1, Component: "listener", Name: "entry", Status: "matched", Detail: "port 443", Address: "0.0.0.0:443"},
			{Order: 2, Component: "edge_mux", Name: "sni_match", Status: "matched", Detail: "edge rule found", Address: "127.0.0.1:8443"},
			{Order: 3, Component: "caddy", Name: "tls_termination", Status: "matched", Detail: "Caddy at 8443", Address: "127.0.0.1:8443"},
			{Order: 4, Component: "route", Name: "route_match", Status: "matched", Detail: "route rt_123 active"},
		},
		FinalTarget: &TargetInfo{
			Host:     "10.0.0.5",
			Port:     8080,
			Protocol: "http",
		},
	}

	if tr.Input != "example.com" {
		t.Errorf("expected Input='example.com', got '%s'", tr.Input)
	}
	if len(tr.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(tr.Steps))
	}
	if tr.FinalTarget.Host != "10.0.0.5" {
		t.Errorf("expected target host='10.0.0.5', got '%s'", tr.FinalTarget.Host)
	}
	t.Logf("Model: %s trace with %d steps → %s:%d", tr.Input, len(tr.Steps), tr.FinalTarget.Host, tr.FinalTarget.Port)
}

func TestTraceStatusConstants(t *testing.T) {
	if StatusComplete != "complete" {
		t.Error("StatusComplete mismatch")
	}
	if StatusIncomplete != "incomplete" {
		t.Error("StatusIncomplete mismatch")
	}
	if StatusNotFound != "not_found" {
		t.Error("StatusNotFound mismatch")
	}
	if StatusError != "error" {
		t.Error("StatusError mismatch")
	}
	t.Log("All trace status constants verified")
}

func TestTargetErrorCodes(t *testing.T) {
	codes := map[string]string{
		ErrTargetUnreachable: "TARGET_UNREACHABLE",
		ErrTargetTimeout:     "TARGET_TIMEOUT",
		ErrTargetDNSFailed:   "TARGET_DNS_FAILED",
		ErrTargetConnRefused: "TARGET_CONNECTION_REFUSED",
		ErrTraceNotFound:     "TRACE_NOT_FOUND",
		ErrTraceIncomplete:   "TRACE_INCOMPLETE",
	}
	for name, expected := range codes {
		if name != expected {
			t.Errorf("constant %s has wrong value: %s", name, name)
		}
	}
	t.Logf("All %d target error codes verified", len(codes))
}

func TestTargetInfoReachable(t *testing.T) {
	reachable := true
	target := &TargetInfo{
		Host:      "10.0.0.1",
		Port:      8080,
		Protocol:  "http",
		Reachable: &reachable,
	}

	if target.Reachable == nil || !*target.Reachable {
		t.Error("expected reachable=true")
	}

	unreachable := false
	target.Reachable = &unreachable
	target.ErrorCode = ErrTargetConnRefused
	target.ConnectError = "connection refused"

	if *target.Reachable {
		t.Error("expected reachable=false")
	}
	if target.ErrorCode != ErrTargetConnRefused {
		t.Errorf("expected %s, got %s", ErrTargetConnRefused, target.ErrorCode)
	}
	t.Logf("TargetInfo reachable state: ok (reachable=%v, code=%s)", *target.Reachable, target.ErrorCode)
}

func TestTraceStepOrdering(t *testing.T) {
	steps := []TraceStep{
		{Order: 1, Component: "listener", Status: "matched"},
		{Order: 2, Component: "edge_mux", Status: "matched"},
		{Order: 3, Component: "caddy", Status: "matched"},
		{Order: 4, Component: "route", Status: "matched"},
		{Order: 5, Component: "target", Status: "matched"},
	}

	// Steps must be in increasing order
	for i := 1; i < len(steps); i++ {
		if steps[i].Order <= steps[i-1].Order {
			t.Errorf("step order violation: step %d order %d after step %d order %d",
				i, steps[i].Order, i-1, steps[i-1].Order)
		}
	}
	t.Log("TraceStep ordering: monotonically increasing — OK")
}

func TestTraceMissingDomain(t *testing.T) {
	tr := &AccessPathTrace{
		Input:       "nonexistent.example.com",
		InputType:   "domain",
		TraceStatus: StatusNotFound,
		Errors:      []string{"no route found for domain 'nonexistent.example.com'"},
		Steps: []TraceStep{
			{Order: 1, Component: "route", Name: "route_lookup", Status: "missing",
				Detail: "no route matching domain 'nonexistent.example.com'"},
		},
	}

	if tr.TraceStatus != StatusNotFound {
		t.Errorf("expected status=%s, got %s", StatusNotFound, tr.TraceStatus)
	}
	if len(tr.Errors) != 1 {
		t.Errorf("expected 1 error, got %d", len(tr.Errors))
	}
	if len(tr.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(tr.Steps))
	}
	t.Logf("Missing domain: status=%s errors=%d steps=%d", tr.TraceStatus, len(tr.Errors), len(tr.Steps))
}

func TestTraceIncomplete(t *testing.T) {
	tr := &AccessPathTrace{
		Input:       "example.com",
		InputType:   "domain",
		TraceStatus: StatusIncomplete,
		Steps: []TraceStep{
			{Order: 1, Component: "route", Status: "matched", Detail: "route found"},
			{Order: 2, Component: "listener", Status: "matched", Detail: "port 443 found"},
			{Order: 3, Component: "edge_mux", Status: "missing", Detail: "no edge rule for SNI"},
		},
		Warnings: []string{"TLS domain has no matching edge_mux_rule — SNI passthrough may fail"},
	}

	if tr.TraceStatus != StatusIncomplete {
		t.Errorf("expected status=%s, got %s", StatusIncomplete, tr.TraceStatus)
	}
	if len(tr.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %d", len(tr.Warnings))
	}
	// Verify the missing step is correctly identified
	missingFound := false
	for _, s := range tr.Steps {
		if s.Component == "edge_mux" && s.Status == "missing" {
			missingFound = true
		}
	}
	if !missingFound {
		t.Error("expected edge_mux step with status='missing'")
	}
	t.Logf("Incomplete trace: warnings=%d breakpoint=edge_mux", len(tr.Warnings))
}

func TestTraceProviderUnhealthy(t *testing.T) {
	tr := &AccessPathTrace{
		Input:       "example.com",
		InputType:   "domain",
		TraceStatus: StatusIncomplete,
		Steps: []TraceStep{
			{Order: 1, Component: "route", Status: "matched"},
			{Order: 6, Component: "provider", Name: "haproxy_diag", Status: "error",
				Detail: "HAProxy: config_invalid: syntax error"},
		},
		Warnings: []string{"HAProxy config invalid"},
	}

	// Provider diagnostic step should be present and show error
	hasDiag := false
	for _, s := range tr.Steps {
		if s.Component == "provider" && s.Status == "error" {
			hasDiag = true
			if s.Name != "haproxy_diag" {
				t.Errorf("expected diag step name='haproxy_diag', got '%s'", s.Name)
			}
		}
	}
	if !hasDiag {
		t.Error("expected provider diagnostic step with status='error'")
	}
	t.Logf("Provider unhealthy trace: diag present, %d warnings", len(tr.Warnings))
}

func TestTraceTargetUnreachable(t *testing.T) {
	unreachable := false
	target := &TargetInfo{
		Host:         "192.168.1.100",
		Port:         9999,
		Protocol:     "http",
		Reachable:    &unreachable,
		ErrorCode:    ErrTargetTimeout,
		ConnectError: "connection to 192.168.1.100:9999 timed out after 2s",
	}

	if target.Reachable != nil && *target.Reachable {
		t.Error("expected reachable=false")
	}
	if target.ErrorCode != ErrTargetTimeout {
		t.Errorf("expected %s, got %s", ErrTargetTimeout, target.ErrorCode)
	}
	if target.ConnectError == "" {
		t.Error("expected non-empty ConnectError")
	}
	t.Logf("Target unreachable: code=%s error=%s", target.ErrorCode, target.ConnectError)
}

func TestServiceNew(t *testing.T) {
	svc := NewService(Dependencies{})
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if svc.tcpTimeout == 0 {
		t.Error("expected non-zero tcpTimeout")
	}
	t.Logf("Service created: tcpTimeout=%v", svc.tcpTimeout)
}

// =============================================================================
// v1.7X: Trace Runtime Connectivity Tests (with real TCP listener)
// =============================================================================

func TestCheckTargetConnectivity_Reachable(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	defer l.Close()

	port := l.Addr().(*net.TCPAddr).Port

	svc := NewService(Dependencies{})
	target := &TargetInfo{Host: "127.0.0.1", Port: port, Protocol: "http"}
	svc.checkTargetConnectivity(target)

	if target.Reachable == nil {
		t.Fatal("expected Reachable to be set")
	}
	if !*target.Reachable {
		t.Error("expected target to be reachable (listener active)")
	}
	t.Logf("Target reachable: %s:%d ✓", target.Host, target.Port)
}

func TestCheckTargetConnectivity_ConnectionRefused(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("cannot listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close() // port now free → connection refused

	svc := NewService(Dependencies{})
	target := &TargetInfo{Host: "127.0.0.1", Port: port, Protocol: "http"}
	svc.checkTargetConnectivity(target)

	if target.Reachable == nil || *target.Reachable {
		t.Error("expected target unreachable (port closed)")
	}
	if target.ErrorCode != ErrTargetConnRefused {
		t.Errorf("expected %s, got %s", ErrTargetConnRefused, target.ErrorCode)
	}
	t.Logf("Connection refused: %s:%d → %s ✓", target.Host, target.Port, target.ErrorCode)
}

func TestCheckTargetConnectivity_DNSFailed(t *testing.T) {
	svc := NewService(Dependencies{})
	target := &TargetInfo{Host: "nonexistent.invalid.domain.test", Port: 8080, Protocol: "http"}
	svc.checkTargetConnectivity(target)

	if target.Reachable == nil || *target.Reachable {
		t.Error("expected target unreachable (DNS failed)")
	}
	if target.ErrorCode != ErrTargetDNSFailed {
		t.Errorf("expected %s, got %s", ErrTargetDNSFailed, target.ErrorCode)
	}
	t.Logf("DNS failed: %s → %s ✓", target.Host, target.ErrorCode)
}

func TestCheckTargetConnectivity_Timeout(t *testing.T) {
	svc := NewService(Dependencies{})
	svc.tcpTimeout = 100 * time.Millisecond

	target := &TargetInfo{Host: "10.255.255.1", Port: 9999, Protocol: "http"}
	svc.checkTargetConnectivity(target)

	if target.Reachable == nil || *target.Reachable {
		t.Error("expected target unreachable")
	}
	if target.ErrorCode == "" {
		t.Error("expected non-empty error code")
	}
	t.Logf("Timeout/unreachable: %s:%d → %s ✓", target.Host, target.Port, target.ErrorCode)
}

func TestTraceStepProviderDiagnostic(t *testing.T) {
	valid := true
	diag := &provider.ProviderDiagnostic{
		Provider:    "caddy_http",
		Installed:   true,
		Version:     "v2.7.6",
		ConfigValid: &valid,
	}

	step := TraceStep{
		Order: 7, Component: "provider", Name: "caddy_diag",
		Status: "matched", Detail: "Caddy: available (v2.7.6)",
		ProviderDiagnostic: diag,
	}

	if step.ProviderDiagnostic == nil {
		t.Fatal("ProviderDiagnostic should be set")
	}
	if step.ProviderDiagnostic.Provider != "caddy_http" {
		t.Errorf("expected caddy_http, got %s", step.ProviderDiagnostic.Provider)
	}
	t.Logf("ProviderDiagnostic in trace step: %s v%s ✓", step.ProviderDiagnostic.Provider, step.ProviderDiagnostic.Version)
}

func TestTraceStepProviderUnhealthy(t *testing.T) {
	running := false
	diag := &provider.ProviderDiagnostic{
		Provider:         "haproxy_edge_mux",
		Installed:        false,
		ServiceRunning:   &running,
		LastErrorCode:    provider.DiagCodeServiceNotRunning,
		LastErrorMessage: "haproxy systemd service is not active",
	}

	step := TraceStep{
		Order: 6, Component: "provider", Name: "haproxy_diag",
		Status: "error",
		Detail: "HAProxy: SERVICE_NOT_RUNNING — haproxy not active",
		ProviderDiagnostic: diag,
	}

	if step.Status != "error" {
		t.Error("unhealthy provider step should have status=error")
	}
	if step.ProviderDiagnostic.LastErrorCode != provider.DiagCodeServiceNotRunning {
		t.Errorf("expected SERVICE_NOT_RUNNING, got %s", step.ProviderDiagnostic.LastErrorCode)
	}
	t.Logf("Unhealthy provider in trace: %s → %s ✓", step.Name, step.ProviderDiagnostic.LastErrorCode)
}
