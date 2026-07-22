package handlers

import (
	"strings"
	"testing"

	"aegis/internal/config"
	"aegis/internal/distnode/onboarding"

	"gopkg.in/yaml.v3"
)

func TestRenderNodeServeConfigUsesControlProvider(t *testing.T) {
	out, err := renderNodeServeConfig(config.ProxyConfig{Provider: "haproxy"}, "node-b", "adm-test", "secret-test", "node-a", "43.159.34.11:80")
	if err != nil {
		t.Fatalf("renderNodeServeConfig: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal([]byte(out), &cfg); err != nil {
		t.Fatalf("unmarshal rendered config: %v\n%s", err, out)
	}

	if cfg.Proxy.Provider != "haproxy" {
		t.Fatalf("provider = %q, want haproxy", cfg.Proxy.Provider)
	}
	if cfg.Proxy.CaddyfilePath != "/etc/haproxy/haproxy.cfg" {
		t.Fatalf("config path = %q, want /etc/haproxy/haproxy.cfg", cfg.Proxy.CaddyfilePath)
	}
	if !strings.Contains(cfg.Proxy.ValidateCommand, "haproxy -c") {
		t.Fatalf("validate command = %q, want haproxy validation", cfg.Proxy.ValidateCommand)
	}
	if cfg.DistNode.Peers[0].Addr != "43.159.34.11:80" {
		t.Fatalf("peer addr = %q, want control edge", cfg.DistNode.Peers[0].Addr)
	}
}

func TestNodeProxyConfigIgnoresDevelopmentPaths(t *testing.T) {
	proxy := nodeProxyConfig(config.ProxyConfig{
		Provider:      "caddy",
		CaddyfilePath: ".aegis/Caddyfile",
		BackupDir:     ".aegis/backups",
	})

	if proxy.CaddyfilePath != "/etc/caddy/Caddyfile" {
		t.Fatalf("config path = %q, want production caddy path", proxy.CaddyfilePath)
	}
	if proxy.BackupDir != "/var/lib/aegis/backups" {
		t.Fatalf("backup dir = %q, want production backup path", proxy.BackupDir)
	}
}

func TestDeployResponseFromEnsurePreservesLegacyFields(t *testing.T) {
	resp := deployResponseFromEnsure(&onboarding.EnsureResult{
		Success: true,
		NodeID:  "node-b",
		Message: "ok",
	})

	if !resp.Success {
		t.Fatal("Success = false, want true")
	}
	if resp.NodeID != "node-b" {
		t.Fatalf("NodeID = %q, want node-b", resp.NodeID)
	}
	if resp.Message != "ok" {
		t.Fatalf("Message = %q, want ok", resp.Message)
	}
}

func TestNativeSSHDoesNotRequireSystemSSHBinary(t *testing.T) {
	if !isSSHAvailable() {
		t.Fatal("native SSH deployment should not depend on an external ssh binary")
	}
}

func TestResolveControlPeerRejectsLocalhostCurrentMode(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_control"

	_, err := h.resolveControlPeer(DeployNodeRequest{TargetIP: "43.160.211.232"}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error for localhost current mode")
	}
	if !strings.Contains(err.Error(), "push_only") {
		t.Fatalf("error = %q, want push_only guidance", err.Error())
	}
}

func TestResolveControlPeerPushOnlyUsesExplicitControl(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	peer, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlNodeID:   "control-a",
		ControlEdgeAddr: "43.159.34.11",
		ControlSecret:   "cluster-secret",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err != nil {
		t.Fatalf("resolveControlPeer: %v", err)
	}
	if !peer.PushOnly {
		t.Fatal("PushOnly = false, want true")
	}
	if peer.NodeID != "node_control-a" {
		t.Fatalf("NodeID = %q, want node_control-a", peer.NodeID)
	}
	if peer.EdgeAddr != "43.159.34.11:80" {
		t.Fatalf("EdgeAddr = %q, want 43.159.34.11:80", peer.EdgeAddr)
	}
	if peer.Secret != "cluster-secret" {
		t.Fatalf("Secret = %q, want cluster-secret", peer.Secret)
	}
}

func TestResolveControlPeerPushOnlyRequiresControlNodeID(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	_, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlEdgeAddr: "43.159.34.11:80",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error without control_node_id")
	}
	if !strings.Contains(err.Error(), "control_node_id") {
		t.Fatalf("error = %q, want control_node_id guidance", err.Error())
	}
}

func TestResolveControlPeerPushOnlyRequiresControlSecret(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_laptop"

	_, err := h.resolveControlPeer(DeployNodeRequest{
		ControllerMode:  controllerModePushOnly,
		ControlNodeID:   "node_control",
		ControlEdgeAddr: "43.159.34.11:80",
		TargetIP:        "43.160.211.232",
	}, "localhost:7380")
	if err == nil {
		t.Fatal("resolveControlPeer returned nil error without control_secret")
	}
	if !strings.Contains(err.Error(), "control_secret") {
		t.Fatalf("error = %q, want control_secret guidance", err.Error())
	}
}

func TestResolveControlPeerCurrentUsesRequestHost(t *testing.T) {
	h := &Handlers{Config: &config.Config{}}
	h.Config.DistNode.ID = "node_control"
	h.Config.DistNode.Secret = "current-secret"

	peer, err := h.resolveControlPeer(DeployNodeRequest{TargetIP: "43.160.211.232"}, "43.159.34.11:7380")
	if err != nil {
		t.Fatalf("resolveControlPeer: %v", err)
	}
	if peer.PushOnly {
		t.Fatal("PushOnly = true, want false")
	}
	if peer.NodeID != "node_control" {
		t.Fatalf("NodeID = %q, want node_control", peer.NodeID)
	}
	if peer.EdgeAddr != "43.159.34.11:80" {
		t.Fatalf("EdgeAddr = %q, want 43.159.34.11:80", peer.EdgeAddr)
	}
	if peer.Secret != "current-secret" {
		t.Fatalf("Secret = %q, want current-secret", peer.Secret)
	}
}
