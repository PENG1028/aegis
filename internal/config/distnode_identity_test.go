package config

import (
	"path/filepath"
	"testing"
)

func TestApplyDistNodeDefaultsCanonicalizesAegisNodeIDs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DistNode.Enabled = true
	cfg.DistNode.ID = "VM-0-11-ubuntu"
	cfg.DistNode.Name = "VM-0-11-ubuntu"
	cfg.DistNode.Secret = "test-secret"
	cfg.DistNode.Peers = []DistNodePeer{
		{ID: "VM-0-4-ubuntu", Addr: "43.160.211.232:80"},
		{ID: "node_VM-0-5-ubuntu", Addr: "43.160.211.233:80"},
	}

	applyDistNodeDefaults(cfg, filepath.Join(t.TempDir(), "config.yaml"))

	if cfg.DistNode.ID != "node_VM-0-11-ubuntu" {
		t.Fatalf("distnode id = %q, want stable node id", cfg.DistNode.ID)
	}
	if cfg.DistNode.Peers[0].ID != "node_VM-0-4-ubuntu" {
		t.Fatalf("peer id = %q, want stable node id", cfg.DistNode.Peers[0].ID)
	}
	if cfg.DistNode.Peers[1].ID != "node_VM-0-5-ubuntu" {
		t.Fatalf("stable peer id changed to %q", cfg.DistNode.Peers[1].ID)
	}
}
