package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/apply"
	"aegis/internal/certstore"
	"aegis/internal/cluster"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// fakeApply is a scriptable ApplyService. Its purpose is to let the renewal
// handler reach its own success and partial-success branches, which the concrete
// *apply.AppService put behind real config rendering and gateway reloads.
type fakeApply struct {
	forceApplyErr error

	// coordinatorErr is returned by RenewCertificateAndApply *after* the mutate
	// callback runs, modelling the real failure: ACME issued the certificate and
	// it is on disk, but the gateway reload that would serve it did not happen.
	coordinatorErr error

	forceApplyCalls int
	coordinatorRuns int
	mutateRan       bool
}

func (f *fakeApply) DryRun(context.Context) (*apply.ApplyPlan, error) {
	return &apply.ApplyPlan{}, nil
}
func (f *fakeApply) TryApply(context.Context) (*apply.ApplyPlan, error) {
	return &apply.ApplyPlan{}, nil
}
func (f *fakeApply) Apply(context.Context) (*apply.ApplyPlan, error) {
	return &apply.ApplyPlan{}, nil
}
func (f *fakeApply) ForceApply(context.Context) (*apply.ApplyPlan, error) {
	f.forceApplyCalls++
	if f.forceApplyErr != nil {
		return nil, f.forceApplyErr
	}
	return &apply.ApplyPlan{}, nil
}
func (f *fakeApply) SwitchMode(context.Context, string) error { return nil }
func (f *fakeApply) History(context.Context) ([]apply.ApplyVersion, error) {
	return nil, nil
}
func (f *fakeApply) GetCurrentConfig() (string, error) { return "", nil }

func (f *fakeApply) RenewCertificateAndApply(_ context.Context, mutate func() error) error {
	f.coordinatorRuns++
	if err := mutate(); err != nil {
		return err
	}
	f.mutateRan = true
	return f.coordinatorErr
}

var _ ApplyService = (*fakeApply)(nil)

// pendingRecorder captures what the handler reports to the cluster's pending
// state. The 202 is only half the signal; the pending mark is what makes the
// stale certificate visible after the response is gone.
type pendingHarness struct {
	h  *Handlers
	fa *fakeApply
	fc *fakeACME
	ps *cluster.PendingState
}

func newPendingHarness(t *testing.T, fa *fakeApply, fc *fakeACME) *pendingHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	_, err = db.Exec(`INSERT INTO certificates
		(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
		VALUES ('cert_acme', '["app.example.com"]', 'test', ?, ?, '', '', ?, '', ?, ?)`,
		now.Add(-time.Hour).Format(time.RFC3339), now.Add(240*time.Hour).Format(time.RFC3339),
		certstore.SourceLocalACME, now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}

	return &pendingHarness{
		h: &Handlers{
			CertStore:    certstore.NewService(certstore.NewRepository(db), t.TempDir()),
			ACMEClient:   fc,
			Apply:        fa,
			PendingState: cluster.NewPendingState(db),
		},
		fa: fa,
		fc: fc,
		ps: cluster.NewPendingState(db),
	}
}

func (p *pendingHarness) renew(t *testing.T) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/certificates/cert_acme/renew", nil)
	req.SetPathValue("id", "cert_acme")
	rec := httptest.NewRecorder()
	p.h.AdminRenewCert(rec, req)

	body := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
	}
	return rec, body
}

