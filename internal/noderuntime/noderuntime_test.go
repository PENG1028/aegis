package noderuntime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"reflect"
	"testing"
)

// ============================================================================
// Test helpers
// ============================================================================

func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
	}
}

func assertTrue(t *testing.T, val bool) {
	t.Helper()
	if !val {
		t.Errorf("expected true, got false")
	}
}

func assertFalse(t *testing.T, val bool) {
	t.Helper()
	if val {
		t.Errorf("expected false, got true")
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error, msgAndArgs ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(msgAndArgs) > 0 {
		if !strings.Contains(err.Error(), msgAndArgs[0]) {
			t.Errorf("error %q does not contain %q", err.Error(), msgAndArgs[0])
		}
	}
}

func assertNil(t *testing.T, val interface{}) {
	t.Helper()
	if val == nil {
		return
	}
	// Handle typed nil (e.g., nil *CandidateEntry, nil []CandidateEntry)
	// that becomes non-nil when boxed into interface{}
	switch v := reflect.ValueOf(val); v.Kind() {
	case reflect.Ptr, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if v.IsNil() {
			return
		}
	}
	t.Errorf("expected nil, got %v (type: %T)", val, val)
}

func assertNotNil(t *testing.T, val interface{}) {
	t.Helper()
	if val == nil {
		t.Errorf("expected non-nil, got nil")
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected %q to NOT contain %q", s, substr)
	}
}

func requireValid(t *testing.T, r *ValidationResult) {
	t.Helper()
	if !r.IsValid {
		t.Fatalf("expected valid, got errors: %v", r.Errors)
	}
}

func requireInvalid(t *testing.T, r *ValidationResult, optSubstr ...string) {
	t.Helper()
	if r.IsValid {
		t.Fatal("expected invalid, got valid")
	}
	if len(optSubstr) > 0 && len(r.Errors) > 0 {
		found := false
		for _, e := range r.Errors {
			if strings.Contains(e, optSubstr[0]) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected error containing %q, got: %v", optSubstr[0], r.Errors)
		}
	}
}

// ============================================================================
// Config tests (3 tests)
// ============================================================================

func TestLoadConfig(t *testing.T) {
	// Ensure no env var interference for this test
	t.Setenv("AEGIS_CONTROL_PLANE_URL", "")
	t.Setenv("AEGIS_NODE_ID", "")
	t.Setenv("AEGIS_NODE_TOKEN", "")
	t.Setenv("AEGIS_NODE_TOKEN_FILE", "")

	dir := t.TempDir()
	configPath := filepath.Join(dir, "node.yaml")
	tokenPath := filepath.Join(dir, "node.token")

	// Write token file
	assertNoError(t, os.WriteFile(tokenPath, []byte("test-token-abc\n"), 0644))

	// Write config file with minimal required fields:
	// node_id, node_token_file (to load the token), control_plane_url
	configContent := fmt.Sprintf("node_id: test-node\nnode_token_file: %s\ncontrol_plane_url: http://192.168.1.1:8080\n", tokenPath)
	assertNoError(t, os.WriteFile(configPath, []byte(configContent), 0644))

	cfg, err := LoadConfig(configPath)
	assertNoError(t, err)
	assertEqual(t, cfg.NodeID, "test-node")
	assertEqual(t, cfg.NodeToken, "test-token-abc")
	assertEqual(t, cfg.ControlPlaneURL, "http://192.168.1.1:8080")
	// Defaults should be applied
	assertEqual(t, cfg.CacheDir, DefaultCacheDir)
	assertEqual(t, cfg.RuntimeDir, DefaultRuntimeDir)
	assertEqual(t, cfg.HeartbeatIntervalSec, DefaultHeartbeatSec)
	assertEqual(t, cfg.SyncIntervalSec, DefaultSyncSec)
	assertEqual(t, cfg.ReconcileMode, DefaultReconcileMode)
}

func TestConfigSafeStringNoToken(t *testing.T) {
	cfg := &Config{
		ControlPlaneURL:      "http://cp.example.com:8080",
		NodeID:               "node-alpha",
		NodeToken:            "super-secret-token-64hexchars...",
		CacheDir:             "/custom/cache",
		RuntimeDir:           "/custom/run",
		HeartbeatIntervalSec: 30,
		SyncIntervalSec:      60,
		ReconcileMode:        "apply",
	}

	s := cfg.SafeString()

	// Must contain safe fields
	assertContains(t, s, "http://cp.example.com:8080")
	assertContains(t, s, "node-alpha")
	assertContains(t, s, "/custom/cache")
	assertContains(t, s, "30s")
	assertContains(t, s, "60s")
	assertContains(t, s, "apply")

	// Must NOT contain the token
	assertNotContains(t, s, "super-secret-token")
	assertNotContains(t, s, "64hexchars")
}

func TestConfigValidate(t *testing.T) {
	// Missing node_id returns error
	cfg := &Config{
		ControlPlaneURL: "http://cp:8080",
		NodeToken:       "some-token",
	}
	err := cfg.Validate()
	assertError(t, err, "node_id")

	// Missing node_token returns error
	cfg2 := &Config{
		ControlPlaneURL: "http://cp:8080",
		NodeID:          "node-a",
	}
	err2 := cfg2.Validate()
	assertError(t, err2, "node_token")

	// Missing control_plane_url returns error
	cfg3 := &Config{
		NodeID:    "node-a",
		NodeToken: "some-token",
	}
	err3 := cfg3.Validate()
	assertError(t, err3, "control_plane_url")

	// Valid config passes
	cfg4 := &Config{
		ControlPlaneURL: "http://cp:8080",
		NodeID:          "node-a",
		NodeToken:       "some-token",
	}
	err4 := cfg4.Validate()
	assertNoError(t, err4)
}

// ============================================================================
// Cache tests (3 tests)
// ============================================================================

func TestWriteDesiredStateCache(t *testing.T) {
	dir := t.TempDir()
	cm := NewCacheManager(dir)

	cache := &DesiredStateCache{
		NodeID:    "test-node",
		Revision:  42,
		StateHash: "sha256-abcdef1234567890",
		StateJSON: `{"domains": [{"name": "example.com"}]}`,
	}

	err := cm.WriteDesiredState(cache)
	assertNoError(t, err)

	// Verify file exists
	assertTrue(t, cm.CacheFileExists(DesiredStateCacheFile))

	// Read back and verify
	read, err := cm.ReadDesiredState()
	assertNoError(t, err)
	assertEqual(t, read.NodeID, cache.NodeID)
	assertEqual(t, read.Revision, cache.Revision)
	assertEqual(t, read.StateHash, cache.StateHash)
	assertEqual(t, read.StateJSON, cache.StateJSON)
}

func TestWriteRoutingTableCache(t *testing.T) {
	dir := t.TempDir()
	cm := NewCacheManager(dir)

	cache := &RoutingTableCache{
		NodeID:   "test-node",
		Revision: 7,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				RouteID:      "route-1",
				ServiceID:    "svc-web",
				EndpointID:   "ep-main",
				FromNodeID:   "test-node",
				TargetNodeID: "test-node",
				Protocol:     "http",
				Status:       "available",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80", Priority: 1},
				},
			},
		},
	}

	err := cm.WriteRoutingTable(cache)
	assertNoError(t, err)

	assertTrue(t, cm.CacheFileExists(RoutingTableCacheFile))

	read, err := cm.ReadRoutingTable()
	assertNoError(t, err)
	assertEqual(t, read.NodeID, cache.NodeID)
	assertEqual(t, read.Revision, cache.Revision)
	assertEqual(t, len(read.Entries), 1)
	assertEqual(t, read.Entries[0].Domain, "example.com")
	assertEqual(t, read.Entries[0].Candidates[0].Mode, "local_gateway")
}

