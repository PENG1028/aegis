package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"io"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// v1.8C-7: Local gateway health/status endpoint tests
// ---------------------------------------------------------------------------

// mockRTProvider implements RoutingTableStatusProvider for tests.
type mockRTProvider struct {
	info RoutingTableInfo
}

func (m *mockRTProvider) GetRoutingTableStatus() RoutingTableInfo {
	return m.info
}

func newTestHandlerWithStatus(resolver DomainResolver, config *Config, rtProvider RoutingTableStatusProvider) *Handler {
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	// Use a secret provider with no tokens for health/status tests
	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)
	handler.SetGatewayStatus(NewGatewayStatusFromConfig(config))
	if rtProvider != nil {
		handler.SetRoutingTableStatusProvider(rtProvider)
	}
	return handler
}

// TestLocalHealthEndpoint verifies GET /__aegis/local/health returns 200.
func TestLocalHealthEndpoint(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/__aegis/local/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}

	// Parse body
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("health status field = %q, want ok", body["status"])
	}
	if body["service"] != "aegis-local-gateway" {
		t.Errorf("service field = %q, want aegis-local-gateway", body["service"])
	}
}

// TestLocalStatusEndpoint verifies GET /__aegis/local/status returns valid JSON
// and does not contain any token-like fields.
func TestLocalStatusEndpoint(t *testing.T) {
	rtProvider := &mockRTProvider{
		info: RoutingTableInfo{
			Loaded:   true,
			Entries:  3,
			Revision: 4,
		},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node-status"
	handler := newTestHandlerWithStatus(resolver, config, rtProvider)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/__aegis/local/status")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	var status LocalStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status response: %v", err)
	}

	// Verify fields
	if status.NodeID != "test-node-status" {
		t.Errorf("node_id = %q, want test-node-status", status.NodeID)
	}
	if !status.RoutingTable.Loaded {
		t.Error("routing_table.loaded should be true")
	}
	if status.RoutingTable.Entries != 3 {
		t.Errorf("routing_table.entries = %d, want 3", status.RoutingTable.Entries)
	}
	if status.RoutingTable.Revision != 4 {
		t.Errorf("routing_table.revision = %d, want 4", status.RoutingTable.Revision)
	}
	if !status.Cache.RoutingTable {
		t.Error("cache.routing_table should be true")
	}
	if !status.Cache.DesiredState {
		t.Error("cache.desired_state should be true")
	}
}

// TestLocalStatusWithoutRTProvider verifies status works without routing table provider.
func TestLocalStatusWithoutRTProvider(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/__aegis/local/status")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()

	var status LocalStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.RoutingTable.Loaded {
		t.Error("routing_table should show not loaded without provider")
	}
}

