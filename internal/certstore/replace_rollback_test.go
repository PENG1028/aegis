package certstore

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// replaceFixture builds two local-ACME certificates with real PEM files on disk,
// which is what ReplaceWith operates on.
func replaceFixture(t *testing.T) (*Service, string, string) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	oldCert, oldKey := write("old.crt", "OLD-CERT"), write("old.key", "OLD-KEY")
	newCert, newKey := write("new.crt", "NEW-CERT"), write("new.key", "NEW-KEY")

	now := time.Now().UTC()
	ins := func(id, cert, key string) {
		_, err := db.Exec(`INSERT INTO certificates
			(id, domains, issuer, not_before, not_after, cert_path, key_path, source, note, created_at, updated_at)
			VALUES (?, '["app.example.com"]', 'test', ?, ?, ?, ?, ?, '', ?, ?)`,
			id, now.Add(-time.Hour).Format(time.RFC3339), now.Add(720*time.Hour).Format(time.RFC3339),
			cert, key, SourceLocalACME, now.Format(time.RFC3339), now.Format(time.RFC3339))
		if err != nil {
			t.Fatal(err)
		}
	}
	ins("cert_existing", oldCert, oldKey)
	ins("cert_replacement", newCert, newKey)

	return NewService(NewRepository(db), dir), oldCert, oldKey
}

// TestReplaceErrorDistinguishesRecoveredFromBrokenState pins the contract that
// makes the typed error worth having.
//
// A renewal that fails after the certificate file is written but before the key is
// leaves a mismatched pair on disk, and every gateway refuses to load one. The
// caller has to tell "renewal failed, disk restored" (safe to retry) from
// "renewal failed and the asset is now broken" (needs a human) without parsing
// prose. Before this type, both cases returned the same shape of error and the
// rollback outcome was discarded entirely.
func TestReplaceErrorDistinguishesRecoveredFromBrokenState(t *testing.T) {
	recovered := &ReplaceError{
		Reason:         "replace key file: disk full",
		RollbackStatus: RollbackComplete,
	}
	if !strings.Contains(recovered.Error(), "restored") {
		t.Errorf("a recovered failure must say the previous certificate is back, got: %v", recovered)
	}
	if strings.Contains(recovered.Error(), "may not match") {
		t.Errorf("a recovered failure must not imply disk inconsistency, got: %v", recovered)
	}

	broken := &ReplaceError{
		Reason:           "replace key file: disk full",
		RollbackStatus:   RollbackIncomplete,
		RollbackFailures: []string{"restore certificate /etc/ssl/app.crt: permission denied"},
	}
	msg := broken.Error()
	if !strings.Contains(msg, "may not match") {
		t.Errorf("an unrecovered failure must state the pair on disk may be mismatched, got: %v", msg)
	}
	if !strings.Contains(msg, "/etc/ssl/app.crt") {
		t.Errorf("the message must name the file a human has to inspect, got: %v", msg)
	}
	if !strings.Contains(msg, "disk full") {
		t.Errorf("the original cause must survive alongside the rollback outcome, got: %v", msg)
	}
}

// TestReplaceWithSucceedsInPlaceAndKeepsBindings guards the property the whole
// in-place design exists for: the surviving certificate ID must be the original,
// because routes and rendered gateway config point at it. If a renewal swapped in
// the new ID instead, every binding would dangle.
func TestReplaceWithSucceedsInPlaceAndKeepsBindings(t *testing.T) {
	svc, oldCert, oldKey := replaceFixture(t)

	if err := svc.ReplaceWith("cert_existing", "cert_replacement"); err != nil {
		t.Fatalf("replace failed on the happy path: %v", err)
	}

	cert, err := os.ReadFile(oldCert)
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(oldKey)
	if err != nil {
		t.Fatal(err)
	}
	if string(cert) != "NEW-CERT" || string(key) != "NEW-KEY" {
		t.Errorf("in-place replace did not install the new pair: cert=%q key=%q", cert, key)
	}

	if got, _ := svc.Get("cert_replacement"); got != nil {
		t.Error("replacement record should be deleted after an in-place renewal")
	}
	surviving, _ := svc.Get("cert_existing")
	if surviving == nil {
		t.Fatal("the existing certificate ID must survive so bindings stay valid")
	}
	if surviving.CertPath != oldCert || surviving.KeyPath != oldKey {
		t.Errorf("paths must not move during in-place renewal: cert=%q key=%q",
			surviving.CertPath, surviving.KeyPath)
	}
}

// TestReplaceWithRejectsNonACMESources guards the precondition. Replacing a
// manually uploaded asset in place would overwrite a file the operator supplied
// and cannot regenerate.
func TestReplaceWithRejectsNonACMESources(t *testing.T) {
	svc, _, _ := replaceFixture(t)

	// Demote the replacement to an upload and confirm the swap is refused.
	if _, err := svc.repo.db.Exec(
		`UPDATE certificates SET source = ? WHERE id = ?`, SourceManualUpload, "cert_replacement",
	); err != nil {
		t.Fatal(err)
	}
	err := svc.ReplaceWith("cert_existing", "cert_replacement")
	if err == nil {
		t.Fatal("in-place replace must be refused for non-ACME sources")
	}
	if !strings.Contains(err.Error(), "local ACME") {
		t.Errorf("refusal should explain the source restriction, got: %v", err)
	}
}