func TestContainsRawToken(t *testing.T) {
	// Short string (< 32 chars) => false
	assertFalse(t, ContainsRawToken("short"))
	assertFalse(t, ContainsRawToken("abcdef0123456789abcdef0123456")) // 31 chars

	// 32 consecutive hex chars (passes len check but hexCount < 64) => false
	assertFalse(t, ContainsRawToken(strings.Repeat("a", 32)))

	// 63 consecutive hex chars (passes len check but hexCount < 64) => false
	assertFalse(t, ContainsRawToken(strings.Repeat("a", 63)))

	// Exactly 64 consecutive hex chars => true
	assertTrue(t, ContainsRawToken(strings.Repeat("a", 64)))

	// More than 64 consecutive hex chars => true (early return at 64)
	assertTrue(t, ContainsRawToken(strings.Repeat("a", 100)))

	// 64 hex chars in the middle of a string => true
	assertTrue(t, ContainsRawToken("prefix_" + strings.Repeat("a", 64) + "_suffix"))

	// Non-hex characters interspersed => false (hexCount resets)
	assertFalse(t, ContainsRawToken(strings.Repeat("a", 32) + "x" + strings.Repeat("a", 32)))

	// Completely non-hex characters, >= 32 length => false
	assertFalse(t, ContainsRawToken(strings.Repeat("z", 64)))

	// Uppercase hex also detected
	assertTrue(t, ContainsRawToken(strings.Repeat("A", 64)))

	// Mixed case hex
	assertTrue(t, ContainsRawToken(strings.Repeat("a", 32)+strings.Repeat("A", 32)))

	// Realistic token pattern: hex chars separated by non-hex, at no point 64 consecutive
	assertFalse(t, ContainsRawToken("token=abc123def456abc123def456abc123def456abc123def456"))
	// Check length: "token=" + 48 chars = 55 chars, but let's count the hex portion
	// 48 hex chars, not 64 => false
}

