package handlers

import (
	"testing"

	"aegis/internal/distnode"
	"aegis/internal/node"
)

func TestMergeMembershipNodesCanonicalizesLegacyPeerID(t *testing.T) {
	nodes := []node.NodeRecord{
		{
			NodeID:   "node_VM-0-4-ubuntu",
			Name:     "VM-0-4-ubuntu",
			Hostname: "VM-0-4-ubuntu",
			Status:   node.StatusOffline,
		},
	}
	dn := distnode.New(distnode.Config{
		ID:     "node_VM-0-11-ubuntu",
		Secret: "test-secret",
		Peers: []distnode.PeerConfig{
			{ID: "VM-0-4-ubuntu", Addr: "43.160.211.232:80"},
		},
	})
	peer := dn.Membership.GetPeer("VM-0-4-ubuntu")
	if peer == nil {
		t.Fatal("peer missing")
	}
	peer.Alive = true

	merged := mergeMembershipNodes(nodes, dn)
	if len(merged) != 1 {
		t.Fatalf("merged node count = %d, want 1: %#v", len(merged), merged)
	}
	if merged[0].NodeID != "node_VM-0-4-ubuntu" {
		t.Fatalf("node_id = %q, want stable node id", merged[0].NodeID)
	}
	if merged[0].Status != node.StatusOnline {
		t.Fatalf("status = %q, want online", merged[0].Status)
	}
}

func TestMergeMembershipNodesCreatesStableVirtualNode(t *testing.T) {
	dn := distnode.New(distnode.Config{
		ID:     "node_VM-0-11-ubuntu",
		Secret: "test-secret",
		Peers: []distnode.PeerConfig{
			{ID: "VM-0-4-ubuntu", Addr: "43.160.211.232:80"},
		},
	})

	merged := mergeMembershipNodes(nil, dn)
	if len(merged) != 1 {
		t.Fatalf("merged node count = %d, want 1", len(merged))
	}
	if merged[0].NodeID != "node_VM-0-4-ubuntu" {
		t.Fatalf("node_id = %q, want stable node id", merged[0].NodeID)
	}
	if merged[0].Name != "VM-0-4-ubuntu" {
		t.Fatalf("name = %q, want legacy display name", merged[0].Name)
	}
}
