package exposure

import (
	"context"
	"database/sql"
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

func testExposure(id string) *Exposure {
	now := time.Now()
	return &Exposure{
		ID:             id,
		Type:           TypeTCP,
		Mode:           ModePrivate,
		Host:           "127.0.0.1",
		Port:           18080,
		TargetHost:     "10.0.0.5",
		TargetPort:     3306,
		OwnerRef:       "owner-x",
		Status:         StatusPending,
		Provider:       "caddy",     // ← must survive the round trip
		ListenerID:     "lis_123",   // ← must survive the round trip
		AllowPublicTCP: true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// TestProviderAndListenerIDPersist is a regression test: the provider and
// listener_id columns exist in the schema and the model carries them, but
// Create/Update/scan never touched them — provider selection results were
// silently lost, so ActivateExposure's provider checks saw stale data.
func TestProviderAndListenerIDPersist(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)

	e := testExposure("exp_persist")
	if err := repo.Create(e); err != nil {
		t.Fatal(err)
	}

	got, err := repo.FindByID(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("exposure not found after create")
	}
	if got.Provider != "caddy" {
		t.Errorf("provider = %q after round trip, want %q (column not persisted)", got.Provider, "caddy")
	}
	if got.ListenerID != "lis_123" {
		t.Errorf("listener_id = %q after round trip, want %q (column not persisted)", got.ListenerID, "lis_123")
	}

	// Update path must preserve them too.
	e.Status = StatusActive
	e.Provider = "haproxy"
	e.ListenerID = "lis_456"
	if err := repo.Update(e); err != nil {
		t.Fatal(err)
	}
	got2, err := repo.FindByID(e.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got2.Provider != "haproxy" || got2.ListenerID != "lis_456" {
		t.Errorf("after update: provider=%q listener_id=%q, want haproxy/lis_456", got2.Provider, got2.ListenerID)
	}
}

// TestUpdateExposureRejectsStatusWrite is a regression test: PATCH could
// write status=active directly, bypassing ActivateExposure's provider
// availability checks and proxy startup — a record could claim "active"
// while no proxy runs (or worse, drift from the real listener state).
func TestUpdateExposureRejectsStatusWrite(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	svc := NewAppService(repo, nil, nil, nil)

	e := testExposure("exp_status")
	if err := repo.Create(e); err != nil {
		t.Fatal(err)
	}

	status := "active"
	_, err := svc.UpdateExposure(context.Background(), e.ID, UpdateExposureInput{Status: &status}, "")
	if err == nil {
		t.Fatal("UpdateExposure must reject direct status writes — activation goes through ActivateExposure")
	}
}

// TestUpdateExposureRejectsInvalidPort is a regression test: port 0/negative
// could be written via PATCH, so a later activate bound an ephemeral port
// while the DB record kept Port=0 (permanent record/listener drift).
func TestUpdateExposureRejectsInvalidPort(t *testing.T) {
	db := testDB(t)
	repo := NewRepository(db)
	svc := NewAppService(repo, nil, nil, nil)

	e := testExposure("exp_port")
	if err := repo.Create(e); err != nil {
		t.Fatal(err)
	}

	port := 0
	_, err := svc.UpdateExposure(context.Background(), e.ID, UpdateExposureInput{Port: &port}, "")
	if err == nil {
		t.Fatal("UpdateExposure must reject port 0")
	}

	port = 70000
	if _, err := svc.UpdateExposure(context.Background(), e.ID, UpdateExposureInput{Port: &port}, ""); err == nil {
		t.Fatal("UpdateExposure must reject port > 65535")
	}
}
