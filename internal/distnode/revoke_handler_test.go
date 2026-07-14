package distnode_test

import (
	"context"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"aegis/internal/distnode"
)

// Tests for the per-peer credential + revocation handler flow, exercising the
// full Transport.Handler() → authenticateCaller → Membership.RevokePeer path.

func newTestDistNode(name string, peers []distnode.PeerConfig) *distnode.DistNode {
	return distnode.New(distnode.Config{
		ID:     name,
		Name:   name,
		Addr:   "127.0.0.1:7380",
		Secret: "cluster-secret",
		Peers:  peers,
	})
}

// postCall constructs a distnode call request and marshals it to an io.ReadCloser.
func postCall(method string, args any) io.ReadCloser {
	body, _ := json.Marshal(map[string]any{"method": method, "args": args})
	return wrapReadCloser(body)
}

func wrapReadCloser(body []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(body))
}

func TestRevokeHandler_RejectsAfterRevoke(t *testing.T) {
	dn := newTestDistNode("panel", []distnode.PeerConfig{
		{ID: "node-b", Addr: "10.0.0.2:80", Secret: "b-secret"},
	})

	// Register a test handler
	hitTarget := false
	dn.Transport.Register("Test.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		hitTarget = true
		return map[string]string{"ok": "pong"}, nil
	})

	handler := dn.Transport.Handler()

	// Before revoke: valid per-peer auth passes
	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer b-secret")
	req.Header.Set("X-Aegis-Node-ID", "node-b")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !hitTarget {
		t.Fatalf("pre-revoke should succeed: code=%d hit=%v body=%s", w.Code, hitTarget, w.Body.String())
	}

	// Revoke node-b
	if !dn.Membership.RevokePeer("node-b") {
		t.Fatal("RevokePeer should return true")
	}

	// After revoke: same valid per-peer secret is rejected (401)
	hitTarget = false
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req2.Header.Set("Authorization", "Bearer b-secret")
	req2.Header.Set("X-Aegis-Node-ID", "node-b")
	req2.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("post-revoke should return 401: got %d", w2.Code)
	}
	if hitTarget {
		t.Fatal("revoked peer's handler must not be called")
	}
}

func TestRevokeHandler_OnlyAffectsTarget(t *testing.T) {
	dn := newTestDistNode("panel", []distnode.PeerConfig{
		{ID: "node-b", Addr: "10.0.0.2:80", Secret: "b-secret"},
		{ID: "node-c", Addr: "10.0.0.3:80", Secret: "c-secret"},
	})

	dn.Transport.Register("Test.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		return map[string]string{"ok": "pong"}, nil
	})

	// Revoke node-b only
	dn.Membership.RevokePeer("node-b")

	// node-c should still authenticate with its own per-peer secret
	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer c-secret")
	req.Header.Set("X-Aegis-Node-ID", "node-c")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("node-c still authenticates after node-b revoked: got %d body=%s",
			w.Code, w.Body.String())
	}
}

func TestRevokeHandler_LegacyCaller_SharedSecret(t *testing.T) {
	// Phase-0 cluster: no per-peer secrets, no node-id header. Must still work.
	dn := newTestDistNode("panel", nil)
	dn.Transport.Register("Test.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		return map[string]string{"ok": "pong"}, nil
	})

	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer cluster-secret")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Phase-0 caller must still authenticate: got %d body=%s",
			w.Code, w.Body.String())
	}
}

func TestRevokeHandler_MixedCluster(t *testing.T) {
	// Mixed: node-b has a per-peer secret, node-c uses shared secret.
	dn := newTestDistNode("panel", []distnode.PeerConfig{
		{ID: "node-b", Addr: "10.0.0.2:80", Secret: "b-secret"},
		{ID: "node-c", Addr: "10.0.0.3:80"}, // no per-peer → shared fallback
	})
	dn.Transport.Register("Test.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		return map[string]string{"ok": "pong"}, nil
	})

	// node-c with shared secret → should authenticate
	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer cluster-secret")
	req.Header.Set("X-Aegis-Node-ID", "node-c")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("mixed: node-c shared should pass: got %d body=%s", w.Code, w.Body.String())
	}

	// node-b with wrong secret → rejected
	req2 := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req2.Header.Set("Authorization", "Bearer cluster-secret")
	req2.Header.Set("X-Aegis-Node-ID", "node-b")
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w2, req2)
	if w2.Code == http.StatusOK {
		t.Fatal("mixed: node-b has per-peer secret — shared secret must NOT work for it")
	}
}

func TestRevokeHandler_WrongSecret(t *testing.T) {
	dn := newTestDistNode("panel", []distnode.PeerConfig{
		{ID: "node-b", Addr: "10.0.0.2:80", Secret: "b-secret"},
	})

	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer wrong-secret")
	req.Header.Set("X-Aegis-Node-ID", "node-b")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong secret must be rejected: got %d", w.Code)
	}
}

func TestRevokeHandler_MissingAuth(t *testing.T) {
	dn := newTestDistNode("panel", nil)
	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("missing auth header must be rejected: got %d", w.Code)
	}
}

func TestRevokeHandler_ShareSecretWrong(t *testing.T) {
	dn := newTestDistNode("panel", nil)
	dn.Transport.Register("Test.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		return map[string]string{"ok": "pong"}, nil
	})

	req := httptest.NewRequest("POST", "/api/distnode/v1/call", postCall("Test.Ping", map[string]string{}))
	req.Header.Set("Authorization", "Bearer wrong")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	dn.Transport.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong shared secret must be rejected: got %d", w.Code)
	}
}
