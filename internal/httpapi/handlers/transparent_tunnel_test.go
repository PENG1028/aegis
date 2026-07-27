package handlers

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"aegis/internal/endpoint"
	"aegis/internal/node"
	"aegis/internal/store"
)

func TestTransparentTunnelAllowedRequiresLocalEnabledEndpoint(t *testing.T) {
	h := newTunnelTestHandlers(t)

	if !h.transparentTunnelAllowed("svc_runsping", "node_a", 9100) {
		t.Fatal("expected registered local endpoint to be allowed")
	}
	if h.transparentTunnelAllowed("svc_runsping", "node_a", 22) {
		t.Fatal("unexpectedly allowed unregistered port")
	}
	if h.transparentTunnelAllowed("svc_runsping", "node_b", 9100) {
		t.Fatal("unexpectedly allowed non-current target node")
	}
	if h.transparentTunnelAllowed("svc_other", "node_a", 9100) {
		t.Fatal("unexpectedly allowed wrong service")
	}
	if h.transparentTunnelAllowed("svc_admin", "node_a", 7380) {
		t.Fatal("unexpectedly allowed control-plane port")
	}
}

func newTunnelTestHandlers(t *testing.T) *Handlers {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	nodeRepo := node.NewRepository(db)
	endpointRepo := endpoint.NewRepository(db)
	now := time.Now()
	if err := nodeRepo.Create(&node.NodeRecord{
		ID:        "node_row_a",
		NodeID:    "node_a",
		Name:      "node-a",
		Role:      node.RoleWorker,
		Status:    node.StatusOnline,
		Hostname:  "node-a",
		LocalIP:   "127.0.0.1",
		PublicIP:  "203.0.113.10",
		IsCurrent: true,
		LastSeen:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := endpointRepo.Create(&endpoint.Endpoint{
		ID:        "ep_runsping_a",
		ServiceID: "svc_runsping",
		Type:      "local",
		Address:   "127.0.0.1:9100",
		Enabled:   true,
		NodeID:    "node_a",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := endpointRepo.Create(&endpoint.Endpoint{
		ID:        "ep_admin_a",
		ServiceID: "svc_admin",
		Type:      "local",
		Address:   "127.0.0.1:7380",
		Enabled:   true,
		NodeID:    "node_a",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	return &Handlers{NodeRepo: nodeRepo, EndpointRepo: endpointRepo}
}
