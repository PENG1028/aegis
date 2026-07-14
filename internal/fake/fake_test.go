package fake

import (
	"strings"
	"testing"

	"aegis/internal/hostdep/provider"
)

// =============================================================================
// FakeProvider Diagnostic Error Code Tests (v1.7R — all 7 error types)
// =============================================================================

func TestFakeProviderDiagnose_ProviderMissing(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")
	fp.MissingBinary = true

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeProviderMissing {
		t.Errorf("expected %s, got %s", provider.DiagCodeProviderMissing, diag.LastErrorCode)
	}
	if diag.Installed {
		t.Error("expected Installed=false")
	}
	t.Logf("PROVIDER_MISSING: code=%s installed=%v", diag.LastErrorCode, diag.Installed)
}

func TestFakeProviderDiagnose_VersionUnsupported(t *testing.T) {
	fp := NewFakeProvider("haproxy_edge_mux", "tcp")
	fp.VersionUnsupported = true

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeVersionUnsupported {
		t.Errorf("expected %s, got %s", provider.DiagCodeVersionUnsupported, diag.LastErrorCode)
	}
	if diag.VersionSupported {
		t.Error("expected VersionSupported=false")
	}
	t.Logf("PROVIDER_VERSION_UNSUPPORTED: code=%s version=%s", diag.LastErrorCode, diag.Version)
}

func TestFakeProviderDiagnose_ConfigFileMissing(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")
	fp.ConfigFileMissing = true

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeConfigFileMissing {
		t.Errorf("expected %s, got %s", provider.DiagCodeConfigFileMissing, diag.LastErrorCode)
	}
	if diag.ConfigExists {
		t.Error("expected ConfigExists=false")
	}
	// Also check Validate returns the right code
	err := fp.Validate("/etc/caddy/Caddyfile")
	if err == nil {
		t.Error("expected validate error for missing config file")
	}
	if errStr := err.Error(); !strings.Contains(errStr, provider.DiagCodeConfigFileMissing) {
		t.Errorf("validate error should contain %s: %s", provider.DiagCodeConfigFileMissing, errStr)
	}
	t.Logf("CONFIG_FILE_MISSING: code=%s config_path=%s", diag.LastErrorCode, diag.ConfigPath)
}

func TestFakeProviderDiagnose_ConfigValidateFailed(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")
	fp.FailValidate = true
	fp.ValidateErr = "unexpected token '}' at line 42"

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeConfigValidateFailed {
		t.Errorf("expected %s, got %s", provider.DiagCodeConfigValidateFailed, diag.LastErrorCode)
	}
	if diag.ConfigValid != nil && *diag.ConfigValid {
		t.Error("expected ConfigValid=false")
	}
	if diag.Stderr == "" {
		t.Error("expected stderr to contain validation error details")
	}
	t.Logf("CONFIG_VALIDATE_FAILED: code=%s stderr=%s", diag.LastErrorCode, diag.Stderr)
}

func TestFakeProviderDiagnose_ServiceNotRunning(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")
	fp.Running = false

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeServiceNotRunning {
		t.Errorf("expected %s, got %s", provider.DiagCodeServiceNotRunning, diag.LastErrorCode)
	}
	if diag.ServiceRunning != nil && *diag.ServiceRunning {
		t.Error("expected ServiceRunning=false")
	}
	t.Logf("SERVICE_NOT_RUNNING: code=%s service_running=%v", diag.LastErrorCode, diag.ServiceRunning)
}

func TestFakeProviderDiagnose_ListenerConflict(t *testing.T) {
	fp := NewFakeProvider("haproxy_edge_mux", "tcp")
	fp.ListenerConflict = true
	fp.ListenerConflictDetail = "port 443 already bound by caddy_http"

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeListenerConflict {
		t.Errorf("expected %s, got %s", provider.DiagCodeListenerConflict, diag.LastErrorCode)
	}
	if diag.ListenerOK {
		t.Error("expected ListenerOK=false")
	}
	t.Logf("LISTENER_CONFLICT: code=%s detail=%s", diag.LastErrorCode, diag.LastErrorMessage)

	// Validate should also return the listener conflict error
	err := fp.Validate("/etc/haproxy/haproxy.cfg")
	if err == nil {
		t.Error("expected validate error for listener conflict")
	}
	if errStr := err.Error(); !strings.Contains(errStr, provider.DiagCodeListenerConflict) {
		t.Errorf("validate error should contain %s: %s", provider.DiagCodeListenerConflict, errStr)
	}
}