// ============================================================================
// Validator tests (8 tests)
// ============================================================================

func TestValidateRoutingTableValid(t *testing.T) {
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				TargetLocalHost: "127.0.0.1",
				TargetLocalPort: 80,
				Status:       "available",
				Protocol:     "http",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80"},
				},
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireValid(t, result)
	assertEqual(t, len(result.Errors), 0)
	assertEqual(t, len(result.Warnings), 0)
}

func TestValidateRoutingTableWrongNodeID(t *testing.T) {
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-b", // Does NOT match node-a
				TargetNodeID: "node-a",
				Status:       "available",
				Protocol:     "http",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80"},
				},
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "from_node_id")
}

func TestValidateRoutingTableDirectRemote(t *testing.T) {
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-b",
				Status:       "available",
				Protocol:     "http",
				Candidates: []CandidateEntry{
					{
						Mode:       "direct_remote_target",
						GatewayURL: "http://target:80",
					},
				},
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "direct_remote_target")
}

func TestValidateRoutingTableMissingGatewayLink(t *testing.T) {
	// Cross-node entry (target != node), available, but no candidate has a gateway_link_id
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-b", // Different from node-a => cross-node
				Status:       "available",
				Protocol:     "http",
				Candidates: []CandidateEntry{
					{Mode: "remote_gateway", GatewayURL: "http://gateway:80"},
					// No GatewayLinkID on any candidate
				},
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "gateway_link_id")
}

func TestValidateRoutingTableAvailableNoCandidates(t *testing.T) {
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "available",
				Protocol:     "http",
				Candidates:   nil, // No candidates but status is "available"
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "no candidates")
}

func TestValidateRoutingTableUnsupportedProtocol(t *testing.T) {
	table := &RoutingTableCache{
		Revision: 1,
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "available",
				Protocol:     "tcp", // Only "http" (or "") is allowed
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80"},
				},
			},
		},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "tcp")
	requireInvalid(t, result, "only http")
}

func TestValidateDesiredStateForNodeValid(t *testing.T) {
	ds := &DesiredStateCache{
		NodeID:    "node-a",
		Revision:  5,
		StateHash: "sha256:aabbccdd",
		StateJSON: `{"local_routing_table": [{"domain":"test.com"}]}`, // no raw tokens
	}

	result := ValidateDesiredStateForNode("node-a", ds)
	requireValid(t, result)
	assertEqual(t, len(result.Errors), 0)
}

func TestValidateDesiredStateForNodeWrongID(t *testing.T) {
	ds := &DesiredStateCache{
		NodeID:    "node-b", // Wrong! Should match "node-a"
		Revision:  5,
		StateHash: "sha256:aabbccdd",
		StateJSON: `{"local_routing_table": []}`,
	}

	result := ValidateDesiredStateForNode("node-a", ds)
	requireInvalid(t, result, "node_id")

	// Should also work for empty hash/JSON (secondary checks)
	ds2 := &DesiredStateCache{
		NodeID:    "node-b",
		Revision:  0,
		StateHash: "",
		StateJSON: "",
	}
	result2 := ValidateDesiredStateForNode("node-a", ds2)
	requireInvalid(t, result2, "node_id")
	assertTrue(t, len(result2.Errors) >= 3) // node_id, hash, json errors
}

