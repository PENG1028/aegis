package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/hostdep/provider"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"
	"aegis/internal/tlslifecycle"

	_ "modernc.org/sqlite"
)

func newLifecycleContractHandlers(t *testing.T) *Handlers {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES ('cert_wild', '["*.example.com"]', 'test', ?, ?, '', '', 'manual_upload', '', ?, ?)`,
		now.Add(-time.Hour).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), now.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	routes := route.NewAppService(route.NewRepository(db), logs.NewAppService(logs.NewRepository(db)), nil)
	certs := certstore.NewService(certstore.NewRepository(db), t.TempDir())
	registry := provider.NewRegistry()
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "caddy", Name: "TLS executor", Status: "ready", Installed: true, Running: true,
		Capabilities: []provider.Capability{provider.CapRouteHost, provider.CapAutoCert, provider.CapLoadCert},
	}})
	registry.Register(&modeTLSProvider{state: provider.ProviderState{
		ID: "haproxy", Name: "Edge executor", Status: "ready", Installed: true, Running: true,
		Capabilities: []provider.Capability{provider.CapRawTCP, provider.CapTLSPassthrough, provider.CapAutoCert},
	}})
	certID := "cert_wild"
	for _, rt := range []route.Route{
		{ID: "rt_asset", Domain: "asset.example.com", ServiceID: "svc", Composition: "https_route", SourceProvider: "caddy", TLSBindingMode: route.TLSBindingCertificate, CertID: &certID, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "rt_auto", Domain: "auto.example.com", ServiceID: "svc", Composition: "https_route", SourceProvider: "caddy", TLSBindingMode: route.TLSBindingProviderAuto, TLSProvider: "caddy", Status: "active", CreatedAt: now, UpdatedAt: now},
		{ID: "rt_move", Domain: "move.example.com", ServiceID: "svc", Composition: "https_route", SourceProvider: "haproxy", TLSBindingMode: route.TLSBindingProviderAuto, TLSProvider: "haproxy", Status: "active", CreatedAt: now, UpdatedAt: now},
	} {
		if err := routes.CreateRouteDirect(&rt); err != nil {
			t.Fatal(err)
		}
	}
	return &Handlers{Route: routes, CertStore: certs, ProvReg: registry, TLSLifecycle: tlslifecycle.New(routes, certs, registry)}
}

func decodeContractBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
	return body
}

func TestLifecycleHTTPContractMatrix(t *testing.T) {
	h := newLifecycleContractHandlers(t)
	tests := []struct {
		name   string
		path   string
		id     string
		handle func(http.ResponseWriter, *http.Request)
		assert func(*testing.T, map[string]interface{})
	}{
		{
			name: "certificate delete preview", path: "/api/admin/v1/certificates/cert_wild/delete-preview", id: "cert_wild", handle: h.AdminPreviewDeleteCertificate,
			assert: func(t *testing.T, body map[string]interface{}) {
				if body["reason_code"] != tlslifecycle.ReasonHasReferences || body["managed_by"] != "user" {
					t.Fatalf("delete reason contract = %+v", body)
				}
				refs, _ := body["references"].([]interface{})
				if len(refs) != 1 || refs[0].(map[string]interface{})["id"] != "rt_asset" {
					t.Fatalf("delete references contract = %+v", body["references"])
				}
				if effects, _ := body["effects"].([]interface{}); len(effects) != 0 {
					t.Fatalf("blocked delete must not advertise effects: %+v", effects)
				}
			},
		},
		{
			name: "route delete preview", path: "/api/admin/v1/routes/rt_asset/delete-preview", id: "rt_asset", handle: h.AdminPreviewDeleteRoute,
			assert: func(t *testing.T, body map[string]interface{}) {
				if body["tls_binding_mode"] != route.TLSBindingCertificate || body["certificate_id"] != "cert_wild" || body["delete_unused_certificate_allowed"] != true {
					t.Fatalf("route delete contract = %+v", body)
				}
				if effects, _ := body["effects"].([]interface{}); len(effects) != 2 {
					t.Fatalf("route delete effects = %+v", effects)
				}
			},
		},
		{
			name: "certificate binding preview", path: "/api/admin/v1/certificates/cert_wild/bindings", id: "cert_wild", handle: h.AdminPreviewCertificateBindings,
			assert: func(t *testing.T, body map[string]interface{}) {
				candidates := body["candidates"].([]interface{})
				seenReplacement := false
				for _, raw := range candidates {
					candidate := raw.(map[string]interface{})
					if candidate["route_id"] == "rt_auto" && candidate["current_binding_mode"] == route.TLSBindingProviderAuto && candidate["replaces_automatic_tls"] == true {
						seenReplacement = true
					}
				}
				if !seenReplacement {
					t.Fatalf("automatic TLS replacement missing: %+v", candidates)
				}
			},
		},
		{
			name: "route capability status", path: "/api/admin/v1/routes/rt_auto/capability-status", id: "rt_auto", handle: h.AdminRouteCapabilityStatus,
			assert: func(t *testing.T, body map[string]interface{}) {
				if body["provider"] != "caddy" {
					t.Fatalf("route executor contract = %+v", body)
				}
				semantics := body["capability_semantics"].(map[string]interface{})
				autoTLS := semantics[string(provider.CapAutoCert)].(map[string]interface{})
				if autoTLS["state_class"] != string(provider.StateProviderManaged) || autoTLS["migration"] != string(provider.MigrationRecreate) {
					t.Fatalf("route capability semantics = %+v", autoTLS)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.SetPathValue("id", tc.id)
			rec := httptest.NewRecorder()
			tc.handle(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			tc.assert(t, decodeContractBody(t, rec))
		})
	}
}

func TestProviderAndModeHTTPContracts(t *testing.T) {
	h := newLifecycleContractHandlers(t)

	t.Run("provider capability statuses", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.ListProviders(rec, httptest.NewRequest(http.MethodGet, "/api/admin/v1/providers", nil))
		body := decodeContractBody(t, rec)
		providers := body["providers"].([]interface{})
		if len(providers) != 2 {
			t.Fatalf("providers = %+v", providers)
		}
		statuses := providers[0].(map[string]interface{})["capability_statuses"].([]interface{})
		if len(statuses) == 0 {
			t.Fatal("capability_statuses missing")
		}
		first := statuses[0].(map[string]interface{})
		if first["availability"] != "ready" || first["semantics"] == nil {
			t.Fatalf("capability status = %+v", first)
		}
	})

	t.Run("mode preview executor migration", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/mode/preview?target=legacy", nil)
		rec := httptest.NewRecorder()
		h.ModePreview(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := decodeContractBody(t, rec)
		impacts := body["rpcb_conflicts"].([]interface{})
		byID := make(map[string]map[string]interface{}, len(impacts))
		for _, raw := range impacts {
			impact := raw.(map[string]interface{})
			byID[impact["route_id"].(string)] = impact
		}
		if got := byID["rt_auto"]; got["current_executor"] != "caddy" || got["target_executor"] != "caddy" || got["migration"] != string(provider.MigrationRerender) {
			t.Fatalf("same executor impact = %+v", got)
		}
		if got := byID["rt_move"]; got["current_executor"] != "haproxy" || got["target_executor"] != "caddy" || got["migration"] != string(provider.MigrationRecreate) {
			t.Fatalf("moved executor impact = %+v", got)
		}
		if got := byID["rt_asset"]; got["state_class"] != string(provider.StatePortableAsset) || got["migration"] != string(provider.MigrationReloadAsset) {
			t.Fatalf("portable asset impact = %+v", got)
		}
	})
}
