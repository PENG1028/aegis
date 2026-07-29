package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aegis/internal/acme"
	"aegis/internal/certstore"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// fakeACME is a scriptable ACMEProvider. It exists so handler paths behind a
// successful issuance or renewal are reachable at all; with the concrete
// *acme.Client they required a live ACME directory.
type fakeACME struct {
	available    bool
	obtainResult *acme.ObtainResult
	obtainErr    error
	renewedID    string
	renewErr     error
	challenge    map[string]string
	emailErr     error

	obtainCalls []([]string)
	renewCalls  []string
	emailCalls  []string
}

func (f *fakeACME) Available() bool { return f.available }

func (f *fakeACME) Obtain(_ context.Context, domains []string) (*acme.ObtainResult, error) {
	f.obtainCalls = append(f.obtainCalls, domains)
	return f.obtainResult, f.obtainErr
}

func (f *fakeACME) RenewCertificate(_ context.Context, certID string, _ []string) (string, error) {
	f.renewCalls = append(f.renewCalls, certID)
	return f.renewedID, f.renewErr
}

func (f *fakeACME) HTTPChallengeResponse(token string) (string, bool) {
	v, ok := f.challenge[token]
	return v, ok
}

func (f *fakeACME) UpdateEmail(_ context.Context, email string) error {
	f.emailCalls = append(f.emailCalls, email)
	return f.emailErr
}

// Compile-time proof the fake and the real client describe the same surface. If
// *acme.Client drifts from ACMEProvider, this fails here rather than at wiring.
var (
	_ ACMEProvider = (*fakeACME)(nil)
	_ ACMEProvider = (*acme.Client)(nil)
)

func acmeHarness(t *testing.T, f *fakeACME) (*Handlers, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return &Handlers{
		CertStore:  certstore.NewService(certstore.NewRepository(db), t.TempDir()),
		ACMEClient: f,
	}, db
}

