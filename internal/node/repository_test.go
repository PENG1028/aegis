package node

import (
	"path/filepath"
	"testing"
	"time"

	"aegis/internal/store"
)

func TestTouchLivenessPreservesCatalogFields(t *testing.T) {
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "aegis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.Initialize(db); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	now := time.Now().Add(-time.Hour)
	n := &NodeRecord{
		ID:           "node-row",
		NodeID:       "node_VM-0-4-ubuntu",
		Name:         "VM-0-4-ubuntu",
		Role:         RoleWorker,
		Status:       StatusOffline,
		Hostname:     "VM-0-4-ubuntu",
		LocalIP:      "127.0.0.1",
		PrivateIP:    "10.3.0.4",
		PublicIP:     "43.160.211.232",
		Capabilities: DefaultCapabilities(),
		LastSeen:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := repo.Create(n); err != nil {
		t.Fatal(err)
	}

	heartbeatAt := time.Now().Round(time.Second)
	if err := repo.TouchLiveness(n.NodeID, StatusOnline, "", heartbeatAt); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByNodeID(n.NodeID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusOnline {
		t.Fatalf("status = %q, want online", got.Status)
	}
	if !got.LastHeartbeatAt.Equal(heartbeatAt) {
		t.Fatalf("last heartbeat = %s, want %s", got.LastHeartbeatAt, heartbeatAt)
	}
	if got.PublicIP != n.PublicIP || got.PrivateIP != n.PrivateIP || got.Hostname != n.Hostname {
		t.Fatalf("catalog fields changed: public=%q private=%q hostname=%q", got.PublicIP, got.PrivateIP, got.Hostname)
	}
}
