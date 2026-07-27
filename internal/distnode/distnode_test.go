package distnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := Config{
		ID:     "node_a",
		Name:   "test-node",
		Addr:   "127.0.0.1:0",
		Secret: "test-secret",
		Peers: []PeerConfig{
			{ID: "node_b", Addr: "127.0.0.1:9999"},
		},
	}
	dn := New(cfg)
	if dn == nil {
		t.Fatal("New() returned nil")
	}
	if dn.ID != "node_a" {
		t.Errorf("ID = %q, want %q", dn.ID, "node_a")
	}
	if dn.Role.Current() != "node" {
		t.Errorf("default role = %q, want %q", dn.Role.Current(), "node")
	}
	if dn.Config.HealthPath != "/healthz" {
		t.Errorf("default health path = %q, want %q", dn.Config.HealthPath, "/healthz")
	}
	if dn.Config.CallPath != "/distnode/call" {
		t.Errorf("default call path = %q, want %q", dn.Config.CallPath, "/distnode/call")
	}
	if dn.Config.NodeIDHeader != "X-DistNode-ID" {
		t.Errorf("default node header = %q, want %q", dn.Config.NodeIDHeader, "X-DistNode-ID")
	}
}

func TestCustomConfigSurface(t *testing.T) {
	cfg := Config{
		ID:           "node_a",
		Scheme:       "http://",
		HealthPath:   "ready",
		CallPath:     "rpc/call",
		NodeIDHeader: "X-Runner-Node-ID",
		DefaultRole:  "worker",
	}
	dn := New(cfg)

	if dn.Config.Scheme != "http" {
		t.Errorf("scheme = %q, want %q", dn.Config.Scheme, "http")
	}
	if dn.Config.HealthPath != "/ready" {
		t.Errorf("health path = %q, want %q", dn.Config.HealthPath, "/ready")
	}
	if dn.Config.CallPath != "/rpc/call" {
		t.Errorf("call path = %q, want %q", dn.Config.CallPath, "/rpc/call")
	}
	if dn.Role.Current() != "worker" {
		t.Errorf("role = %q, want %q", dn.Role.Current(), "worker")
	}
}

func TestRole(t *testing.T) {
	dn := New(Config{ID: "test"})
	if dn.Role.Is("panel") {
		t.Error("should not be panel by default")
	}
	dn.Role.Declare("panel")
	if !dn.Role.Is("panel") {
		t.Error("should be panel after declare")
	}
	if dn.Role.Current() != "panel" {
		t.Errorf("Current = %q, want %q", dn.Role.Current(), "panel")
	}
}

func TestMembershipPeers(t *testing.T) {
	cfg := Config{
		ID:   "node_a",
		Addr: "127.0.0.1:0",
		Peers: []PeerConfig{
			{ID: "node_b", Addr: "127.0.0.1:9999"},
			{ID: "node_c", Addr: "127.0.0.1:9998"},
		},
	}
	dn := New(cfg)

	if dn.Membership.PeerCount() != 2 {
		t.Errorf("PeerCount = %d, want 2", dn.Membership.PeerCount())
	}

	all := dn.Membership.AllPeers()
	if len(all) != 2 {
		t.Errorf("AllPeers = %d, want 2", len(all))
	}

	// Unknown peer returns nil
	p := dn.Membership.GetPeer("nonexistent")
	if p != nil {
		t.Errorf("GetPeer(nonexistent) = %v, want nil", p)
	}

	// Known peer
	p = dn.Membership.GetPeer("node_b")
	if p == nil {
		t.Fatal("GetPeer(node_b) = nil, want peer")
	}
	if p.Info.ID != "node_b" {
		t.Errorf("peer ID = %q, want %q", p.Info.ID, "node_b")
	}
	if p.Info.Addr != "127.0.0.1:9999" {
		t.Errorf("peer addr = %q, want %q", p.Info.Addr, "127.0.0.1:9999")
	}
}

