package gateway

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupGWTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS gateways (
			gateway_id TEXT PRIMARY KEY, node_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT 'local', provider TEXT NOT NULL DEFAULT 'aegis',
			bind_addr TEXT NOT NULL DEFAULT '0.0.0.0', host TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 80, scheme TEXT NOT NULL DEFAULT 'http',
			public_accessible INTEGER NOT NULL DEFAULT 0, private_accessible INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1, priority INTEGER NOT NULL DEFAULT 100,
			status TEXT NOT NULL DEFAULT 'unknown', last_verified_at TEXT DEFAULT '',
			last_error TEXT DEFAULT '', created_at TEXT NOT NULL, updated_at TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("create gateways table: %v", err)
	}
	return db
}

func TestCreateGateway(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)
	svc := NewInventoryService(repo)

	gw, err := svc.CreateGateway(CreateGatewayInput{
		NodeID: "nd_test", Name: "public-gw", Type: GWTypePublic,
		Provider: GWProviderCaddy, Host: "43.x.x.x", Port: 80, Scheme: GWSchemeHTTP,
		PublicAccessible: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if gw.GatewayID == "" {
		t.Error("expected non-empty gateway_id")
	}
	if gw.Status != GWStatusUnknown {
		t.Errorf("expected status unknown, got %s", gw.Status)
	}
}

func TestListGatewaysByNode(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)
	svc := NewInventoryService(repo)

	svc.CreateGateway(CreateGatewayInput{NodeID: "nd_a", Name: "gw1", Port: 80})
	svc.CreateGateway(CreateGatewayInput{NodeID: "nd_a", Name: "gw2", Port: 443})
	svc.CreateGateway(CreateGatewayInput{NodeID: "nd_b", Name: "gw3", Port: 8080})

	list, err := repo.FindByNodeID("nd_a")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 gateways for nd_a, got %d", len(list))
	}
}

func TestUpdateGatewayStatus(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)
	svc := NewInventoryService(repo)

	gw, _ := svc.CreateGateway(CreateGatewayInput{NodeID: "nd_test", Name: "gw1", Port: 80})

	err := repo.SetStatus(gw.GatewayID, GWStatusOnline, "")
	if err != nil {
		t.Fatalf("set status: %v", err)
	}

	updated, _ := repo.FindByID(gw.GatewayID)
	if updated == nil || updated.Status != GWStatusOnline {
		t.Errorf("expected status online, got %v", updated)
	}
}

func TestRejectGatewayMissingNode(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)
	svc := NewInventoryService(repo)

	_, err := svc.CreateGateway(CreateGatewayInput{NodeID: "", Name: "no-node"})
	if err == nil {
		t.Error("expected error for empty node_id")
	}
}

func TestUpsertByNodeAndName(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)

	gw1 := &GatewayInventory{
		GatewayID: "", NodeID: "nd_test", Name: "gw-upsert",
		Type: GWTypePublic, Host: "1.2.3.4", Port: 80,
	}
	repo.UpsertByNodeAndName(gw1)

	// Upsert again with different host
	gw2 := &GatewayInventory{
		GatewayID: "", NodeID: "nd_test", Name: "gw-upsert",
		Type: GWTypePublic, Host: "5.6.7.8", Port: 80,
	}
	repo.UpsertByNodeAndName(gw2)

	list, _ := repo.FindByNodeID("nd_test")
	if len(list) != 1 {
		t.Errorf("expected 1 gateway after upsert, got %d", len(list))
	}
	if list[0].Host != "5.6.7.8" {
		t.Errorf("expected updated host 5.6.7.8, got %s", list[0].Host)
	}
}

func TestListAllGateways(t *testing.T) {
	db := setupGWTestDB(t)
	defer db.Close()
	repo := NewInventoryRepository(db)
	svc := NewInventoryService(repo)

	svc.CreateGateway(CreateGatewayInput{NodeID: "nd_a", Name: "gw-a"})
	svc.CreateGateway(CreateGatewayInput{NodeID: "nd_b", Name: "gw-b"})

	all, err := repo.FindAll()
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 gateways, got %d", len(all))
	}
}
