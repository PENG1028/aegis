package secrets

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDevEphemeralKeyIsUnstable documents WHY production cannot use
// LoadMasterKey(true): every call generates a fresh random key, so any
// AES-GCM data encrypted with one boot's key is unrecoverable after restart.
// This is the regression test for the v1.9C-3 fix that replaced the
// production path with EnsureMasterKey (stable, persisted).
func TestDevEphemeralKeyIsUnstable(t *testing.T) {
	k1, err := LoadMasterKey(true)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := LoadMasterKey(true)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(k1.KeyMaterial(), k2.KeyMaterial()) {
		t.Fatal("ephemeral dev keys must differ per call — production must never rely on them")
	}
}

// TestEnsureMasterKeyStableAcrossRestarts is the core fix test: the first
// call generates and PERSISTS a key; a second call (simulating a restart)
// must return the identical key material. Before the fix this function did
// not exist and the production path used ephemeral keys (data loss).
func TestEnsureMasterKeyStableAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")

	mk1, err := EnsureMasterKey(path)
	if err != nil {
		t.Fatalf("first EnsureMasterKey: %v", err)
	}
	mk2, err := EnsureMasterKey(path)
	if err != nil {
		t.Fatalf("second EnsureMasterKey (restart): %v", err)
	}
	if !bytes.Equal(mk1.KeyMaterial(), mk2.KeyMaterial()) {
		t.Fatal("master key changed across restarts — encrypted data would be unrecoverable")
	}
}

// TestEnsureMasterKeyPersistsWith0600 verifies the persisted key file is
// root-only readable, matching the security posture of the key material.
func TestEnsureMasterKeyPersistsWith0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	if _, err := EnsureMasterKey(path); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not enforced on Windows")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("key file perms = %o, want 0600", perm)
	}
}

// TestEnsureMasterKeyReusesExistingKey verifies an already-present key file
// is reused (not overwritten with a new key — which would also break
// encrypted data).
func TestEnsureMasterKeyReusesExistingKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.key")
	mk1, err := EnsureMasterKey(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureMasterKey(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("existing key file was overwritten")
	}
	if !bytes.Equal(mk1.KeyMaterial(), mustLoad(t, path).KeyMaterial()) {
		t.Fatal("loaded key does not match persisted key")
	}
}

func mustLoad(t *testing.T, path string) *MasterKey {
	t.Helper()
	mk, err := LoadMasterKeyFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return mk
}
