package distnode

import "testing"

// Phase 1 per-peer credential + revocation auth tests.
// Verifies backward compatibility (Phase 0 shared secret) is preserved and that
// per-peer secrets enable real per-node revocation.

func newAuthTestTransport(shared string, peers []PeerConfig) *Transport {
	cfg := Config{ID: "self", Secret: shared, Peers: peers}
	return newTransport(cfg, newMembership(cfg))
}

func TestAuth_SharedSecretFallback_Phase0(t *testing.T) {
	tr := newAuthTestTransport("clustersecret", nil)

	// No caller ID, valid shared secret → accepted (exactly Phase 0).
	if _, err := tr.authenticateCaller("", "Bearer clustersecret"); err != nil {
		t.Fatalf("shared secret should authenticate: %v", err)
	}
	// Wrong secret → rejected.
	if _, err := tr.authenticateCaller("", "Bearer wrong"); err == nil {
		t.Fatal("wrong shared secret must be rejected")
	}
	// Empty auth → rejected.
	if _, err := tr.authenticateCaller("nodeA", ""); err == nil {
		t.Fatal("empty auth must be rejected")
	}
}

func TestAuth_PerPeerSecret_Strict(t *testing.T) {
	tr := newAuthTestTransport("clustersecret", []PeerConfig{
		{ID: "nodeA", Addr: "10.0.0.1:80", Secret: "secretA"},
	})

	// nodeA presents its own per-peer secret → accepted, callerID returned.
	id, err := tr.authenticateCaller("nodeA", "Bearer secretA")
	if err != nil {
		t.Fatalf("per-peer secret should authenticate: %v", err)
	}
	if id != "nodeA" {
		t.Fatalf("expected callerID nodeA, got %q", id)
	}

	// nodeA presenting the SHARED secret must be rejected — a peer with a
	// per-peer secret is verified strictly against it (no shared fallback).
	if _, err := tr.authenticateCaller("nodeA", "Bearer clustersecret"); err == nil {
		t.Fatal("peer with per-peer secret must not accept the shared secret")
	}
}

func TestAuth_KnownPeerWithoutSecret_UsesShared(t *testing.T) {
	tr := newAuthTestTransport("clustersecret", []PeerConfig{
		{ID: "nodeB", Addr: "10.0.0.2:80"}, // no per-peer secret
	})
	// Known peer, no per-peer secret → shared secret path.
	id, err := tr.authenticateCaller("nodeB", "Bearer clustersecret")
	if err != nil {
		t.Fatalf("known peer w/o secret should use shared: %v", err)
	}
	if id != "nodeB" {
		t.Fatalf("expected callerID nodeB, got %q", id)
	}
}

func TestAuth_RevokedPeer_Rejected(t *testing.T) {
	tr := newAuthTestTransport("clustersecret", []PeerConfig{
		{ID: "nodeA", Addr: "10.0.0.1:80", Secret: "secretA"},
	})
	// Before revoke: valid.
	if _, err := tr.authenticateCaller("nodeA", "Bearer secretA"); err != nil {
		t.Fatalf("pre-revoke should authenticate: %v", err)
	}
	// Revoke, then even a valid per-peer secret is rejected.
	if !tr.members.RevokePeer("nodeA") {
		t.Fatal("RevokePeer should return true for a known peer")
	}
	if _, err := tr.authenticateCaller("nodeA", "Bearer secretA"); err == nil {
		t.Fatal("revoked peer must be rejected even with a valid secret")
	}
	// Other nodes unaffected: shared-secret caller still works.
	if _, err := tr.authenticateCaller("", "Bearer clustersecret"); err != nil {
		t.Fatalf("revoking one peer must not affect others: %v", err)
	}
}

func TestRevokePeer_UnknownReturnsFalse(t *testing.T) {
	tr := newAuthTestTransport("s", nil)
	if tr.members.RevokePeer("ghost") {
		t.Fatal("RevokePeer on unknown peer should return false")
	}
}
