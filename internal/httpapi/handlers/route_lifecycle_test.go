package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"
	"aegis/internal/tlslifecycle"

	_ "modernc.org/sqlite"
)

func TestRouteDeleteRetainsAssetByDefaultAndCleansItOnlyWhenRequested(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	logSvc := logs.NewAppService(logs.NewRepository(db))
	routes := route.NewAppService(route.NewRepository(db), logSvc, nil)
	certs := certstore.NewService(certstore.NewRepository(db), t.TempDir())
	lifecycle := tlslifecycle.New(routes, certs, nil)
	h := &Handlers{Route: routes, CertStore: certs, TLSLifecycle: lifecycle}

	create := func(routeID, certID, domain string) {
		t.Helper()
		now := time.Now()
		_, err := db.Exec(`INSERT INTO certificates
			(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
			VALUES (?, ?, 'test', ?, ?, '', '', 'manual_upload', '', ?, ?)`,
			certID, `["`+domain+`"]`, now.Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			t.Fatal(err)
		}
		if err := routes.CreateRouteDirect(&route.Route{
			ID: routeID, Domain: domain, ServiceID: "svc_test", Composition: "https_route",
			TLSEnabled: true, CertID: &certID, Status: "active", CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			t.Fatal(err)
		}
	}

	create("rt_keep", "cert_keep", "keep.example.com")
	request := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/routes/rt_keep", nil)
	request.SetPathValue("id", "rt_keep")
	response := httptest.NewRecorder()
	h.AdminDeleteRoute(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("default delete status = %d: %s", response.Code, response.Body.String())
	}
	if cert, err := certs.Get("cert_keep"); err != nil || cert == nil {
		t.Fatalf("default route deletion removed asset: cert=%v err=%v", cert, err)
	}

	create("rt_clean", "cert_clean", "clean.example.com")
	request = httptest.NewRequest(http.MethodDelete, "/api/admin/v1/routes/rt_clean?delete_unused_certificate=true", nil)
	request.SetPathValue("id", "rt_clean")
	response = httptest.NewRecorder()
	h.AdminDeleteRoute(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("cleanup delete status = %d: %s", response.Code, response.Body.String())
	}
	if cert, err := certs.Get("cert_clean"); err != nil || cert != nil {
		t.Fatalf("explicit unused asset cleanup failed: cert=%v err=%v", cert, err)
	}
}
