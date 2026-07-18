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
