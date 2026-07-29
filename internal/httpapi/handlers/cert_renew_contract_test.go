package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// renewHarness wires AdminRenewCert against a real certificate store.
//
// ACMEClient is a concrete *acme.Client on Handlers, so no fake renewer can be
// injected here; the paths this exercises are the ones reachable without one.
// That is deliberate rather than incidental — see the note on the 202 case below.
type renewHarness struct {
	h  *Handlers
	db *sql.DB
}

func newRenewHarness(t *testing.T) *renewHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return &renewHarness{
		h:  &Handlers{CertStore: certstore.NewService(certstore.NewRepository(db), t.TempDir())},
		db: db,
	}
}

func (rh *renewHarness) insertCert(t *testing.T, id, source string) {
	t.Helper()
	now := time.Now().UTC()
	_, err := rh.db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES (?, '["app.example.com"]', 'test', ?, ?, '', '', ?, '', ?, ?)`,
		id, now.Add(-time.Hour).Format(time.RFC3339), now.Add(720*time.Hour).Format(time.RFC3339),
		source, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("insert cert %s: %v", id, err)
	}
}

func (rh *renewHarness) renew(t *testing.T, id string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/certificates/"+id+"/renew", nil)
	req.SetPathValue("id", id)
	rec := httptest.NewRecorder()
	rh.h.AdminRenewCert(rec, req)

	body := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
	}
	return rec, body
}

// TestRenewDoesNotClaimSuccessForCertificatesItCannotRenew is the property the
// renew scripts were checking by reading the message field.
//
// A manually uploaded certificate cannot be renewed, and a gateway-managed one is
// renewed by the provider rather than here. Both return 200, because the request
// was understood — but neither renewed anything. Any caller that treats HTTP 200
// as "the certificate is fresh" would schedule nothing and let the asset expire.
// So the body must carry renewed=false, and the message must say why.
func TestRenewDoesNotClaimSuccessForCertificatesItCannotRenew(t *testing.T) {
	cases := map[string]string{
		"manual upload":   certstore.SourceManualUpload,
		"gateway managed": certstore.SourceGatewayAuto,
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			rh := newRenewHarness(t)
			rh.insertCert(t, "cert_x", source)

			rec, body := rh.renew(t, "cert_x")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			if renewed, _ := body["renewed"].(bool); renewed {
				t.Errorf("%s reported renewed=true — nothing was renewed, and a caller "+
					"trusting this would let the certificate expire", name)
			}
			if msg, _ := body["message"].(string); msg == "" {
				t.Errorf("%s: message is empty — a 200 with renewed=false and no reason "+
					"is indistinguishable from success", name)
			}
		})
	}
}

// TestRenewLocalACMEWithoutClientFailsLoudly pins the opposite direction. A
// local-ACME certificate with no ACME client configured must not come back 200
// with renewed=false, because that shape is what "cannot be renewed by design"
// looks like. A missing client is a fixable misconfiguration and has to read as an
// error, not as a permanent property of the certificate.
func TestRenewLocalACMEWithoutClientFailsLoudly(t *testing.T) {
	rh := newRenewHarness(t)
	rh.insertCert(t, "cert_acme", certstore.SourceLocalACME)

	rec, _ := rh.renew(t, "cert_acme")
	if rec.Code == http.StatusOK {
		t.Fatalf("a local-ACME renewal with no ACME client returned 200 — a misconfiguration "+
			"would read as 'this certificate cannot be renewed': %s", rec.Body.String())
	}
	if rec.Code < 400 {
		t.Errorf("status = %d, want a 4xx/5xx", rec.Code)
	}
}

// TestRenewRejectsUnknownCertificate guards that a missing ID is a 404 rather than
// a 200 with an empty result, which a batch renewal script would count as done.
func TestRenewRejectsUnknownCertificate(t *testing.T) {
	rh := newRenewHarness(t)

	rec, _ := rh.renew(t, "cert_missing")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 — a 200 here would be counted as a successful "+
			"renewal by any script iterating certificate IDs", rec.Code)
	}
}

// TestRenewFailsClosedWithoutCertStore matches the convention on the rest of this
// surface.
func TestRenewFailsClosedWithoutCertStore(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/certificates/x/renew", nil)
	req.SetPathValue("id", "x")
	rec := httptest.NewRecorder()

	defer func() {
		if p := recover(); p != nil {
			t.Fatalf("panicked on missing wiring: %v", p)
		}
	}()
	h.AdminRenewCert(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rec.Code)
	}
}

// NOTE on the 202 path: when a renewal succeeds but the provider reload does not,
// the handler returns 202 and marks pending state. That distinction is the most
// consequential one on this endpoint — 200 means the gateway serves the new
// certificate, 202 means it still serves the old one.
//
// ACMEClient is now an interface (see acme_provider.go), so a renewal can be made
// to succeed in a test. The remaining blocker is h.Apply: AdminRenewCert calls
// prepareACMEHTTP01 first, which requires a concrete *apply.AppService, so a
// local-ACME renewal stops at 503 before the renewer is reached. That ordering is
// itself correct and is pinned by TestRenewReachesTheACMEClientForLocalACMECerts —
// placing an order with no challenge route would burn a rate-limit slot.
//
// The service-level equivalents of the 202 state are covered in certstore:
// TestRenewReportsReloadFailureWithoutLosingRenewedState and
// TestRenewReportsCoordinatedApplyFailureWithoutLosingRenewedState. The HTTP status
// mapping stays unpinned until h.Apply has a seam — 24 call sites across 7 methods,
// including the mode-switch and apply surfaces.