func TestFakeProviderDiagnose_RuntimeVerifyFailed(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")
	fp.RuntimeVerifyFailed = true
	fp.RuntimeVerifyErr = "health check returned 502 after reload"

	// Runtime verify is triggered during Reload
	err := fp.Reload()
	if err == nil {
		t.Error("expected reload to fail with runtime verify error")
	}
	if errStr := err.Error(); !strings.Contains(errStr, provider.DiagCodeRuntimeVerifyFailed) {
		t.Errorf("reload error should contain %s: %s", provider.DiagCodeRuntimeVerifyFailed, errStr)
	}

	diag := fp.Diagnose()
	if diag.LastErrorCode != provider.DiagCodeRuntimeVerifyFailed {
		t.Errorf("expected %s, got %s", provider.DiagCodeRuntimeVerifyFailed, diag.LastErrorCode)
	}
	t.Logf("RUNTIME_VERIFY_FAILED: code=%s err=%s", diag.LastErrorCode, diag.LastErrorMessage)
}

func TestFakeProviderDiagnose_Healthy(t *testing.T) {
	fp := NewFakeProvider("caddy_http", "http")

	diag := fp.Diagnose()
	if diag.LastErrorCode != "" {
		t.Errorf("expected no error code for healthy provider, got %s", diag.LastErrorCode)
	}
	if !diag.Installed {
		t.Error("expected Installed=true")
	}
	if !diag.VersionSupported {
		t.Error("expected VersionSupported=true")
	}
	if !diag.ConfigExists {
		t.Error("expected ConfigExists=true")
	}
	if !diag.ListenerOK {
		t.Error("expected ListenerOK=true")
	}
	t.Logf("Healthy: provider=%s version=%s all systems go", diag.Provider, diag.Version)
}

// =============================================================================
// FakeProvider Diagnoser Interface Test
// =============================================================================

func TestFakeProviderImplementsDiagnoser(t *testing.T) {
	fp := NewFakeProvider("test", "http")
	var d provider.Diagnoser = fp
	diag := d.Diagnose()
	if diag.Provider != "test" {
		t.Errorf("expected provider name 'test', got '%s'", diag.Provider)
	}
	t.Logf("Diagnoser interface: OK — provider=%s", diag.Provider)
}

// =============================================================================
// FakeProvider ResetErrors Test
// =============================================================================

func TestFakeProviderResetErrors(t *testing.T) {
	fp := NewFakeProvider("test", "http")
	fp.MissingBinary = true
	fp.FailValidate = true
	fp.FailReload = true
	fp.ListenerConflict = true
	fp.RuntimeVerifyFailed = true

	fp.ResetErrors()

	// After reset, all diagnostics should be clean
	diag := fp.Diagnose()
	if diag.LastErrorCode != "" {
		t.Errorf("expected no errors after reset, got %s", diag.LastErrorCode)
	}
	if !fp.Installed || !fp.Running {
		t.Error("expected healthy state after reset")
	}
	t.Logf("ResetErrors: all flags cleared — provider healthy")
}

// =============================================================================
// FakeCluster Enhanced Tests (v1.7R)
// =============================================================================

func TestFakeClusterVersionMismatch(t *testing.T) {
	fc := NewFakeCluster(3)
	fc.InjectVersionMismatch()

	if !fc.VersionMismatch {
		t.Error("expected VersionMismatch=true")
	}
	// Nodes should have diverging state versions
	if fc.Nodes[0].StateVersion == fc.Nodes[2].StateVersion {
		t.Error("expected different state versions across nodes")
	}
	t.Logf("VersionMismatch: node0=%d node1=%d node2=%d",
		fc.Nodes[0].StateVersion, fc.Nodes[1].StateVersion, fc.Nodes[2].StateVersion)
}