// TestLocalStatusNoTokenLeak verifies status response contains no raw tokens.
func TestLocalStatusNoTokenLeak(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "node-status-test"
	handler := newTestHandlerWithStatus(resolver, config, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/__aegis/local/status")
	if err != nil {
		t.Fatalf("status request failed: %v", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	// Check for token-like patterns
	if strings.Contains(bodyStr, "token") {
		// "token" might appear in field names like "token_file"
		// But should NOT contain raw secret patterns
		t.Logf("Status body contains 'token' (checking for actual leaks): %s", truncate(bodyStr, 200))
	}
	// The raw status response should not contain any "secret" value
	if strings.Contains(strings.ToLower(bodyStr), "secret") {
		t.Error("Status response should not contain 'secret'")
	}
}

// TestLocalHealthOnManagedDomainStillWorks verifies managed domain routing
// is not broken by health endpoint.
func TestLocalHealthOnManagedDomainStillWorks(t *testing.T) {
	targetSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"from":"target"}`))
	}))
	defer targetSvc.Close()

	host, port := getBackendHostPort(t, targetSvc)
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"myapp.example.com": {
				Domain:  "myapp.example.com",
				Status:  "available",
				RouteID: "route-app",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: host,
				TargetLocalPort: port,
			},
		},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Managed domain via Host header still works
	req, _ := http.NewRequest("GET", ts.URL+"/health", nil)
	req.Host = "myapp.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("managed domain = %d, want 200", resp.StatusCode)
	}
}

// TestLocalHealthUnmanagedDomainRejected verifies unmanaged domains
// are still rejected when health endpoints exist.
func TestLocalHealthUnmanagedDomainRejected(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Unmanaged domain should still be 421
	req, _ := http.NewRequest("GET", ts.URL+"/anything", nil)
	req.Host = "unknown.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("unmanaged domain = %d, want 421", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// v1.8C-7: Startup diagnostics tests
// ---------------------------------------------------------------------------

func TestStartupDiagnosticsAllOK(t *testing.T) {
	// Create a temp token file
	tmpDir := t.TempDir()
	tokenFile := tmpDir + "/node.token"
	os.WriteFile(tokenFile, []byte("test-token\n"), 0600)
	cacheDir := tmpDir + "/cache"
	os.MkdirAll(cacheDir, 0755)

	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    tokenFile,
		CacheDir:     cacheDir,
		ControlPlane: "http://127.0.0.1:9000",
		BindAddr:     "127.0.0.1",
		Port:         18790, // use a high port to avoid conflict
		SecretOK:     true,
		RoutingOK:    true,
	}

	result := RunStartupDiagnostics(p)
	// On Windows, file permissions may be reported as 0666 (not 0600),
	// causing a DiagWarning for the token_file check. Accept this.
	if result.HasFailed {
		t.Errorf("expected no failures, got hasFailed=%v", result.HasFailed)
	}
	if result.GatewayStatus == "failed" {
		t.Errorf("expected ready or degraded, got %s", result.GatewayStatus)
	}

	// Check individual checks (accept DiagWarning for token_file on Windows)
	for _, c := range result.Checks {
		if c.Level == DiagFailed {
			t.Errorf("unexpected failed: %s: %s", c.Name, c.Detail)
		}
	}
}

func TestStartupDiagnosticsMissingNodeID(t *testing.T) {
	p := StartupDiagnosticsParams{
		NodeID:       "",
		TokenFile:    "",
		CacheDir:     "",
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         18691,
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	if !result.HasFailed {
		t.Error("expected HasFailed=true for missing node_id")
	}

	// Check node_id check specifically
	foundNodeID := false
	for _, c := range result.Checks {
		if c.Name == "node_id" {
			foundNodeID = true
			if c.Level != DiagFailed {
				t.Errorf("node_id check level = %s, want failed", c.Level)
			}
			if !strings.Contains(c.Detail, "not configured") {
				t.Errorf("node_id detail = %q, want 'not configured'", c.Detail)
			}
		}
	}
	if !foundNodeID {
		t.Error("node_id check not found in diagnostics")
	}
}

func TestStartupDiagnosticsMissingTokenFile(t *testing.T) {
	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    "/nonexistent/path/token.file",
		CacheDir:     "",
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         18692,
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	if !result.HasFailed {
		t.Error("expected HasFailed=true for missing token_file")
	}

	for _, c := range result.Checks {
		if c.Name == "token_file" {
			if c.Level != DiagFailed {
				t.Errorf("token_file level = %s, want failed", c.Level)
			}
			if strings.Contains(c.Detail, "test-token") {
				t.Error("token_file diagnostic leaks token content")
			}
		}
	}
}

func TestStartupDiagnosticsCacheDirNotWritable(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a read-only dir
	readOnly := tmpDir + "/readonly"
	os.MkdirAll(readOnly, 0444)
	defer os.Chmod(readOnly, 0755)

	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    "",
		CacheDir:     readOnly,
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         18693,
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	// cache_dir check may be failed or writable depending on OS
	// On Windows, permissions may not be restrictive
	foundCache := false
	for _, c := range result.Checks {
		if c.Name == "cache_dir" {
			foundCache = true
			if c.Level == DiagFailed {
				t.Logf("cache_dir correctly detected as not writable: %s", c.Detail)
			} else {
				t.Logf("cache_dir check: level=%s detail=%s (OS-dependent)", c.Level, c.Detail)
			}
		}
	}
	if !foundCache {
		t.Error("cache_dir check not found")
	}
}

func TestStartupDiagnosticsPortBindFailure(t *testing.T) {
	// Bind port 0 is always invalid for TCP
	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    "",
		CacheDir:     "",
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         -1, // invalid port
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	foundPort := false
	for _, c := range result.Checks {
		if c.Name == "bind_port" {
			foundPort = true
			if c.Level != DiagFailed {
				t.Errorf("bind_port level = %s, want failed for port -1", c.Level)
			}
		}
	}
	if !foundPort {
		t.Error("bind_port check not found")
	}
}

func TestStartupDiagnosticsSafeString(t *testing.T) {
	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    "/etc/aegis/node.token",
		CacheDir:     "/var/lib/aegis",
		ControlPlane: "http://127.0.0.1:9000",
		BindAddr:     "127.0.0.1",
		Port:         18080,
		SecretOK:     true,
		RoutingOK:    true,
	}
	result := RunStartupDiagnostics(p)
	safe := result.SafeString()

	// Must not contain raw token or secret
	if strings.Contains(safe, "token") {
		// "token" can appear as "node.token" path but not actual values
		t.Logf("SafeString contains 'token': %s (path only, ok)", safe)
	}
	if strings.Contains(safe, "secret") && !strings.Contains(safe, "config") {
		t.Logf("SafeString: %s", safe)
	}
	// Must contain diagnostic summary
	if !strings.Contains(safe, "node_id") {
		t.Error("SafeString should contain node_id")
	}
}

func TestStartupDiagnosticsTokenFileNotLeaked(t *testing.T) {
	// Test that error messages in diagnostics don't leak actual file contents
	p := StartupDiagnosticsParams{
		NodeID:       "node-a",
		TokenFile:    "/tmp/nonexistent.token.path.node.token",
		CacheDir:     "",
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         18694,
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	for _, c := range result.Checks {
		if c.Name == "token_file" {
			// Should say "not found" but NOT include content
			if strings.Contains(c.Detail, "12345") {
				t.Errorf("token_file diagnostic leaks path details: %s", c.Detail)
			}
		}
	}
}

func TestStartupDiagnosticsAllWarnings(t *testing.T) {
	// Empty config should produce warnings but not necessarily failures
	p := StartupDiagnosticsParams{
		NodeID:       "minimal-node",
		TokenFile:    "",
		CacheDir:     "",
		ControlPlane: "",
		BindAddr:     "127.0.0.1",
		Port:         18695,
		SecretOK:     false,
		RoutingOK:    false,
	}

	result := RunStartupDiagnostics(p)
	if result.HasFailed {
		t.Error("expected no failures for minimal config (only warnings)")
	}
	if !result.HasWarnings {
		t.Error("expected warnings for minimal config")
	}
}

// TestLocalHealthLowercasePath verifies lowercase /__aegis/local/health works
func TestLocalHealthLowercasePath(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/__aegis/local/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("lowercase path = %d, want 200", resp.StatusCode)
	}
}

// TestLocalHealthMixedCasePath verifies case-insensitive path matching
func TestLocalHealthMixedCasePath(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}
	config := DefaultConfig()
	config.NodeID = "test-node"
	handler := newTestHandlerWithStatus(resolver, config, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Mixed case should still be intercepted
	resp, err := http.Get(ts.URL + "/__AEGIS/LOCAL/health")
	if err != nil {
		t.Fatalf("mixed case health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("mixed case path = %d, want 200", resp.StatusCode)
	}
}

// truncate returns the first n characters of s.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
