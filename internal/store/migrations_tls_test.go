package store

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigration046SeparatesManagedTLSFromCertificateAssets(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE certificates (id TEXT PRIMARY KEY, source TEXT NOT NULL);
		CREATE TABLE routes (
			id TEXT PRIMARY KEY, composition TEXT NOT NULL, tls_enabled INTEGER NOT NULL,
			source_provider TEXT NOT NULL DEFAULT '', cert_id TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO certificates VALUES ('cert_auto', 'gateway_auto');
		INSERT INTO certificates VALUES ('cert_manual', 'manual_upload');
		INSERT INTO routes VALUES ('rt_auto', 'https_route', 1, 'caddy', 'cert_auto');
		INSERT INTO routes VALUES ('rt_manual', 'https_route', 1, 'caddy', 'cert_manual');
		INSERT INTO routes VALUES ('rt_http', 'http_route', 0, 'caddy', 'cert_manual');
		INSERT INTO routes VALUES ('rt_dangling', 'https_route', 1, 'caddy', 'cert_missing');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migration046); err != nil {
		t.Fatal(err)
	}

	assertTLSMigrationRow(t, db, "rt_auto", "provider_auto", "caddy", "")
	assertTLSMigrationRow(t, db, "rt_manual", "certificate", "", "cert_manual")
	assertTLSMigrationRow(t, db, "rt_http", "off", "", "")
	assertTLSMigrationRow(t, db, "rt_dangling", "provider_auto", "caddy", "")
	var autoCount, manualCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM certificates WHERE id='cert_auto'`).Scan(&autoCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM certificates WHERE id='cert_manual'`).Scan(&manualCount); err != nil {
		t.Fatal(err)
	}
	if autoCount != 0 || manualCount != 1 {
		t.Fatalf("certificate migration counts: auto=%d manual=%d", autoCount, manualCount)
	}
}

func TestMigration047GuardsCertificateReferences(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE certificates (id TEXT PRIMARY KEY);
		CREATE TABLE routes (id TEXT PRIMARY KEY, cert_id TEXT NOT NULL DEFAULT '');
		INSERT INTO certificates VALUES ('cert_manual');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(migration047); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO routes VALUES ('rt_missing', 'cert_missing')`); err == nil {
		t.Fatal("route accepted a missing certificate")
	}
	if _, err := db.Exec(`INSERT INTO routes VALUES ('rt_bound', 'cert_manual')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM certificates WHERE id='cert_manual'`); err == nil {
		t.Fatal("referenced certificate was deleted")
	}
}

func assertTLSMigrationRow(t *testing.T, db *sql.DB, id, mode, providerID, certID string) {
	t.Helper()
	var gotMode, gotProvider, gotCert string
	if err := db.QueryRow(`SELECT tls_binding_mode, tls_provider, cert_id FROM routes WHERE id=?`, id).
		Scan(&gotMode, &gotProvider, &gotCert); err != nil {
		t.Fatal(err)
	}
	if gotMode != mode || gotProvider != providerID || gotCert != certID {
		t.Fatalf("%s: got mode=%q provider=%q cert=%q", id, gotMode, gotProvider, gotCert)
	}
}