// ============================================================================
// Resolver tests (7 tests)
// ============================================================================

func TestResolveExactDomain(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				RouteID:      "route-web",
				ServiceID:    "svc-web",
				EndpointID:   "ep-main",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "available",
				Protocol:     "http",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80", Priority: 1},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("example.com")

	assertEqual(t, d.Status, "available")
	assertEqual(t, d.Domain, "example.com")
	assertEqual(t, d.RouteID, "route-web")
	assertEqual(t, d.ServiceID, "svc-web")
	assertEqual(t, d.EndpointID, "ep-main")
	assertNotNil(t, d.SelectedCandidate)
	assertEqual(t, d.SelectedCandidate.Mode, "local_gateway")
	assertEqual(t, d.SelectedCandidate.GatewayURL, "http://127.0.0.1:80")
	assertEqual(t, d.TargetNodeID, "node-a")
}

func TestResolveLocalCandidate(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "app.local",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "available",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:8080", Priority: 10},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("app.local")

	assertEqual(t, d.Status, "available")
	assertEqual(t, d.SelectedCandidate.Mode, "local_gateway")
	assertEqual(t, d.SelectedCandidate.Priority, 10)
	// Single candidate, no fallbacks
	assertEqual(t, len(d.FallbackCandidates), 0)
}

func TestResolvePrivateCandidate(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "private.app",
				FromNodeID:   "node-a",
				TargetNodeID: "node-b",
				Status:       "available",
				RouteID:      "route-private",
				Candidates: []CandidateEntry{
					{Mode: "remote_gateway", GatewayURL: "http://gateway-a:80", Priority: 1, RequiresGatewayLink: true, GatewayLinkID: "gl-abc"},
					{Mode: "remote_gateway", GatewayURL: "http://gateway-b:80", Priority: 2},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("private.app")

	assertEqual(t, d.Status, "available")
	assertNotNil(t, d.SelectedCandidate)
	assertEqual(t, d.SelectedCandidate.Mode, "remote_gateway")
	assertEqual(t, d.SelectedCandidate.GatewayURL, "http://gateway-a:80")
	assertEqual(t, d.SelectedCandidate.GatewayLinkID, "gl-abc")
	assertTrue(t, d.SelectedCandidate.RequiresGatewayLink)
	// Fallbacks should contain the second candidate
	assertEqual(t, len(d.FallbackCandidates), 1)
	assertEqual(t, d.FallbackCandidates[0].GatewayURL, "http://gateway-b:80")
	assertEqual(t, d.FallbackCandidates[0].Priority, 2)
}

func TestResolveUnavailableNoCandidate(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "available",
				Candidates:   nil, // No candidates
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("example.com")

	assertEqual(t, d.Status, "unavailable")
	assertContains(t, d.UnavailableReason, "no candidates")
	assertNil(t, d.SelectedCandidate)
}

func TestResolveDisabledEntry(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "disabled",
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80"},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("example.com")

	assertEqual(t, d.Status, "disabled")
	assertContains(t, d.UnavailableReason, "disabled by policy")
	assertNil(t, d.SelectedCandidate)
}

func TestResolveUnknownDomain(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{}, // Empty routing table
	}

	r := NewResolver(table)
	d := r.Resolve("unknown.example.com")

	assertEqual(t, d.Status, "unavailable")
	assertContains(t, d.UnavailableReason, "not found")
}

