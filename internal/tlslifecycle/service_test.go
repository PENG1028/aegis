package tlslifecycle

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/provider"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

func TestReferencedCertificateRequiresUnbindAndRouteDeleteRetainsAsset(t *testing.T) {
	db, routes, certs, lifecycle := setupLifecycleTest(t)
	insertCertificate(t, db, "cert_manual", `["app.example.com"]`, certstore.SourceManualUpload)
	certID := "cert_manual"
	rt := newTestRoute("rt_app", "app.example.com", &certID)
	if err := routes.CreateRouteDirect(rt); err != nil {
		t.Fatal(err)
	}

	preview, err := lifecycle.PreviewDeleteCertificate(context.Background(), certID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Allowed || preview.ReasonCode != ReasonHasReferences || len(preview.References) != 1 {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	if err := routes.DeleteRoute(context.Background(), rt.ID); err != nil {
		t.Fatal(err)
	}
	if cert, err := certs.Get(certID); err != nil || cert == nil {
		t.Fatalf("route deletion removed certificate asset: cert=%v err=%v", cert, err)
	}
	preview, err = lifecycle.DeleteCertificate(context.Background(), certID)
	if err != nil || !preview.Allowed {
		t.Fatalf("unreferenced certificate should delete: preview=%+v err=%v", preview, err)
	}
}

func TestBindCertificateValidatesWildcardCoverage(t *testing.T) {
	db, routes, _, lifecycle := setupLifecycleTest(t)
	insertCertificate(t, db, "cert_wild", `["*.example.com"]`, certstore.SourceManualUpload)
	if err := routes.CreateRouteDirect(newTestRoute("rt_api", "api.example.com", nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.BindCertificate(context.Background(), "rt_api", "cert_wild"); err != nil {
		t.Fatalf("bind wildcard certificate: %v", err)
	}
	bound, _ := routes.GetRoute(context.Background(), "rt_api")
	if bound.TLSBindingMode != route.TLSBindingCertificate || bound.CertID == nil || *bound.CertID != "cert_wild" {
		t.Fatalf("unexpected binding: %+v", bound)
	}

	if err := routes.CreateRouteDirect(newTestRoute("rt_deep", "deep.api.example.com", nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := lifecycle.BindCertificate(context.Background(), "rt_deep", "cert_wild"); err == nil {
		t.Fatal("multi-label wildcard binding should fail")
	}
}

func TestWildcardBindingPreviewAndBatchCommit(t *testing.T) {
	db, routes, _, lifecycle := setupLifecycleTest(t)
	insertCertificate(t, db, "cert_wild", `["*.example.com"]`, certstore.SourceManualUpload)
	for _, rt := range []*route.Route{
		newTestRoute("rt_api", "api.example.com", nil),
		newTestRoute("rt_admin", "admin.example.com", nil),
		newTestRoute("rt_deep", "deep.api.example.com", nil),
		newTestRoute("rt_apex", "example.com", nil),
	} {
		if err := routes.CreateRouteDirect(rt); err != nil {
			t.Fatal(err)
		}
	}
	preview, err := lifecycle.PreviewCertificateBindings(context.Background(), "cert_wild")
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 2 {
		t.Fatalf("wildcard candidates = %+v, want two single-label subdomains", preview.Candidates)
	}
	for _, candidate := range preview.Candidates {
		if candidate.AlreadyBound || !candidate.ReplacesAutomaticTLS || candidate.CurrentBindingMode != route.TLSBindingProviderAuto {
			t.Fatalf("automatic TLS replacement was not explicit in preview: %+v", candidate)
		}
	}
	if _, err := lifecycle.BindCertificateToRoutes(context.Background(), "cert_wild", []string{"rt_api", "rt_deep"}); err == nil {
		t.Fatal("batch containing an ineligible route should fail")
	}
	api, _ := routes.GetRoute(context.Background(), "rt_api")
	if api.CertID != nil && *api.CertID != "" {
		t.Fatal("failed batch partially updated an eligible route")
	}
	if _, err := lifecycle.BindCertificateToRoutes(context.Background(), "cert_wild", []string{"rt_api", "rt_admin"}); err != nil {
		t.Fatal(err)
	}
	for _, routeID := range []string{"rt_api", "rt_admin"} {
		bound, _ := routes.GetRoute(context.Background(), routeID)
		if bound.CertID == nil || *bound.CertID != "cert_wild" {
			t.Fatalf("route %s was not bound: %+v", routeID, bound)
		}
	}
	preview, err = lifecycle.PreviewCertificateBindings(context.Background(), "cert_wild")
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range preview.Candidates {
		if !candidate.AlreadyBound || !candidate.Selected || candidate.ReplacesAutomaticTLS {
			t.Fatalf("completed binding state was not reflected in preview: %+v", candidate)
		}
	}
}

func TestRouteDeletePreviewKeepsAssetByDefaultAndOffersLastReferenceCleanup(t *testing.T) {
	db, routes, _, lifecycle := setupLifecycleTest(t)
	insertCertificate(t, db, "cert_manual", `["app.example.com"]`, certstore.SourceManualUpload)
	certID := "cert_manual"
	if err := routes.CreateRouteDirect(newTestRoute("rt_app", "app.example.com", &certID)); err != nil {
		t.Fatal(err)
	}
	preview, err := lifecycle.PreviewDeleteRoute(context.Background(), "rt_app")
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Allowed || !preview.DeleteUnusedCertificateAllowed || preview.CertificateID != certID {
		t.Fatalf("unexpected route delete preview: %+v", preview)
	}
	if len(preview.Effects) < 2 || preview.Effects[1] != "retain independent certificate assets by default" {
		t.Fatalf("default retention effect missing: %+v", preview.Effects)
	}
}

func TestUseProviderAutoDoesNotReplaceExistingExecutor(t *testing.T) {
	_, routes, certs, _ := setupLifecycleTest(t)
	registry := provider.NewRegistry()
	registry.Register(&lifecycleTestProvider{state: provider.ProviderState{
		ID: "caddy", Installed: true, Running: true,
		Capabilities: []provider.Capability{provider.CapAutoCert},
	}})
	lifecycle := New(routes, certs, registry)
	rt := newTestRoute("rt_app", "app.example.com", nil)
	rt.SourceProvider = "caddy"
	rt.TLSBindingMode = route.TLSBindingProviderAuto
	rt.TLSProvider = "unavailable-executor"
	if err := routes.CreateRouteDirect(rt); err != nil {
		t.Fatal(err)
	}

	if _, err := lifecycle.UseProviderAuto(context.Background(), rt.ID, ""); err == nil {
		t.Fatal("ordinary binding silently replaced an unavailable automatic TLS executor")
	}
	if _, err := lifecycle.UseProviderAuto(context.Background(), rt.ID, "caddy"); err == nil {
		t.Fatal("ordinary binding accepted a different executor without explicit migration")
	}
	stored, err := routes.GetRoute(context.Background(), rt.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.TLSProvider != "unavailable-executor" {
		t.Fatalf("executor changed after rejected operation: %+v", stored)
	}
}

type lifecycleTestProvider struct{ state provider.ProviderState }

func (p *lifecycleTestProvider) State() provider.ProviderState { return p.state }
func (p *lifecycleTestProvider) Diagnose() provider.ProviderDiagnostic {
	return provider.ProviderDiagnostic{}
}
func (p *lifecycleTestProvider) Render(provider.Plan) ([]provider.ConfigFile, error) { return nil, nil }
func (p *lifecycleTestProvider) Apply([]provider.ConfigFile) error                   { return nil }

func setupLifecycleTest(t *testing.T) (*sql.DB, *route.AppService, *certstore.Service, *Service) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	logSvc := logs.NewAppService(logs.NewRepository(db))
	routeSvc := route.NewAppService(route.NewRepository(db), logSvc, nil)
	certSvc := certstore.NewService(certstore.NewRepository(db), t.TempDir())
	return db, routeSvc, certSvc, New(routeSvc, certSvc, nil)
}

func insertCertificate(t *testing.T, db *sql.DB, id, domains, source string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES (?, ?, 'test', ?, ?, '', '', ?, '', ?, ?)`,
		id, domains, now, time.Now().Add(24*time.Hour).Format(time.RFC3339), source, now, now)
	if err != nil {
		t.Fatal(err)
	}
}

func newTestRoute(id, domain string, certID *string) *route.Route {
	now := time.Now()
	return &route.Route{
		ID: id, Domain: domain, ServiceID: "svc_test", Composition: "https_route",
		TLSEnabled: true, CertID: certID, Status: "active", CreatedAt: now, UpdatedAt: now,
	}
}
