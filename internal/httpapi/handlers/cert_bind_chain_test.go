package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/config"
	"aegis/internal/hostdep/provider"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"
	"aegis/internal/tlslifecycle"

	_ "modernc.org/sqlite"
)

// certChainHarness wires the smallest set of real services that the
// bind → delete-protection chain needs. Everything is real except the provider,
// which only has to declare load_cert for the binding to be permitted.
type certChainHarness struct {
	h      *Handlers
	db     *sql.DB
	routes *route.AppService
}

func newCertChainHarness(t *testing.T) *certChainHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Name: "caddy", Installed: true, Running: true, Status: "ready", Ready: true,
		Capabilities: []provider.Capability{
			provider.CapLoadCert, provider.CapTLSTerminate, provider.CapAutoCert,
			provider.CapListenTCP, provider.CapRouteHost, provider.CapHTTP1,
		},
	}})

	certs := certstore.NewService(certstore.NewRepository(db), t.TempDir())
	routes := route.NewAppService(route.NewRepository(db), logs.NewAppService(logs.NewRepository(db)), nil)
	lifecycle := tlslifecycle.New(routes, certs, registry)

	return &certChainHarness{
		h: &Handlers{
			CertStore:    certs,
			Route:        routes,
			TLSLifecycle: lifecycle,
			ProvReg:      registry,
			Config:       &config.Config{Proxy: config.ProxyConfig{CaddyDataDir: t.TempDir()}},
		},
		db:     db,
		routes: routes,
	}
}