// TestRenewReturns202WhenTheGatewayStillServesTheOldCertificate is the assertion
// the debug scripts could never make, and the reason both seams were opened.
//
// ACME issued the new certificate and it is on disk. The apply that would make the
// gateway serve it failed. Nothing is broken — the old certificate is still valid
// and traffic is fine — but the renewal did not take effect, and it will expire on
// its original schedule.
//
// 200 here is the dangerous answer: an operator or a cron job reading it marks the
// domain as renewed and stops watching. 202 is the honest one.
func TestRenewReturns202WhenTheGatewayStillServesTheOldCertificate(t *testing.T) {
	fa := &fakeApply{coordinatorErr: fmt.Errorf("caddy reload refused: config validation failed")}
	fc := &fakeACME{available: true, renewedID: "cert_renewed"}
	p := newPendingHarness(t, fa, fc)

	rec, body := p.renew(t)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 — the certificate was renewed but the gateway still "+
			"serves the old pair, and 200 would stop anyone from following up: %s",
			rec.Code, rec.Body.String())
	}
	if pending, _ := body["pending_apply"].(bool); !pending {
		t.Error("pending_apply must be true — it is what tells the caller the renewal is " +
			"not yet in effect")
	}
	if renewed, _ := body["renewed"].(bool); !renewed {
		t.Error("renewed must stay true: the new certificate exists on disk, and reporting " +
			"otherwise would invite a second order against the rate limit")
	}
	if msg, _ := body["message"].(string); msg == "" {
		t.Error("message must carry the reload failure; without it the operator cannot tell " +
			"why the renewal did not take effect")
	}

	if !fa.mutateRan {
		t.Error("the renewal never ran inside the coordinator — PEM replacement and reload must " +
			"share the apply lock so validation cannot observe a mismatched pair")
	}
	if fa.coordinatorRuns != 1 {
		t.Errorf("coordinator ran %d times, want exactly 1", fa.coordinatorRuns)
	}

	// The 202 lives only as long as the response. The pending mark is what keeps
	// the stale certificate visible to the cluster afterwards.
	st := p.ps.Status()
	if !st.Pending {
		t.Error("cluster pending state was not marked — once the response is gone, nothing " +
			"records that the gateway is serving a superseded certificate")
	}
	if st.Reason == "" {
		t.Error("the pending mark carries no reason; an operator finding it later has no " +
			"way to know which certificate is stale")
	}
}

// TestRenewReturns200WhenTheGatewayPickedUpTheNewCertificate is the other half of
// the pair. Without it, a handler that always returned 202 would pass the test
// above while making every successful renewal look unfinished.
func TestRenewReturns200WhenTheGatewayPickedUpTheNewCertificate(t *testing.T) {
	fa := &fakeApply{}
	fc := &fakeACME{available: true, renewedID: "cert_renewed"}
	p := newPendingHarness(t, fa, fc)

	rec, body := p.renew(t)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 when the reload succeeded: %s", rec.Code, rec.Body.String())
	}
	if pending, _ := body["pending_apply"].(bool); pending {
		t.Error("pending_apply must be false — a successful renewal reported as pending would " +
			"send the operator looking for an apply that is not needed")
	}
	if renewed, _ := body["renewed"].(bool); !renewed {
		t.Error("renewed must be true")
	}
}

// TestRenewPreparesTheChallengeRouteBeforeOrdering pins the ordering that the
// earlier 503 test could only observe by its absence. Now that ForceApply can
// succeed, assert it actually ran before the renewer was called: an order placed
// with no challenge route reachable fails validation and still counts against the
// directory's rate limit.
func TestRenewPreparesTheChallengeRouteBeforeOrdering(t *testing.T) {
	fa := &fakeApply{}
	fc := &fakeACME{available: true, renewedID: "cert_renewed"}
	p := newPendingHarness(t, fa, fc)

	if _, _ = p.renew(t); fa.forceApplyCalls != 1 {
		t.Errorf("ForceApply ran %d times, want 1 — the HTTP-01 route must be installed before "+
			"the order", fa.forceApplyCalls)
	}
	if len(fc.renewCalls) != 1 || fc.renewCalls[0] != "cert_acme" {
		t.Errorf("renewer calls = %v, want exactly one for cert_acme", fc.renewCalls)
	}
}

// TestRenewDoesNotOrderWhenChallengeRoutePreparationFails is the negative of the
// above, now provable rather than incidental: a failing ForceApply must stop the
// request before any ACME traffic.
func TestRenewDoesNotOrderWhenChallengeRoutePreparationFails(t *testing.T) {
	fa := &fakeApply{forceApplyErr: fmt.Errorf("render failed: upstream unreachable")}
	fc := &fakeACME{available: true, renewedID: "cert_renewed"}
	p := newPendingHarness(t, fa, fc)

	rec, _ := p.renew(t)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503: %s", rec.Code, rec.Body.String())
	}
	if len(fc.renewCalls) != 0 {
		t.Errorf("an order was placed with no challenge route (%v) — it would fail validation "+
			"and burn a rate-limit slot", fc.renewCalls)
	}
	if fa.coordinatorRuns != 0 {
		t.Error("the coordinator ran despite challenge preparation failing")
	}
}