// TestTransportCall verifies that Transport.Call reaches a peer and
// that the remote side's registered handler is invoked correctly.
func TestTransportCall(t *testing.T) {
	// Node B: set up a real HTTP server
	bCfg := Config{ID: "node_b", Addr: "127.0.0.1:0", Secret: "shared-secret"}
	bDn := New(bCfg)

	bDn.Transport.Register("Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		var req struct {
			Msg string `json:"msg"`
		}
		json.Unmarshal(args, &req)
		return map[string]string{"reply": "pong: " + req.Msg}, nil
	})

	bMux := http.NewServeMux()
	bMux.Handle("POST "+bDn.Config.CallPath, bDn.Transport.Handler())
	bSrv := httptest.NewServer(bMux)
	defer bSrv.Close()

	// Node A: config points to node B
	aCfg := Config{
		ID:     "node_a",
		Addr:   "127.0.0.1:0",
		Secret: "shared-secret",
		Peers:  []PeerConfig{{ID: "node_b", Addr: bSrv.Listener.Addr().String()}},
	}
	aDn := New(aCfg)

	// Manually mark node_b as alive (no health check running)
	peer := aDn.Membership.GetPeer("node_b")
	peer.Alive = true
	peer.AliveAt = time.Now()
	peer.Info.Status = StatusAlive

	// Call node B from node A
	var result map[string]string
	err := aDn.Transport.Call(context.Background(), "node_b", "Ping", map[string]string{"msg": "hello"}, &result)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result["reply"] != "pong: hello" {
		t.Errorf("reply = %q, want %q", result["reply"], "pong: hello")
	}
}

func TestTransportCallCustomPathAndHeader(t *testing.T) {
	var gotCallerID string
	bCfg := Config{
		ID:           "node_b",
		Addr:         "127.0.0.1:0",
		Secret:       "shared-secret",
		CallPath:     "/runner/dist/call",
		NodeIDHeader: "X-Runner-Node-ID",
	}
	bDn := New(bCfg)
	bDn.Transport.Register("Ping", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		gotCallerID = callerID
		return map[string]string{"reply": "pong"}, nil
	})

	bMux := http.NewServeMux()
	bMux.Handle("POST /runner/dist/call", bDn.Transport.Handler())
	bSrv := httptest.NewServer(bMux)
	defer bSrv.Close()

	aCfg := Config{
		ID:           "node_a",
		Addr:         "127.0.0.1:0",
		Secret:       "shared-secret",
		CallPath:     "/runner/dist/call",
		NodeIDHeader: "X-Runner-Node-ID",
		Peers:        []PeerConfig{{ID: "node_b", Addr: bSrv.Listener.Addr().String()}},
	}
	aDn := New(aCfg)
	peer := aDn.Membership.GetPeer("node_b")
	peer.Alive = true
	peer.Info.Status = StatusAlive

	var result map[string]string
	err := aDn.Transport.Call(context.Background(), "node_b", "Ping", map[string]string{}, &result)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if result["reply"] != "pong" {
		t.Errorf("reply = %q, want %q", result["reply"], "pong")
	}
	if gotCallerID != "node_a" {
		t.Errorf("callerID = %q, want %q", gotCallerID, "node_a")
	}
}

// TestTransportCallDeadPeer verifies Call returns ErrPeerDead for dead peers.
func TestTransportCallDeadPeer(t *testing.T) {
	cfg := Config{
		ID:     "node_a",
		Addr:   "127.0.0.1:0",
		Secret: "test-secret",
		Peers:  []PeerConfig{{ID: "node_b", Addr: "127.0.0.1:19999"}},
	}
	dn := New(cfg)
	// Peer is dead by default (never checked in)

	var reply any
	err := dn.Transport.Call(context.Background(), "node_b", "AnyMethod", nil, &reply)
	if !errors.Is(err, ErrPeerDead) {
		t.Errorf("expected ErrPeerDead, got %v", err)
	}
}

