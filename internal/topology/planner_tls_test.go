package topology

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

func TestCollectIntentsBlocksMissingCertificateFiles(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(`
		INSERT INTO services (id, project_id, name, kind, env, status, note, created_at, updated_at)
		VALUES ('svc_test', 'proj_test', 'test', 'http', 'test', 'active', '', ?, ?);
		INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES ('cert_missing_files', '["app.example.com"]', '', ?, ?, 'missing.crt', 'missing.key', 'manual_upload', '', ?, ?);
	`, now.Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339),
		now.Add(24*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	certID := "cert_missing_files"
	routeRepo := route.NewRepository(db)
	if err := routeRepo.Create(&route.Route{
		ID: "rt_test", Domain: "app.example.com", ServiceID: "svc_test",
		Composition: "https_route", TLSEnabled: true, SourceProvider: "caddy",
		TLSBindingMode: route.TLSBindingCertificate, CertID: &certID,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	planner := NewPlanner(nil, Dependencies{
		RouteRepo:   routeRepo,
		ServiceRepo: service.NewRepository(db),
		CertStore:   certstore.NewService(certstore.NewRepository(db), t.TempDir()),
	})
	if _, _, err := planner.collectIntents(); err == nil {
		t.Fatal("missing certificate files did not block planning")
	}

	certPath := filepath.Join(t.TempDir(), "cert.crt")
	keyPath := filepath.Join(t.TempDir(), "cert.key")
	if err := os.WriteFile(certPath, []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE certificates SET cert_path=?, key_path=? WHERE id=?`, certPath, keyPath, certID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE routes SET domain='other.example.net' WHERE id='rt_test'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := planner.collectIntents(); err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Fatalf("domain mismatch was not rejected: %v", err)
	}
	if _, err := db.Exec(`UPDATE routes SET domain='app.example.com' WHERE id='rt_test'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE certificates SET not_after=? WHERE id=?`, now.Add(-time.Minute).Format(time.RFC3339), certID); err != nil {
		t.Fatal(err)
	}
	intents, warnings, err := planner.collectIntents()
	if err != nil || len(intents) != 1 {
		t.Fatalf("expired existing binding should remain renderable: intents=%d err=%v", len(intents), err)
	}
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, "\n"), "not currently valid") {
		t.Fatalf("expired binding warning missing: %v", warnings)
	}
}

func TestInjectControlPlaneIncludesACMEChallenge(t *testing.T) {
	planner := NewPlanner(nil, Dependencies{ControlPort: 7380})
	plan := &TopologyPlan{Plans: map[string]provider.Plan{"caddy": {}}}
	state := provider.ProviderState{
		ID: "caddy", Installed: true, Running: true,
		Capabilities: []provider.Capability{provider.CapRouteHost, provider.CapUpstreamTCP},
	}
	planner.injectControlPlaneRoute(plan, []provider.ProviderState{state})
	found := false
	for _, route := range plan.Plans["caddy"].Routes {
		if route.Match.Path == "/.well-known/acme-challenge" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("control-plane plan omitted ACME challenge route")
	}
}

func TestPlanMatchesCompleteRuntimeModeProviderSet(t *testing.T) {
	mode := provider.RuntimeMode{Providers: []provider.ProviderAtoms{
		{ProviderID: "haproxy"}, {ProviderID: "caddy"},
	}}
	if planMatchesMode(&TopologyPlan{Plans: map[string]provider.Plan{"caddy": {}}}, mode) {
		t.Fatal("partial provider plan matched multi-provider runtime mode")
	}
	if !planMatchesMode(&TopologyPlan{Plans: map[string]provider.Plan{"caddy": {}, "haproxy": {}}}, mode) {
		t.Fatal("complete provider plan did not match runtime mode")
	}
}
