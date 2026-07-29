package manageddomain

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"aegis/internal/logs"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// newTestAppService builds an AppService over an in-memory database. The real
// logs.AppService is used rather than a stub: the Logger interface is wide and
// the writes are incidental to what these tests assert.
func newTestAppService(t *testing.T) (*AppService, context.Context) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	return NewAppService(NewRepository(db), logs.NewAppService(logs.NewRepository(db))), context.Background()
}

// TestTLSStatusNeverAdvances pins that managed domains do not carry live TLS
// state. TLSStatus was previously initialised to "pending" while the field
// comment advertised issued/failed/disabled — values no code path could ever
// produce. A reader (or a future UI) would reasonably treat "pending" as an
// in-flight issuance that resolves on its own. Nothing issues certificates for
// a managed domain, so it never resolved.
//
// If an issuance path is added later, this test should fail — that is the signal
// to advance the field deliberately and update the comment, not to loosen the
// assertion.
func TestTLSStatusNeverAdvances(t *testing.T) {
	svc, ctx := newTestAppService(t)

	md, err := svc.CreateManagedDomain(ctx, CreateManagedDomainInput{
		Domain:     "tls-status.example.com",
		ServiceID:  "svc_1",
		OwnerRef:   "owner_1",
		TargetType: "custom",
		TargetRef:  "ref_1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if md.TLSStatus != TLSStatusNotRequested {
		t.Fatalf("at creation: got %q, want %q", md.TLSStatus, TLSStatusNotRequested)
	}

	// Drive every state transition the package exposes. None of them may touch
	// TLSStatus, because none of them requests a certificate.
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"verify", func() error { _, _, err := svc.VerifyDomain(ctx, md.ID); return err }},
		{"enable_force", func() error { _, err := svc.EnableDomain(ctx, md.ID, true); return err }},
		{"disable", func() error { _, err := svc.DisableDomain(ctx, md.ID); return err }},
	} {
		// Errors are expected and irrelevant here (DNS lookups fail in tests);
		// the assertion is that TLSStatus is untouched either way.
		_ = step.run()

		got, err := svc.GetManagedDomain(ctx, md.ID)
		if err != nil {
			t.Fatalf("after %s: reload: %v", step.name, err)
		}
		if got.TLSStatus != TLSStatusNotRequested {
			t.Errorf("after %s: TLSStatus advanced to %q — if issuance was wired, update the field comment and this test",
				step.name, got.TLSStatus)
		}
	}
}

// TestTLSStatusValueIsSelfDescribing guards the wire contract: the value is
// surfaced verbatim by the API and CLI, so it must read as "no request was made"
// rather than implying something is in progress.
func TestTLSStatusValueIsSelfDescribing(t *testing.T) {
	if TLSStatusNotRequested == "pending" {
		t.Fatal(`TLSStatus must not be "pending" — it implies an in-flight issuance that never happens`)
	}
	if !strings.Contains(TLSStatusNotRequested, "not") {
		t.Errorf("value %q should read as an absence of a request", TLSStatusNotRequested)
	}
}