// TestTransportCallUnknownPeer verifies Call returns ErrPeerNotFound.
func TestTransportCallUnknownPeer(t *testing.T) {
	dn := New(Config{ID: "test"})
	var reply any
	err := dn.Transport.Call(context.Background(), "unknown", "AnyMethod", nil, &reply)
	if !errors.Is(err, ErrPeerNotFound) {
		t.Errorf("expected ErrPeerNotFound, got %v", err)
	}
}

// TestTransportUnauthorized verifies the /call handler rejects bad tokens.
func TestTransportUnauthorized(t *testing.T) {
	dn := New(Config{ID: "node_b", Secret: "real-secret"})
	mux := http.NewServeMux()
	mux.Handle("POST "+dn.Config.CallPath, dn.Transport.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Wrong secret
	req, _ := http.NewRequest("POST", srv.URL+dn.Config.CallPath,
		bytes.NewReader([]byte(`{"method":"Ping"}`)))
	req.Header.Set("Authorization", "Bearer wrong-secret")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestTransportHandlerNotFound tests a non-existent method returns 404.
func TestTransportHandlerNotFound(t *testing.T) {
	dn := New(Config{ID: "test", Secret: "sec"})
	dn.Transport.Register("Exists", func(ctx context.Context, callerID string, args json.RawMessage) (any, error) {
		return "ok", nil
	})

	mux := http.NewServeMux()
	mux.Handle("POST "+dn.Config.CallPath, dn.Transport.Handler())
	srv := httptest.NewServer(mux)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+dn.Config.CallPath,
		bytes.NewReader([]byte(`{"method":"DoesNotExist"}`)))
	req.Header.Set("Authorization", "Bearer sec")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// TestHealthCheck verifies the membership health-check loop detects alive peers.
// This test runs the actual health check loop briefly.
func TestHealthCheck(t *testing.T) {
	// Server: node B with the default health endpoint.
	bMux := http.NewServeMux()
	bMux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	bSrv := httptest.NewServer(bMux)
	defer bSrv.Close()

	cfg := Config{
		ID:   "node_a",
		Addr: bSrv.Listener.Addr().String(), // use same addr for health check endpoint
		Peers: []PeerConfig{{
			ID:   "node_b",
			Addr: bSrv.Listener.Addr().String(),
		}},
	}
	dn := New(cfg)

	// Verify peer starts dead
	peer := dn.Membership.GetPeer("node_b")
	if peer == nil {
		t.Fatal("peer not found")
	}

	// Run one health check cycle manually
	dn.Membership.checkOne(context.Background(), peer)

	if !peer.Alive {
		t.Errorf("peer should be alive after health check")
	}
	if peer.FailCount != 0 {
		t.Errorf("FailCount = %d, want 0", peer.FailCount)
	}
}

func TestHealthCheckCustomPath(t *testing.T) {
	bMux := http.NewServeMux()
	bMux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	bSrv := httptest.NewServer(bMux)
	defer bSrv.Close()

	dn := New(Config{
		ID:         "node_a",
		HealthPath: "ready",
		Peers: []PeerConfig{{
			ID:   "node_b",
			Addr: bSrv.Listener.Addr().String(),
		}},
	})
	peer := dn.Membership.GetPeer("node_b")
	if peer == nil {
		t.Fatal("peer not found")
	}

	dn.Membership.checkOne(context.Background(), peer)

	if !peer.Alive {
		t.Errorf("peer should be alive after custom health check")
	}
}

// TestInfo verifies Info() returns correct data.
func TestInfo(t *testing.T) {
	cfg := Config{ID: "node_x", Name: "My Node", Addr: "10.0.0.1:7380", Secret: "s"}
	dn := New(cfg)
	dn.Role.Declare("panel")

	info := dn.Info()
	if info.ID != "node_x" {
		t.Errorf("Info.ID = %q", info.ID)
	}
	if info.Name != "My Node" {
		t.Errorf("Info.Name = %q", info.Name)
	}
	if info.Addr != "10.0.0.1:7380" {
		t.Errorf("Info.Addr = %q", info.Addr)
	}
	if info.Role != "panel" {
		t.Errorf("Info.Role = %q", info.Role)
	}
}
