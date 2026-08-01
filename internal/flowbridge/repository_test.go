package flowbridge

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func testInstance(id string) *Instance {
	now := time.Now()
	return &Instance{
		ID:               id,
		Name:             "edge-" + id,
		MachineIP:        "10.0.0.5",
		DataPlanePort:    8080,
		ControlAddress:   "10.0.0.5:9090",
		Enabled:          true,
		LastHealthStatus: HealthUnknown,
		OwnerType:        "admin",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestInstanceCRUD(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	if err := repo.Create(testInstance("fb_1")); err != nil {
		t.Fatal(err)
	}
	inst, err := repo.FindByID("fb_1")
	if err != nil {
		t.Fatal(err)
	}
	if inst == nil || inst.Name != "edge-fb_1" || !inst.Enabled {
		t.Fatalf("unexpected instance: %+v", inst)
	}

	all, err := repo.FindAll()
	if err != nil || len(all) != 1 {
		t.Fatalf("FindAll: %v len=%d", err, len(all))
	}

	inst.Enabled = false
	inst.LastHealthStatus = HealthUnhealthy
	inst.UpdatedAt = time.Now()
	if err := repo.Update(inst); err != nil {
		t.Fatal(err)
	}
	got, _ := repo.FindByID("fb_1")
	if got.Enabled || got.LastHealthStatus != HealthUnhealthy {
		t.Fatalf("update not persisted: %+v", got)
	}

	if err := repo.Delete("fb_1"); err != nil {
		t.Fatal(err)
	}
	if inst, _ := repo.FindByID("fb_1"); inst != nil {
		t.Fatal("instance still exists after delete")
	}
}

func TestInstanceDeleteBlockedWhenReferenced(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	if err := repo.Create(testInstance("fb_1")); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)
	if _, err := db.Exec(
		`INSERT INTO routes (id, domain, path_prefix, strip_prefix, service_id, tls_enabled, composition,
			source_provider, source_capabilities, tls_binding_mode, tls_provider, status,
			maintenance_enabled, maintenance_message, space_id, owner_type, owner_id,
			created_by_token_id, gateway_link_id, cert_id, flowbridge_id, created_at, updated_at)
		 VALUES ('rt_1', 'a.test', '', 0, 'svc_1', 1, 'https_route', 'caddy', '', 'provider_auto', 'caddy', 'active',
			0, '', '', 'admin', '', '', '', '', 'fb_1', ?, ?)`,
		now, now); err != nil {
		t.Fatal(err)
	}

	err := repo.Delete("fb_1")
	if err == nil || !strings.Contains(err.Error(), "referenced") {
		t.Fatalf("expected reference-blocked delete, got: %v", err)
	}
	count, err := repo.RoutesReferencing("fb_1")
	if err != nil || count != 1 {
		t.Fatalf("RoutesReferencing: %v count=%d", err, count)
	}
}

func TestRouteFlowBridgeReferenceGuard(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(
		`INSERT INTO routes (id, domain, path_prefix, strip_prefix, service_id, tls_enabled, composition,
			source_provider, source_capabilities, tls_binding_mode, tls_provider, status,
			maintenance_enabled, maintenance_message, space_id, owner_type, owner_id,
			created_by_token_id, gateway_link_id, cert_id, flowbridge_id, created_at, updated_at)
		 VALUES ('rt_bad', 'b.test', '', 0, 'svc_1', 1, 'https_route', 'caddy', '', 'provider_auto', 'caddy', 'active',
			0, '', '', 'admin', '', '', '', '', 'fb_missing', ?, ?)`,
		now, now)
	if err == nil || !strings.Contains(err.Error(), "FLOWBRIDGE_NOT_FOUND") {
		t.Fatalf("expected FLOWBRIDGE_NOT_FOUND guard, got: %v", err)
	}
	if inst, _ := repo.FindByID("fb_missing"); inst != nil {
		t.Fatal("guard rejected insert but instance lookup succeeded")
	}
}