func TestFakeClusterAllHealthy(t *testing.T) {
	fc := NewFakeCluster(3)

	if len(fc.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(fc.Nodes))
	}
	leader := fc.GetLeader()
	if leader == nil {
		t.Fatal("expected a leader")
	}
	if leader.NodeID != "node-1" {
		t.Errorf("expected node-1 as leader, got %s", leader.NodeID)
	}
	current := fc.GetCurrent()
	if current == nil {
		t.Fatal("expected a current node")
	}
	if current.NodeID != "node-1" {
		t.Errorf("expected node-1 as current, got %s", current.NodeID)
	}

	// All nodes should be at version 100
	for _, n := range fc.Nodes {
		if n.StateVersion != 100 {
			t.Errorf("node %s: expected state_version 100, got %d", n.NodeID, n.StateVersion)
		}
	}
	t.Logf("All healthy: %d nodes, leader=%s, version=100", len(fc.Nodes), leader.NodeID)
}

// =============================================================================
// Classic Tests (preserved from v1.6B)
// =============================================================================

func TestFakeProviderValidateFails(t *testing.T) {
	fp := NewFakeProvider("test_provider", "http")
	fp.FailValidate = true
	fp.ValidateErr = "syntax error at line 5"

	err := fp.Validate("/tmp/test.conf")
	if err == nil {
		t.Error("expected validation error")
	}
	t.Logf("Validate failed with: %v", err)

	if errStr := err.Error(); !strings.Contains(errStr, provider.DiagCodeConfigValidateFailed) {
		t.Logf("error should include %s: %s", provider.DiagCodeConfigValidateFailed, errStr)
	}
}

func TestFakeProviderReloadFails(t *testing.T) {
	fp := NewFakeProvider("test_provider", "http")
	fp.FailReload = true
	fp.ReloadErr = "service not running"

	err := fp.Reload()
	if err == nil {
		t.Error("expected reload error")
	}
	t.Logf("Reload failed with: %v", err)
}

func TestFakeProviderMissing(t *testing.T) {
	fp := NewFakeProvider("missing_provider", "http")
	fp.Installed = false

	info := fp.Info()
	if info.Status != "unavailable" {
		t.Errorf("expected status unavailable, got %s", info.Status)
	}
	t.Logf("Provider missing: status=%s", info.Status)
}

func TestFakeClusterACKTimeout(t *testing.T) {
	fc := NewFakeCluster(3)
	fc.ACKTimeout = true

	if len(fc.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(fc.Nodes))
	}
	leader := fc.GetLeader()
	if leader == nil {
		t.Error("expected a leader")
	}
	t.Logf("ACK timeout simulation: leader=%s, ack_timeout=%v", leader.NodeID, fc.ACKTimeout)
}

func TestFakeNodeDriftDetection(t *testing.T) {
	fc := NewFakeCluster(3)
	fc.InjectDrift(1, 6)

	severity := fc.CheckDrift(0, 1)
	if severity != "HIGH" {
		t.Errorf("expected HIGH drift, got %s", severity)
	}
	t.Logf("Drift detection: severity=%s", severity)

	fc2 := NewFakeCluster(3)
	fc2.InjectDrift(2, 2)
	severity2 := fc2.CheckDrift(0, 2)
	t.Logf("Small drift: severity=%s", severity2)
}

func TestFakeClusterSplitBrain(t *testing.T) {
	fc := NewFakeCluster(3)
	fc.InjectSplitBrain()

	leaders := 0
	for _, n := range fc.Nodes {
		if n.IsLeader {
			leaders++
		}
	}
	if leaders != 2 {
		t.Errorf("expected 2 leaders for split brain, got %d", leaders)
	}
	t.Logf("Split brain simulation: %d leaders detected", leaders)
}

func TestFakeNodeStale(t *testing.T) {
	fc := NewFakeCluster(3)
	fc.InjectStaleNode(2)

	if !fc.Nodes[2].IsStale {
		t.Error("node 2 should be stale")
	}
	t.Logf("Stale node: node=%s is_stale=%v last_seen=%s", fc.Nodes[2].NodeID, fc.Nodes[2].IsStale, fc.Nodes[2].LastSeen)
}
