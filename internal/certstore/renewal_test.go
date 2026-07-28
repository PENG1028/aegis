package certstore

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

type renewalStub struct{ called bool }

func (s *renewalStub) RenewCertificate(_ context.Context, certID string, _ []string) (string, error) {
	s.called = true
	return certID, nil
}

type reloadStub struct {
	called bool
	err    error
}

type coordinatorStub struct {
	called bool
	err    error
}

func (s *coordinatorStub) RenewCertificateAndApply(_ context.Context, mutate func() error) error {
	s.called = true
	if err := mutate(); err != nil {
		return err
	}
	return s.err
}

func (s *reloadStub) ReloadCertificateConsumers() error {
	s.called = true
	return s.err
}

func TestRenewLocalACMEReloadsCertificateConsumers(t *testing.T) {
	svc := renewalTestService(t)
	renewer := &renewalStub{}
	reloader := &reloadStub{}
	checker := NewCertRenewalChecker(svc, renewer)
	checker.SetProviderReloader(reloader)

	result, err := checker.Renew(context.Background(), "cert_acme")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renewed || !renewer.called || !reloader.called {
		t.Fatalf("renewal did not complete full lifecycle: %+v", result)
	}
}

func TestRenewLocalACMEUsesApplyCoordinator(t *testing.T) {
	svc := renewalTestService(t)
	renewer := &renewalStub{}
	coordinator := &coordinatorStub{}
	checker := NewCertRenewalChecker(svc, renewer)
	checker.SetRenewalCoordinator(coordinator)
	result, err := checker.Renew(context.Background(), "cert_acme")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renewed || !renewer.called || !coordinator.called || result.PendingApply {
		t.Fatalf("coordinated renewal failed: %+v", result)
	}
}

func TestRenewMissingCertificateReturnsError(t *testing.T) {
	checker := NewCertRenewalChecker(renewalTestService(t), &renewalStub{})
	if _, err := checker.Renew(context.Background(), "cert_missing"); err == nil {
		t.Fatal("missing certificate did not return an error")
	}
}

func TestRenewReportsReloadFailureWithoutLosingRenewedState(t *testing.T) {
	checker := NewCertRenewalChecker(renewalTestService(t), &renewalStub{})
	checker.SetProviderReloader(&reloadStub{err: context.DeadlineExceeded})
	result, err := checker.Renew(context.Background(), "cert_acme")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renewed || !result.PendingApply || !strings.Contains(result.Message, "reload failed") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRenewReportsCoordinatedApplyFailureWithoutLosingRenewedState(t *testing.T) {
	checker := NewCertRenewalChecker(renewalTestService(t), &renewalStub{})
	checker.SetRenewalCoordinator(&coordinatorStub{err: context.DeadlineExceeded})
	result, err := checker.Renew(context.Background(), "cert_acme")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Renewed || !result.PendingApply || !strings.Contains(result.Message, "apply failed") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func renewalTestService(t *testing.T) *Service {
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
		VALUES (?, ?, '', ?, ?, '', '', ?, '', ?, ?)`,
		"cert_acme", `["app.example.com"]`, now.Add(-time.Hour).Format(time.RFC3339),
		now.Add(20*24*time.Hour).Format(time.RFC3339), SourceLocalACME,
		now.Format(time.RFC3339), now.Format(time.RFC3339))
	if err != nil {
		t.Fatal(err)
	}
	return NewService(NewRepository(db), t.TempDir())
}