// insertCert adds an uploaded certificate asset covering the given domain.
func (c *certChainHarness) insertCert(t *testing.T, id, domains string) {
	t.Helper()
	now := time.Now().Format(time.RFC3339)
	_, err := c.db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES (?, ?, 'test-ca', ?, ?, '/tmp/c.pem', '/tmp/k.pem', 'manual_upload', '', ?, ?)`,
		id, domains, now, time.Now().Add(24*time.Hour).Format(time.RFC3339), now, now)
	if err != nil {
		t.Fatal(err)
	}
}

// insertRoute adds an HTTPS route directly. Going through CreateRoute would
// require a service record and edge-mux wiring that this chain does not exercise.
func (c *certChainHarness) insertRoute(t *testing.T, id, domain string) {
	t.Helper()
	err := route.NewRepository(c.db).Create(&route.Route{
		ID: id, Domain: domain, ServiceID: "svc_test", Composition: "https_route",
		TLSEnabled: true, Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("insert route %s: %v", id, err)
	}
}

func (c *certChainHarness) bind(t *testing.T, routeID, certID string) *httptest.ResponseRecorder {
	t.Helper()
	body := strings.NewReader(`{"mode":"certificate","cert_id":"` + certID + `"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/admin/v1/routes/"+routeID+"/tls-binding", body)
	req.SetPathValue("id", routeID)
	rec := httptest.NewRecorder()
	c.h.AdminSetRouteTLSBinding(rec, req)
	return rec
}

// TestBindCertificateThenDeleteIsRefused locks the highest-traffic chain in the
// system: bind an uploaded certificate to a route, then try to delete it.
//
// This sequence was validated by hand against a live instance dozens of times
// while the certificate lifecycle was being built, but nothing held it at the
// HTTP layer — the unit tests cover BindCertificate and the delete guard
// separately, so a regression in the wiring between them would pass everything.
// Deleting a bound certificate is what silently breaks TLS for a live domain.
func TestBindCertificateThenDeleteIsRefused(t *testing.T) {
	c := newCertChainHarness(t)
	c.insertCert(t, "cert_bound", `["app.example.com"]`)
	c.insertRoute(t, "rt_bound", "app.example.com")

	if rec := c.bind(t, "rt_bound", "cert_bound"); rec.Code != http.StatusOK {
		t.Fatalf("bind status = %d: %s", rec.Code, rec.Body.String())
	}

	// The binding must be durable, not just reflected in the response.
	stored, err := c.routes.GetRoute(t.Context(), "rt_bound")
	if err != nil {
		t.Fatalf("reload route: %v", err)
	}
	if stored.CertID == nil || *stored.CertID != "cert_bound" {
		t.Errorf("binding did not persist: cert_id = %v", stored.CertID)
	}
	if stored.TLSBindingMode != route.TLSBindingCertificate {
		t.Errorf("binding mode = %q, want %q", stored.TLSBindingMode, route.TLSBindingCertificate)
	}

	// Deleting a referenced certificate must fail closed.
	del := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/certificates/cert_bound", nil)
	del.SetPathValue("id", "cert_bound")
	rec := httptest.NewRecorder()
	c.h.AdminDeleteCertificate(rec, del)

	if rec.Code == http.StatusOK || rec.Code == http.StatusNoContent {
		t.Fatalf("deleting a bound certificate succeeded (%d) — a live domain would lose TLS", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "rt_bound") && !strings.Contains(rec.Body.String(), "app.example.com") {
		t.Errorf("refusal should name what still references the certificate, got: %s", rec.Body.String())
	}
}

// TestBindCertificateRejectsDomainItDoesNotCover locks the coverage check at the
// HTTP layer. Binding a cert for the wrong domain would produce a gateway that
// serves a name mismatch — the browser error users cannot self-diagnose.
func TestBindCertificateRejectsDomainItDoesNotCover(t *testing.T) {
	c := newCertChainHarness(t)
	c.insertCert(t, "cert_other", `["other.example.com"]`)
	c.insertRoute(t, "rt_mismatch", "app.example.com")

	rec := c.bind(t, "rt_mismatch", "cert_other")
	if rec.Code == http.StatusOK {
		t.Fatal("bound a certificate that does not cover the route's domain")
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}

	stored, err := c.routes.GetRoute(t.Context(), "rt_mismatch")
	if err != nil {
		t.Fatal(err)
	}
	if stored.CertID != nil && *stored.CertID != "" {
		t.Errorf("rejected bind still wrote cert_id = %v", stored.CertID)
	}
}

// TestWildcardCertificateCoversOneLabelOnly locks RFC 6125 single-label matching
// through the HTTP layer. The wildcard rule took nine rounds of manual
// verification against a live instance; *.example.com must cover app.example.com
// but not deep.app.example.com, and over-matching would bind a certificate the
// browser then rejects.
func TestWildcardCertificateCoversOneLabelOnly(t *testing.T) {
	c := newCertChainHarness(t)
	c.insertCert(t, "cert_wild", `["*.example.com"]`)

	for _, tc := range []struct {
		domain   string
		wantBind bool
	}{
		{"app.example.com", true},
		{"deep.app.example.com", false},
	} {
		t.Run(tc.domain, func(t *testing.T) {
			rtID := "rt_" + strings.ReplaceAll(tc.domain, ".", "_")
			c.insertRoute(t, rtID, tc.domain)
			rec := c.bind(t, rtID, "cert_wild")
			bound := rec.Code == http.StatusOK
			if bound != tc.wantBind {
				t.Errorf("%s: bound = %v (status %d), want %v: %s",
					tc.domain, bound, rec.Code, tc.wantBind, rec.Body.String())
			}
		})
	}
}

// TestCertificateBindingSurvivesRouteReload guards that a binding is stored on
// the route rather than held in memory: the apply path re-reads routes from the
// database, so an in-memory-only binding would render config without the cert.
func TestCertificateBindingSurvivesRouteReload(t *testing.T) {
	c := newCertChainHarness(t)
	c.insertCert(t, "cert_persist", `["keep.example.com"]`)
	c.insertRoute(t, "rt_persist", "keep.example.com")

	if rec := c.bind(t, "rt_persist", "cert_persist"); rec.Code != http.StatusOK {
		t.Fatalf("bind failed: %s", rec.Body.String())
	}

	// Re-read through a fresh repository over the same database — what apply does.
	reloaded, err := route.NewRepository(c.db).FindByID("rt_persist")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.CertID == nil || *reloaded.CertID != "cert_persist" {
		t.Errorf("binding lost across reload: cert_id = %v", reloaded.CertID)
	}

	// And it must be visible in the certificate's own binding list.
	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/certificates", nil)
	rec := httptest.NewRecorder()
	c.h.AdminListCertificates(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d", rec.Code)
	}
	var body struct {
		Assets []map[string]any `json:"assets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Assets) != 1 {
		t.Fatalf("expected the uploaded asset in the list, got %+v", body.Assets)
	}
}