func TestResolveNoDirectFallback(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-b",
				Status:       "available",
				Candidates: []CandidateEntry{
					{Mode: "direct_remote_target", GatewayURL: "http://direct:80", Priority: 1},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("example.com")

	// Must NOT select the direct_remote_target; must return unavailable
	assertEqual(t, d.Status, "unavailable")
	assertContains(t, d.UnavailableReason, "forbidden")
	assertNil(t, d.SelectedCandidate)
	assertNil(t, d.FallbackCandidates)
}

// ============================================================================
// Reconciler tests (3 tests)
// ============================================================================

func TestSyncOnceNoOutdated(t *testing.T) {
	// Mock server: heartbeat says not outdated, no desired-state endpoint called
	heartbeatCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/node/v1/heartbeat":
			heartbeatCalled = true
			assertEqual(t, r.Method, "POST")
			assertEqual(t, r.Header.Get("Authorization"), "Bearer test-token")
			assertEqual(t, r.Header.Get("Content-Type"), "application/json")
			json.NewEncoder(w).Encode(HeartbeatResponse{
				NodeID:         "node-a",
				Status:         "online",
				LatestRevision: 1,
				NodeIsOutdated: false,
			})
		default:
			t.Errorf("unexpected call to %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	client := NewClient(server.URL, "node-a", "test-token")
	config := &Config{
		NodeID:          "node-a",
		ControlPlaneURL: server.URL,
		NodeToken:       "test-token",
	}
	cache := NewCacheManager(dir)
	reconciler := NewReconciler(config, client, cache)

	result, err := reconciler.SyncOnce()
	assertNoError(t, err)
	assertTrue(t, heartbeatCalled)
	assertEqual(t, result.Status, "no_update_needed")
	assertEqual(t, result.AppliedRevision, 0)
	assertEqual(t, result.StateHash, "")
}

func TestExtractRoutingTableFromState(t *testing.T) {
	// Valid JSON with routing table entries
	stateJSON := `{
		"local_routing_table": [
			{
				"domain": "example.com",
				"route_id": "route-1",
				"from_node_id": "node-a",
				"target_node_id": "node-a",
				"protocol": "http",
				"status": "available",
				"candidates": [
					{"mode": "local_gateway", "gateway_url": "http://127.0.0.1:80"}
				]
			},
			{
				"domain": "test.com",
				"route_id": "route-2",
				"from_node_id": "node-a",
				"target_node_id": "node-b",
				"protocol": "http",
				"status": "available",
				"candidates": [
					{"mode": "remote_gateway", "gateway_url": "http://gateway:80", "requires_gateway_link": true, "gateway_link_id": "gl-1"}
				]
			}
		]
	}`

	rt, err := extractRoutingTableFromState(stateJSON)
	assertNoError(t, err)
	assertNotNil(t, rt)
	assertEqual(t, len(rt.Entries), 2)
	assertEqual(t, rt.Entries[0].Domain, "example.com")
	assertEqual(t, rt.Entries[0].Candidates[0].Mode, "local_gateway")
	assertEqual(t, rt.Entries[1].Domain, "test.com")
	assertEqual(t, rt.Entries[1].Candidates[0].RequiresGatewayLink, true)
	assertEqual(t, rt.Entries[1].Candidates[0].GatewayLinkID, "gl-1")
	assertEqual(t, rt.Entries[0].RouteID, "route-1")
}

func TestExtractRoutingTableFromState_Empty(t *testing.T) {
	// JSON without local_routing_table => empty entries
	stateJSON := `{"other_field": "value"}`

	rt, err := extractRoutingTableFromState(stateJSON)
	assertNoError(t, err)
	assertNotNil(t, rt)
	assertEqual(t, len(rt.Entries), 0)
}

func TestExtractRoutingTableFromState_InvalidJSON(t *testing.T) {
	// Invalid JSON => error
	_, err := extractRoutingTableFromState(`{invalid json}`)
	assertError(t, err)
}

func TestSyncOnceFailedDesiredState(t *testing.T) {
	// Mock server:
	// 1. Heartbeat => outdated=true
	// 2. Desired-state => returns state with wrong node_id (will fail validation)
	// 3. Actual-state => 200 (best-effort report)
	callSequence := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callSequence = append(callSequence, r.URL.Path)
		switch r.URL.Path {
		case "/api/node/v1/heartbeat":
			assertEqual(t, r.Method, "POST")
			json.NewEncoder(w).Encode(HeartbeatResponse{
				NodeID:         "node-a",
				Status:         "online",
				LatestRevision: 3,
				NodeIsOutdated: true,
			})
		case "/api/node/v1/desired-state":
			assertEqual(t, r.Method, "GET")
			json.NewEncoder(w).Encode(DesiredStateResponse{
				NodeID:    "node-b", // Wrong! Should be "node-a"
				Revision:  3,
				StateHash: "hash123",
				StateJSON: `{"local_routing_table": []}`,
				Status:    "ok",
			})
		case "/api/node/v1/actual-state":
			assertEqual(t, r.Method, "POST")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	client := NewClient(server.URL, "node-a", "test-token")
	config := &Config{
		NodeID:          "node-a",
		ControlPlaneURL: server.URL,
		NodeToken:       "test-token",
	}
	cache := NewCacheManager(dir)
	reconciler := NewReconciler(config, client, cache)

	_, err := reconciler.SyncOnce()
	assertError(t, err)
	assertContains(t, err.Error(), "validation failed")

	// Verify the cache contains the failed state
	actualCache, readErr := cache.ReadActualState()
	assertNoError(t, readErr)
	assertEqual(t, actualCache.Status, "failed")
	assertEqual(t, actualCache.AppliedRevision, 3)
	assertEqual(t, actualCache.StateHash, "hash123")
	assertContains(t, actualCache.LastError, "validation failed")

	// Verify the call sequence: heartbeat -> desired-state -> actual-state
	assertEqual(t, len(callSequence), 3)
	assertEqual(t, callSequence[0], "/api/node/v1/heartbeat")
	assertEqual(t, callSequence[1], "/api/node/v1/desired-state")
	assertEqual(t, callSequence[2], "/api/node/v1/actual-state")
}

// ============================================================================
// Relay Plan tests (5 tests)
// ============================================================================

func TestBuildPlanAvailable(t *testing.T) {
	builder := NewRelayPlanBuilder()

	decision := &RoutingDecision{
		Domain:     "example.com",
		Status:     "available",
		RouteID:    "route-42",
		ServiceID:  "svc-web",
		EndpointID: "ep-main",
		TargetNodeID: "node-b",
		SelectedCandidate: &CandidateEntry{
			GatewayURL: "http://gateway-a:80",
			Mode:       "remote_gateway",
			Priority:   1,
		},
	}

	plan := builder.BuildPlan(decision, "GET")

	assertTrue(t, plan.Available)
	assertEqual(t, plan.Method, "GET")
	assertEqual(t, plan.RouteID, "route-42")
	assertEqual(t, plan.ServiceID, "svc-web")
	assertContains(t, plan.GatewayURL, "/__aegis/relay")
	assertContains(t, plan.GatewayURL, "http://gateway-a:80")

	// Headers
	assertEqual(t, plan.Headers["X-Aegis-Route-ID"], "route-42")
	assertEqual(t, plan.Headers["X-Aegis-Hop"], "1")

	// No Gateway-Link-ID header because RequiresGatewayLink is false
	_, hasLinkHeader := plan.Headers["X-Aegis-Gateway-Link-ID"]
	assertFalse(t, hasLinkHeader)

	// Empty reason for available plan
	assertEqual(t, plan.Reason, "")
}

func TestBuildPlanUnavailable(t *testing.T) {
	builder := NewRelayPlanBuilder()

	decision := &RoutingDecision{
		Domain:            "example.com",
		Status:            "unavailable",
		UnavailableReason: "domain not found in routing table",
	}

	plan := builder.BuildPlan(decision, "POST")

	assertFalse(t, plan.Available)
	assertEqual(t, plan.Method, "POST")
	assertEqual(t, plan.Reason, "domain not found in routing table")
	assertEqual(t, plan.GatewayURL, "")
}

func TestBuildPlanPreserveHost(t *testing.T) {
	builder := NewRelayPlanBuilder()

	decision := &RoutingDecision{
		Domain:     "example.com",
		Status:     "available",
		RouteID:    "route-1",
		ServiceID:  "svc-1",
		SelectedCandidate: &CandidateEntry{
			GatewayURL: "http://gateway:80",
			Mode:       "remote_gateway",
		},
	}

	plan := builder.BuildPlan(decision, "GET")

	assertTrue(t, plan.Available)
	assertTrue(t, plan.PreserveHost)
}

func TestBuildPlanNoRawToken(t *testing.T) {
	builder := NewRelayPlanBuilder()

	decision := &RoutingDecision{
		Domain:     "example.com",
		Status:     "available",
		RouteID:    "route-secure",
		ServiceID:  "svc-secure",
		SelectedCandidate: &CandidateEntry{
			GatewayURL:        "http://gateway-secure:80",
			Mode:              "remote_gateway",
			GatewayLinkID:     "gl-uuid-abc-123", // Not a raw hex token
			RequiresGatewayLink: true,
		},
	}

	plan := builder.BuildPlan(decision, "GET")

	assertTrue(t, plan.Available)

	// GatewayLinkID should be in headers (it's an ID, not a secret)
	assertEqual(t, plan.Headers["X-Aegis-Gateway-Link-ID"], "gl-uuid-abc-123")

	// No field in the plan should contain 64+ consecutive hex characters
	assertFalse(t, ContainsRawToken(plan.GatewayURL))
	assertFalse(t, ContainsRawToken(plan.RouteID))
	assertFalse(t, ContainsRawToken(plan.ServiceID))
	assertFalse(t, ContainsRawToken(plan.Method))
	assertFalse(t, ContainsRawToken(plan.Reason))
		for k, v := range plan.Headers {
			if ContainsRawToken(k) {
				t.Errorf("header key contains raw token: %s", k)
			}
			if ContainsRawToken(v) {
				t.Errorf("header %s value contains raw token: %s", k, v)
			}
		}
}

func TestRelayPlanSafeString(t *testing.T) {
	// Available plan: SafeString should show method, URL, route but NOT headers
	plan := &RelayRequestPlan{
		Method:      "GET",
		GatewayURL:  "http://secure-gateway:80/__aegis/relay",
		RouteID:     "route-1",
		Available:   true,
		PreserveHost: true,
		Headers: map[string]string{
			"X-Aegis-Route-ID":         "route-1",
			"X-Aegis-Hop":              "1",
			"X-Aegis-Gateway-Link-ID":  "gl-abc-123",
			"Authorization":            "Bearer abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		},
	}

	s := plan.SafeString()
	assertContains(t, s, "RelayPlan{")
	assertContains(t, s, "GET")
	assertContains(t, s, "http://secure-gateway:80/__aegis/relay")
	assertContains(t, s, "route-1")

	// Must NOT leak headers (especially sensitive ones)
	assertNotContains(t, s, "X-Aegis-Gateway-Link-ID")
	assertNotContains(t, s, "gl-abc-123")
	assertNotContains(t, s, "Authorization")
	assertNotContains(t, s, "Bearer")
	assertNotContains(t, s, "abcdef0123456789")

	// Unavailable plan: SafeString should show reason
	plan2 := &RelayRequestPlan{
		Available: false,
		Reason:    "domain not found",
	}
	s2 := plan2.SafeString()
	assertContains(t, s2, "unavailable")
	assertContains(t, s2, "domain not found")
}

// ============================================================================
// Additional edge-case tests for completeness
// ============================================================================

func TestValidateRoutingTableNegativeRevision(t *testing.T) {
	table := &RoutingTableCache{
		Revision: -1, // Negative revision is invalid
		Entries:  []RoutingTableEntry{},
	}

	result := ValidateRoutingTable("node-a", table)
	requireInvalid(t, result, "negative revision")
}

func TestValidateDesiredStateForNodeEmptyHash(t *testing.T) {
	ds := &DesiredStateCache{
		NodeID:    "node-a",
		Revision:  1,
		StateHash: "", // Empty hash should fail
		StateJSON: `{"local_routing_table": []}`,
	}

	result := ValidateDesiredStateForNode("node-a", ds)
	requireInvalid(t, result, "hash")
}

func TestValidateDesiredStateForNodeEmptyJSON(t *testing.T) {
	ds := &DesiredStateCache{
		NodeID:    "node-a",
		Revision:  1,
		StateHash: "sha256:abc",
		StateJSON: "", // Empty JSON should fail
	}

	result := ValidateDesiredStateForNode("node-a", ds)
	requireInvalid(t, result, "state_json")
}

func TestValidateDesiredStateForNodeRawToken(t *testing.T) {
	// StateJSON containing 64 consecutive hex chars should be rejected
	ds := &DesiredStateCache{
		NodeID:    "node-a",
		Revision:  1,
		StateHash: "sha256:abc",
		StateJSON: `{"token":"` + strings.Repeat("a", 64) + `"}`,
	}

	result := ValidateDesiredStateForNode("node-a", ds)
	requireInvalid(t, result, "raw token")
}

func TestCacheReadNonExistent(t *testing.T) {
	dir := t.TempDir()
	cm := NewCacheManager(dir)

	_, err := cm.ReadDesiredState()
	assertError(t, err)
	assertContains(t, err.Error(), "not found")

	_, err = cm.ReadRoutingTable()
	assertError(t, err)
	assertContains(t, err.Error(), "not found")
}

func TestContainsRawTokenEdgeCases(t *testing.T) {
	// Exactly 31 chars => false (len < 32)
	assertFalse(t, ContainsRawToken("abcdef0123456789abcdef0123456")) // 31 chars

	// 63 hex chars => false (hexCount never reaches 64)
	sixtyThree := strings.Repeat("a", 63)
	assertFalse(t, ContainsRawToken(sixtyThree))

	// 64 hex chars => true
	sixtyFour := strings.Repeat("a", 64)
	assertTrue(t, ContainsRawToken(sixtyFour))

	// Empty string => false
	assertFalse(t, ContainsRawToken(""))

	// Single character => false
	assertFalse(t, ContainsRawToken("0"))

	// Pure numeric hex: 64 zeros => true
	assertTrue(t, ContainsRawToken(strings.Repeat("0", 64)))

	// Mixed hex with non-hex char in the middle after 64 hex => true
	// 64 hex chars followed by 'x' - the check returns true at position 64 before seeing 'x'
	assertTrue(t, ContainsRawToken(strings.Repeat("a", 64)+"x"))
}

func TestResolverEmptyTable(t *testing.T) {
	r := NewResolver(&RoutingTableCache{Entries: []RoutingTableEntry{}})
	d := r.Resolve("anything.com")
	assertEqual(t, d.Status, "unavailable")
	assertContains(t, d.UnavailableReason, "not found")
}

func TestResolverNonAvailableStatus(t *testing.T) {
	table := &RoutingTableCache{
		Entries: []RoutingTableEntry{
			{
				Domain:       "example.com",
				FromNodeID:   "node-a",
				TargetNodeID: "node-a",
				Status:       "pending", // Neither "available" nor "disabled"
				Candidates: []CandidateEntry{
					{Mode: "local_gateway", GatewayURL: "http://127.0.0.1:80"},
				},
			},
		},
	}

	r := NewResolver(table)
	d := r.Resolve("example.com")

	assertEqual(t, d.Status, "pending")
	assertContains(t, d.UnavailableReason, "not available")
	assertNil(t, d.SelectedCandidate)
}

func TestBuildPlanUnavailableNoCandidate(t *testing.T) {
	builder := NewRelayPlanBuilder()

	// Available status but nil SelectedCandidate => unavailable plan
	decision := &RoutingDecision{
		Domain:            "example.com",
		Status:            "available",
		UnavailableReason: "no candidates",
		// SelectedCandidate is nil
	}

	plan := builder.BuildPlan(decision, "GET")
	assertFalse(t, plan.Available)
	assertEqual(t, plan.Reason, "no candidates")
}

func TestBuildPlanWithGatewayLink(t *testing.T) {
	builder := NewRelayPlanBuilder()

	decision := &RoutingDecision{
		Domain:  "example.com",
		Status:  "available",
		RouteID: "route-gl",
		SelectedCandidate: &CandidateEntry{
			GatewayURL:         "http://gateway:80",
			Mode:               "remote_gateway",
			GatewayLinkID:      "gl-xyz-789",
			RequiresGatewayLink: true,
		},
	}

	plan := builder.BuildPlan(decision, "PUT")

	assertTrue(t, plan.Available)
	assertEqual(t, plan.Headers["X-Aegis-Gateway-Link-ID"], "gl-xyz-789")
	assertEqual(t, plan.Method, "PUT")
}

func TestRelayPlanSafeStringUnavailable(t *testing.T) {
	plans := []*RelayRequestPlan{
		{Available: false, Reason: "domain not found"},
		{Available: false, Reason: "no candidates"},
		{Available: false, Reason: ""},
	}

	for _, p := range plans {
		s := p.SafeString()
		assertContains(t, s, "unavailable")
	}
}

func TestEnsureDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "cache", "path")
	cm := NewCacheManager(dir)

	err := cm.EnsureDir()
	assertNoError(t, err)

	// Verify directory exists
	info, err := os.Stat(dir)
	assertNoError(t, err)
	assertTrue(t, info.IsDir())
}

func TestCacheWriteRoutingTableAutoCreateDir(t *testing.T) {
	// The atomicWrite function should auto-create directories
	deepDir := filepath.Join(t.TempDir(), "a", "b", "c")
	cm := NewCacheManager(deepDir)

	cache := &RoutingTableCache{
		NodeID:   "test",
		Revision: 1,
		Entries:  []RoutingTableEntry{},
	}

	err := cm.WriteRoutingTable(cache)
	assertNoError(t, err)
	assertTrue(t, cm.CacheFileExists(RoutingTableCacheFile))
}
