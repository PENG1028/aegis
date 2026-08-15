package handlers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/logs"
	"aegis/internal/service"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// TestEnsurePanelEndpointDerivesPortFromConfig is a regression test: the
// panel endpoint was hardcoded to 127.0.0.1:7380, so changing cfg.Server.Addr
// left the panel route pointing at a dead port. The endpoint must follow the
// configured address.
func TestEnsurePanelEndpointDerivesPortFromConfig(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	h := &Handlers{
		Config:       &config.Config{Server: config.ServerConfig{Addr: "127.0.0.1:9999"}},
		EndpointRepo: endpoint.NewRepository(db),
		EndpointSvc:  endpoint.NewAppService(endpoint.NewRepository(db), logs.NewAppService(logs.NewRepository(db))),
	}
	// The service used by __panel must exist for CreateEndpoint's project
	// validation? CreateEndpoint requires ServiceID of an existing service —
	// create it directly via the service repo first.
	svcRepo := service.NewRepository(db)
	now := time.Now()
	if err := svcRepo.Create(&service.Service{
		ID: "__panel", ProjectID: "__system", Name: "Aegis Panel",
		Kind: "http", Env: "prod", Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	if err := h.ensurePanelEndpoint(context.Background()); err != nil {
		t.Fatalf("ensurePanelEndpoint: %v", err)
	}

	eps, err := h.EndpointRepo.FindByServiceID("__panel")
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) == 0 {
		t.Fatal("panel endpoint was not created")
	}
	if eps[0].Address != "127.0.0.1:9999" {
		t.Fatalf("panel endpoint address = %q, want 127.0.0.1:9999 (derived from cfg.Server.Addr, not hardcoded 7380)", eps[0].Address)
	}
}