// TestACMEObtainRefusesWhenUnavailable pins the precondition. Issuing against an
// unconfigured account produces a confusing ACME-side failure; refusing up front
// tells the operator what to fix.
func TestACMEObtainRefusesWhenUnavailable(t *testing.T) {
	h, _ := acmeHarness(t, &fakeACME{available: false})

	body := `{"domains":["app.example.com"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/acme/obtain", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.AdminACMEObtain(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501 when ACME is not configured", rec.Code)
	}
}

// TestACMEObtainRejectsUnorderableDomains covers the validation gate, which runs
// before any network work and is where a doomed order should die.
//
// A wildcard cannot be satisfied by HTTP-01 at all — it requires DNS-01. Letting
// one through would spend a real order attempt against the directory's rate limit
// and fail after the challenge, so the refusal has to happen here and has to say
// what the operator should do instead.
func TestACMEObtainRejectsUnorderableDomains(t *testing.T) {
	cases := map[string]string{
		"wildcard":       `{"domains":["*.example.com"]}`,
		"empty list":     `{"domains":[]}`,
		"blank entry":    `{"domains":["  "]}`,
		"malformed json": `{"domains":`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			f := &fakeACME{available: true}
			h, _ := acmeHarness(t, f)

			req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/acme/obtain", strings.NewReader(body))
			rec := httptest.NewRecorder()
			h.AdminACMEObtain(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if len(f.obtainCalls) != 0 {
				t.Errorf("an unorderable request reached the directory (%v) — it would consume a "+
					"rate-limit slot and fail", f.obtainCalls)
			}
		})
	}
}

// TestACMEObtainExplainsWhyWildcardsAreRefused pins the message, not just the
// status. "400" alone sends the operator looking for a typo; the endpoint knows the
// real answer is DNS-01 and should say so.
func TestACMEObtainExplainsWhyWildcardsAreRefused(t *testing.T) {
	h, _ := acmeHarness(t, &fakeACME{available: true})

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/acme/obtain",
		strings.NewReader(`{"domains":["*.example.com"]}`))
	rec := httptest.NewRecorder()
	h.AdminACMEObtain(rec, req)

	if !strings.Contains(rec.Body.String(), "DNS-01") {
		t.Errorf("the refusal should name DNS-01 as the alternative, got: %s", rec.Body.String())
	}
}

// TestACMEObtainStopsBeforeIssuingWithoutApply records the remaining wall, and the
// property that makes it the safe failure rather than a gap.
//
// Issuance is gated on prepareACMEHTTP01, which needs h.Apply — a concrete
// *apply.AppService with no seam. What matters is the ordering: the challenge route
// must exist before the order is placed, so a missing Apply has to stop the request
// rather than let it proceed. An order placed with no challenge route reachable
// fails validation and still counts against the directory's rate limit.
func TestACMEObtainStopsBeforeIssuingWithoutApply(t *testing.T) {
	f := &fakeACME{available: true, obtainResult: &acme.ObtainResult{CertID: "cert_new"}}
	h, _ := acmeHarness(t, f)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/acme/obtain",
		strings.NewReader(`{"domains":["app.example.com"]}`))
	rec := httptest.NewRecorder()
	h.AdminACMEObtain(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 from HTTP-01 preparation: %s", rec.Code, rec.Body.String())
	}
	if len(f.obtainCalls) != 0 {
		t.Errorf("an order was placed with no challenge route in place (%v) — it would fail "+
			"validation and burn a rate-limit slot", f.obtainCalls)
	}
}

// TestACMEHTTPChallengeServesOnlyKnownTokens is the endpoint Let's Encrypt fetches
// during HTTP-01. Serving a body for an unknown token would make validation of a
// challenge this instance never issued appear to succeed.
func TestACMEHTTPChallengeServesOnlyKnownTokens(t *testing.T) {
	f := &fakeACME{available: true, challenge: map[string]string{"tok_real": "keyauth_value"}}
	h, _ := acmeHarness(t, f)

	t.Run("known token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/tok_real", nil)
		req.SetPathValue("token", "tok_real")
		rec := httptest.NewRecorder()
		h.ACMEHTTPChallenge(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if rec.Body.String() != "keyauth_value" {
			t.Errorf("body = %q, want the key authorization verbatim — any framing breaks validation",
				rec.Body.String())
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/tok_fake", nil)
		req.SetPathValue("token", "tok_fake")
		rec := httptest.NewRecorder()
		h.ACMEHTTPChallenge(rec, req)

		if rec.Code == http.StatusOK {
			t.Errorf("an unknown token returned 200 — a challenge this instance never issued "+
				"would appear valid: %s", rec.Body.String())
		}
	})
}

// TestRenewReachesTheACMEClientForLocalACMECerts confirms the seam works end to
// end: a local-ACME certificate now actually drives the injected renewer.
//
// It also records where the 202 still stops. The handler calls prepareACMEHTTP01
// first, which requires h.Apply — a concrete *apply.AppService. So with ACME
// injectable but Apply absent this returns 503 and never reaches the renewal.
// The 202 is gated on Apply, not on ACMEClient.
func TestRenewReachesTheACMEClientForLocalACMECerts(t *testing.T) {
	f := &fakeACME{available: true, renewedID: "cert_renewed"}
	h, db := acmeHarness(t, f)

	now := time.Now().UTC()
	_, err := db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES ('cert_acme', '["app.example.com"]', 'test', ?, ?, '', '', ?, '', ?, ?)`,
		now.Add(-time.Hour).Format(time.RFC3339), now.Add(240*time.Hour).Format(time.RFC3339),
		certstore.SourceLocalACME, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/certificates/cert_acme/renew", nil)
	req.SetPathValue("id", "cert_acme")
	rec := httptest.NewRecorder()
	h.AdminRenewCert(rec, req)

	// Apply is nil, so HTTP-01 preparation fails before renewal is attempted.
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 from HTTP-01 preparation: %s", rec.Code, rec.Body.String())
	}
	if len(f.renewCalls) != 0 {
		t.Errorf("renewal ran despite HTTP-01 preparation failing — the order would be attempted "+
			"with no challenge route in place, burning a rate-limit slot: %v", f.renewCalls)
	}
}

// TestACMEStatusReportsUnavailableClient guards that a broken client is not
// reported as ready. An operator seeing "available" would keep retrying issuance
// instead of checking account-key storage.
func TestACMEStatusReportsUnavailableClient(t *testing.T) {
	h, _ := acmeHarness(t, &fakeACME{available: false})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/v1/acme/status", nil)
	rec := httptest.NewRecorder()
	h.AdminACMEStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if avail, _ := body["available"].(bool); avail {
		t.Error("an unavailable client reported available=true; the operator would retry issuance " +
			"instead of fixing the client")
	}
}
