package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/config"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

type testTLSObserver struct{}

func (testTLSObserver) ExecutorID() string { return "executor-test" }
func (testTLSObserver) Observe(context.Context) ([]certstore.DiscoveredCert, error) {
	return []certstore.DiscoveredCert{{
		Domains: `["auto.example.com"]`, Source: certstore.SourceGatewayAuto,
		Issuer: "test-auto", NotBefore: time.Now().Format(time.RFC3339),
		NotAfter: time.Now().Add(time.Hour).Format(time.RFC3339), ExecutorID: "executor-test",
	}}, nil
}

func TestAdminDeleteCertificateFailsClosedWithoutLifecycleService(t *testing.T) {
	h := &Handlers{CertStore: certstore.NewService(nil, t.TempDir())}
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/v1/certificates/cert_test", nil)
	req.SetPathValue("id", "cert_test")
	response := httptest.NewRecorder()

	h.AdminDeleteCertificate(response, req)

	if response.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 when TLS lifecycle wiring is missing, got %d", response.Code)
	}
}

func TestCertificateListSeparatesAssetsFromAutomaticTLS(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Format(time.RFC3339)
	_, err = db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES ('cert_asset', '["app.example.com"]', 'test', ?, ?, '', '', 'manual_upload', '', ?, ?)`,
		now, time.Now().Add(time.Hour).Format(time.RFC3339), now, now)
	if err != nil {
		t.Fatal(err)
	}
	h := &Handlers{
		CertStore:    certstore.NewService(certstore.NewRepository(db), t.TempDir()),
		Config:       &config.Config{Proxy: config.ProxyConfig{CaddyDataDir: t.TempDir()}},
		TLSObservers: []certstore.AutomaticTLSObserver{testTLSObserver{}},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/admin/v1/certificates", nil)
	response := httptest.NewRecorder()
	h.AdminListCertificates(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Certificates []map[string]interface{} `json:"certificates"`
		Assets       []map[string]interface{} `json:"assets"`
		AutomaticTLS []map[string]interface{} `json:"automatic_tls"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Certificates) != 1 || len(body.Assets) != 1 || len(body.AutomaticTLS) != 1 {
		t.Fatalf("certificate classes were not separated: %+v", body)
	}
	if body.Assets[0]["record_type"] != "asset" {
		t.Fatalf("unexpected asset record: %+v", body.Assets[0])
	}
	if body.AutomaticTLS[0]["record_type"] != "provider_observation" || body.AutomaticTLS[0]["managed_by"] != "executor-test" {
		t.Fatalf("unexpected automatic TLS observation: %+v", body.AutomaticTLS[0])
	}
}
