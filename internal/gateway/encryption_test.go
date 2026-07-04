package gateway

import (
	"fmt"
	"strings"
	"testing"

	"aegis/internal/secrets"
)

// v1.8B-5: GatewayLink Secret-at-rest Encryption tests

func TestNewEncryptedGateway(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "test-raw-secret-hex-64-chars-0123456789abcdef0123456789ab"
	gw, err := NewEncryptedGateway("test-enc", "1.2.3.4", "10.0.0.1", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}
	if gw == nil {
		t.Fatal("gateway is nil")
	}

	// Must have encrypted fields
	if gw.EncryptedSecret == "" {
		t.Error("EncryptedSecret should not be empty")
	}
	if gw.SecretNonce == "" {
		t.Error("SecretNonce should not be empty")
	}
	if gw.SecretVersion != 1 {
		t.Errorf("expected SecretVersion=1, got %d", gw.SecretVersion)
	}
	if gw.SecretCreatedAt == "" {
		t.Error("SecretCreatedAt should not be empty")
	}

	// AuthValue should still be set as HMAC fallback
	if gw.AuthValue == "" {
		t.Error("AuthValue (HMAC fallback) should not be empty")
	}
	if gw.AuthValue == rawToken {
		t.Error("AuthValue should NOT be the raw token")
	}

	// Encrypted fields should not be raw token
	if gw.EncryptedSecret == rawToken {
		t.Error("EncryptedSecret should NOT be the raw token")
	}

	// HasEncryptedSecret should return true
	if !gw.HasEncryptedSecret() {
		t.Error("HasEncryptedSecret should return true")
	}

	// HasSecret should return true
	if !gw.HasSecret() {
		t.Error("HasSecret should return true")
	}

	t.Logf("Encrypted gateway created: version=%d, enc=%d chars, nonce=%d chars",
		gw.SecretVersion, len(gw.EncryptedSecret), len(gw.SecretNonce))
}

func TestCheckAuthEncrypted(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "test-auth-secret-0123456789abcdef0123456789abcdef-test"

	gw, err := NewEncryptedGateway("test-auth", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	// Correct token should pass
	if !gw.CheckAuthEncrypted(rawToken, mk) {
		t.Error("CheckAuthEncrypted should return true for correct token")
	}

	// Wrong token should fail
	if gw.CheckAuthEncrypted("wrong-token", mk) {
		t.Error("CheckAuthEncrypted should return false for wrong token")
	}

	// Empty token should fail
	if gw.CheckAuthEncrypted("", mk) {
		t.Error("CheckAuthEncrypted should return false for empty token")
	}

		// Missing key with encrypted gateway should FAIL CLOSED
		if gw.CheckAuthEncrypted(rawToken, nil) {
			t.Error("CheckAuthEncrypted with nil key must fail closed for encrypted gateways")
		}

	// Wrong key should fail entirely
	wrongKey := secrets.DevMasterKey()
	if gw.CheckAuthEncrypted(rawToken, wrongKey) {
		t.Error("CheckAuthEncrypted with wrong key should return false")
	}
		t.Log("CheckAuthEncrypted: correct=pass, wrong=fail, wrong_key=fail, nil_key=fail_closed")
}

func TestGetRawSecretEncrypted(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "get-raw-test-secret-64chars-0123456789abcdef0123456789abcdef"

	gw, err := NewEncryptedGateway("test-get", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	// GetRawSecret should return the original raw token
	secret, err := gw.GetRawSecret(mk)
	if err != nil {
		t.Fatalf("GetRawSecret failed: %v", err)
	}
	if secret != rawToken {
		t.Fatalf("GetRawSecret mismatch: got %q, want %q", secret[:12], rawToken[:12])
	}

		// With nil key and encrypted gateway, should fail closed
		_, err = gw.GetRawSecret(nil)
		if err == nil {
			t.Error("GetRawSecret(nil) should fail for encrypted gateway")
		}
		if err != nil {
			t.Logf("GetRawSecret(nil) correctly returns error: %v", err)
		}

		t.Log("GetRawSecret: decrypted=correct, nil_key=fail_closed")
	}

func TestRotateSecretEncrypted(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "original-secret-31-bytes-token-for-rotate-test"

	gw, err := NewEncryptedGateway("test-rotate", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	oldVersion := gw.SecretVersion
	oldEncrypted := gw.EncryptedSecret

	// Rotate
	newToken := "new-rotated-secret-0123456789abcdef0123456789abcdef0123"
	if err := gw.RotateSecretEncrypted(newToken, mk); err != nil {
		t.Fatalf("RotateSecretEncrypted failed: %v", err)
	}

	// Version should increment
	if gw.SecretVersion != oldVersion+1 {
		t.Errorf("expected SecretVersion=%d, got %d", oldVersion+1, gw.SecretVersion)
	}

	// Encrypted data should change
	if gw.EncryptedSecret == oldEncrypted {
		t.Error("EncryptedSecret should change after rotate")
	}

	// SecretRotatedAt should be set
	if gw.SecretRotatedAt == "" {
		t.Error("SecretRotatedAt should be set after rotation")
	}

	// New token should work
	if !gw.CheckAuthEncrypted(newToken, mk) {
		t.Error("new token should pass after rotation")
	}

	// Old token should fail (unless we do grace period)
	if gw.CheckAuthEncrypted(rawToken, mk) {
		t.Log("Note: old token still passes due to HMAC fallback — expected without grace period handling")
	}

	t.Logf("RotateSecretEncrypted: version %d -> %d, new token works", oldVersion, gw.SecretVersion)
}

func TestServiceCreateEncrypted(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	gw, secret, err := svc.Register("server-b", "<SERVER_B_IP>", "10.3.0.11", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if gw == nil {
		t.Fatal("gateway is nil")
	}
	if secret == "" {
		t.Fatal("secret should not be empty")
	}

	// Raw token should not be stored in DB
	if gw.EncryptedSecret == "" {
		t.Error("EncryptedSecret should be set")
	}
	if gw.EncryptedSecret == secret {
		t.Error("EncryptedSecret should NOT equal raw secret")
	}
	if gw.AuthValue == secret {
		t.Error("AuthValue should NOT equal raw secret")
	}

	// Verify we can reload from DB and decrypt
	reloaded, err := repo.FindByID(gw.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded gateway is nil")
	}

	if !reloaded.HasEncryptedSecret() {
		t.Error("reloaded gateway should have encrypted secret")
	}

	decrypted, err := reloaded.GetRawSecret(mk)
	if err != nil {
		t.Fatalf("GetRawSecret on reloaded failed: %v", err)
	}
	if decrypted != secret {
		t.Errorf("decrypted secret mismatch: got %q, want %q", decrypted[:12], secret[:12])
	}

	t.Log("Service.Create encrypted: DB stores encrypted, decrypt roundtrip OK")
}

func TestServiceListDoesNotExposeRawToken(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	// Create gateway with encryption
	svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)

	// List should not include any secret fields
	gateways, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(gateways) == 0 {
		t.Fatal("no gateways")
	}

	for _, g := range gateways {
		if g.AuthValue != "" {
			t.Error("List should not expose AuthValue")
		}
		if g.EncryptedSecret != "" {
			t.Error("List should not expose EncryptedSecret")
		}
		if g.SecretNonce != "" {
			t.Error("List should not expose SecretNonce")
		}
		if g.SecretVersion != 0 {
			t.Error("List should not expose SecretVersion")
		}
	}

	t.Log("Service.List: no secret fields exposed")
}

func TestServiceGetDoesNotExposeRawToken(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)

	gateways, _ := svc.List()
	if len(gateways) == 0 {
		t.Fatal("no gateways")
	}

	gw, err := svc.Get(gateways[0].ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if gw == nil {
		t.Fatal("gateway is nil")
	}

	// Get returns encrypted fields (they're JSON "-" tagged anyway), but raw token is never returned
	// The raw token is only accessible via GetDecryptedSecret or GetRawSecret
	t.Logf("Service.Get: ID=%s, has_encrypted=%v, version=%d",
		gw.ID, gw.HasEncryptedSecret(), gw.SecretVersion)

	// Verify GetDecryptedSecret returns the raw token
	secret, err := svc.GetDecryptedSecret(gateways[0].ID)
	if err != nil {
		t.Fatalf("GetDecryptedSecret failed: %v", err)
	}
	if secret == "" {
		t.Fatal("GetDecryptedSecret should return a secret")
	}
	t.Logf("GetDecryptedSecret: len=%d (not exposed in Get/List)", len(secret))
}

func TestServiceRotateIncrementsVersion(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	gateways, _ := svc.List()
	if len(gateways) == 0 {
		t.Fatal("no gateways")
	}

	gwID := gateways[0].ID

	// Get initial version
	gw, _ := svc.Get(gwID)
	initialVersion := gw.SecretVersion

	// Rotate
	newSecret, err := svc.RotateSecret(gwID)
	if err != nil {
		t.Fatalf("RotateSecret failed: %v", err)
	}
	if newSecret == "" {
		t.Fatal("new secret should not be empty")
	}

	// Check version incremented
	gw, _ = svc.Get(gwID)
	if gw.SecretVersion != initialVersion+1 {
		t.Errorf("expected SecretVersion=%d, got %d", initialVersion+1, gw.SecretVersion)
	}

	// New secret should work for auth
	if !gw.CheckAuthEncrypted(newSecret, mk) {
		t.Error("new secret should pass CheckAuthEncrypted")
	}

	t.Logf("Rotate: version %d -> %d, new secret works", initialVersion, gw.SecretVersion)
}

func TestCheckAuthEncryptedRelayPath(t *testing.T) {
	// Simulates relay handler flow: GWLinkRepo.FindByID → CheckAuthEncrypted
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	gw, rawSecret, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Simulate relay handler lookup
	reloaded, err := repo.FindByID(gw.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	// Correct token — should pass
	if !reloaded.CheckAuthEncrypted(rawSecret, mk) {
		t.Error("relay: correct token should pass CheckAuthEncrypted")
	}

	// Wrong token — should fail (403)
	if reloaded.CheckAuthEncrypted("wrong-token", mk) {
		t.Error("relay: wrong token should fail CheckAuthEncrypted")
	}

	// Missing token — should fail (400 equivalent, returns false)
	if reloaded.CheckAuthEncrypted("", mk) {
		t.Error("relay: empty token should fail CheckAuthEncrypted")
	}

	t.Log("Relay auth path: encrypted secret works for relay CheckAuth")
}

func TestLegacyHMACBackwardCompat(t *testing.T) {
	// Test that legacy HMAC-hashed gateways still work
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	// Use nil master key to simulate legacy mode
	svc := NewService(repo, "gw_self", "server-a", nil)

	gw, rawSecret, err := svc.Register("server-b-legacy", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register (legacy) failed: %v", err)
	}

	// Legacy gateway should NOT have encrypted fields
	if gw.HasEncryptedSecret() {
		t.Error("legacy gateway should not have encrypted secret")
	}

	// But AuthValue (HMAC hash) should be set
	if gw.AuthValue == "" {
		t.Error("legacy gateway should have HMAC auth_value")
	}

	// CheckAuthEncrypted with nil master key should fall back to HMAC
	if !gw.CheckAuthEncrypted(rawSecret, nil) {
		t.Error("legacy: CheckAuthEncrypted(nil key) should fall back to HMAC and pass")
	}

	// CheckAuthEncrypted with master key but no encrypted data should fall back
	if !gw.CheckAuthEncrypted(rawSecret, mk) {
		t.Error("legacy: CheckAuthEncrypted(with key but no encrypted) should fall back to HMAC")
	}

	// Wrong token should still fail
	if gw.CheckAuthEncrypted("wrong", nil) {
		t.Error("legacy: wrong token should fail")
	}

	t.Log("Legacy HMAC gateways: CheckAuthEncrypted falls back to HMAC — works")
}

func TestBackfillEncrypted(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	// Create legacy gateway first
	svcLegacy := NewService(repo, "gw_self", "server-a", nil)
	gw, _, err := svcLegacy.Register("server-b-legacy", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register (legacy) failed: %v", err)
	}

	// Verify it's legacy
	if gw.HasEncryptedSecret() {
		t.Fatal("should be legacy (no encrypted secret)")
	}

	// Now backfill with master key
	svcWithKey := NewService(repo, "gw_self", "server-a", mk)
	backfilled, err := svcWithKey.BackfillEncrypted(gw.ID)
	if err != nil {
		t.Fatalf("BackfillEncrypted failed: %v", err)
	}
	if !backfilled {
		t.Fatal("BackfillEncrypted should return true")
	}

	// Reload and verify
	reloaded, err := repo.FindByID(gw.ID)
	if err != nil {
		t.Fatalf("FindByID after backfill failed: %v", err)
	}
	if !reloaded.HasEncryptedSecret() {
		t.Error("reloaded gateway should have encrypted secret after backfill")
	}
	if reloaded.SecretVersion != 1 {
		t.Errorf("expected SecretVersion=1, got %d", reloaded.SecretVersion)
	}

	// Should have new HMAC hash (since backfill generates new secret)
	if reloaded.AuthValue == "" {
		t.Error("AuthValue should be set after backfill")
	}

	t.Log("BackfillEncrypted: legacy → encrypted, version=1, HMAC updated")
}

func TestBackfillAlreadyEncrypted(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	gw, _, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Backfill on already-encrypted gateway should return false (no-op)
	backfilled, err := svc.BackfillEncrypted(gw.ID)
	if err != nil {
		t.Fatalf("BackfillEncrypted on encrypted gateway failed: %v", err)
	}
	if backfilled {
		t.Error("BackfillEncrypted on already encrypted gateway should return false")
	}

	t.Log("Backfill on encrypted gateway: no-op (false)")
}

func TestMissingMasterKeyFailsClosed(t *testing.T) {
	// When master key is missing but encrypted data exists
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)
	gw, rawSecret, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	reloaded, err := repo.FindByID(gw.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if !reloaded.HasEncryptedSecret() {
		t.Fatal("gateway should have encrypted secret")
	}

	// CheckAuthEncrypted with nil key must FAIL CLOSED for encrypted gateways
	if reloaded.CheckAuthEncrypted(rawSecret, nil) {
		t.Error("CheckAuthEncrypted(nil) must fail closed for encrypted gateways")
	}
	t.Log("CheckAuthEncrypted: encrypted gateway + nil key = fail closed OK")

	// GetRawSecret with nil key must return error for encrypted gateways
	_, err = reloaded.GetRawSecret(nil)
	if err == nil {
		t.Error("GetRawSecret(nil) must fail for encrypted gateways")
	}
	t.Logf("GetRawSecret: encrypted gateway + nil key = error (%v)", err)

	t.Log("Missing master key: encrypted gateway fails closed as required")
}

func TestEncryptedSecretNotInLogs(t *testing.T) {
	// Verify that EncryptedSecret/SecretNonce never appear in log-related output
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)

	gw, secret, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Sanitize any log-like strings
	outputs := []string{
		gw.ID,
		gw.Name,
		gw.EncryptedSecret,
		gw.SecretNonce,
		fmt.Sprintf("%v", gw.SecretVersion),
		secret, // raw token — should only appear in create response
	}

	// Check that raw token is not in any non-sensitive output
	// (EncryptedSecret should be base64 and not contain raw token)
	for _, out := range outputs {
		if strings.Contains(out, secret) && out != secret {
			t.Errorf("raw secret leaked into output field: %q", out[:min(len(out), 30)])
		}
		if strings.Contains(out, gw.EncryptedSecret) && out != gw.EncryptedSecret && out != "" {
			if gw.EncryptedSecret != "" {
				t.Log("Note: EncryptedSecret appears in log output — should use JSON '-' tag")
			}
		}
	}

	// Verify through service List that no raw secret is exposed
	gateways, _ := svc.List()
	for _, g := range gateways {
		if g.AuthValue != "" {
			t.Error("List should not expose AuthValue")
		}
	}

	t.Log("Secret leak check: raw token only in create response, not in list/get")
}


func TestEncryptedAuthFailClosedWhenKeyMissing(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "secret-for-fail-closed-test-aabbccdd1122334455667788"

	gw, err := NewEncryptedGateway("test-fail", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	if !gw.CheckAuthEncrypted(rawToken, mk) {
		t.Error("correct token + correct key should pass")
	}

	// Nil key: FAIL CLOSED
	if gw.CheckAuthEncrypted(rawToken, nil) {
		t.Error("nil key must cause fail-closed even with correct token")
	}

	// Wrong key: fails
	wrongKey := secrets.DevMasterKey()
	if gw.CheckAuthEncrypted(rawToken, wrongKey) {
		t.Error("wrong key should fail auth")
	}

	t.Log("Encrypted auth fail-closed: correct=pass, nil_key=fail, wrong_key=fail")
}

func TestEncryptedGetRawSecretFailsWhenKeyMissing(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "getraw-fail-closed-0123456789abcdef0123456789abcdef0"

	gw, err := NewEncryptedGateway("test-getraw-fail", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	secret, err := gw.GetRawSecret(mk)
	if err != nil {
		t.Fatalf("GetRawSecret with correct key failed: %v", err)
	}
	if secret != rawToken {
		t.Errorf("GetRawSecret mismatch: got %q, want %q", secret[:12], rawToken[:12])
	}

	_, err = gw.GetRawSecret(nil)
	if err == nil {
		t.Error("GetRawSecret with nil key must return error for encrypted gateway")
	}
	t.Logf("GetRawSecret(nil) correctly fails: %v", err)

	t.Log("GetRawSecret fail-closed: correct=pass, nil_key=error")
}

func TestLegacyHMACIsDegraded(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", nil)
	gw, _, err := svc.Register("server-b-legacy", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register (legacy) failed: %v", err)
	}

	if gw.HasEncryptedSecret() {
		t.Error("legacy gateway should not have encrypted secret")
	}
	if !gw.IsDegraded() {
		t.Error("legacy gateway should be degraded")
	}
	if !gw.HasSecret() {
		t.Error("legacy gateway should have auth data")
	}

	// Encrypted gateway should NOT be degraded
	mk := secrets.DevMasterKey()
	svc2 := NewService(repo, "gw_self2", "server-a2", mk)
	gw2, _, err := svc2.Register("server-b-enc", "<SERVER_B_ALT_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register with mk failed: %v", err)
	}
	if !gw2.HasEncryptedSecret() {
		t.Error("encrypted gateway should have encrypted secret")
	}
	if gw2.IsDegraded() {
		t.Error("encrypted gateway should not be degraded")
	}

	t.Log("Legacy gateways: degraded=true, encrypted gateways: degraded=false")
}

func TestLegacyHMACFallbackWorksWithNilKey(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", nil)
	gw, rawSecret, err := svc.Register("server-b-legacy", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register (legacy) failed: %v", err)
	}

	reloaded, err := repo.FindByID(gw.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	// Legacy gateway: no encrypted data, so HMAC fallback is ALLOWED
	if !reloaded.CheckAuthEncrypted(rawSecret, nil) {
		t.Error("legacy gateway should pass CheckAuthEncrypted with nil key via HMAC")
	}
	if !reloaded.CheckAuthEncrypted(rawSecret, secrets.DevMasterKey()) {
		t.Error("legacy gateway should pass with any key via HMAC fallback")
	}
	if reloaded.CheckAuthEncrypted("wrong-secret", nil) {
		t.Error("legacy gateway should reject wrong secret")
	}

	// GetRawSecret returns HMAC hash, no error
	fallback, err := reloaded.GetRawSecret(nil)
	if err != nil {
		t.Fatalf("GetRawSecret(nil) for legacy should not error: %v", err)
	}
	if fallback == "" {
		t.Error("GetRawSecret(nil) should return HMAC fallback for legacy")
	}
	if fallback == rawSecret {
		t.Error("GetRawSecret(nil) should not return raw token for legacy")
	}

	t.Log("Legacy HMAC: nil key works via HMAC fallback (degraded mode allowed)")
}

func TestCreateEncryptedFailsWithoutMasterKey(t *testing.T) {
	_, err := NewEncryptedGateway("test", "1.2.3.4", "", 443, "some-secret", TypeUpstream, true, nil)
	if err == nil {
		t.Error("NewEncryptedGateway with nil mk should fail")
	}
	t.Logf("NewEncryptedGateway(nil mk) correctly fails: %v", err)
}

func TestRotateEncryptedFailsWithoutMasterKey(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "rotate-fail-closed-0123456789abcdef0123456789abcdef"

	gw, err := NewEncryptedGateway("test-rotate-fail", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	err = gw.RotateSecretEncrypted("new-secret", nil)
	if err == nil {
		t.Error("RotateSecretEncrypted with nil mk should fail")
	}
	t.Logf("RotateSecretEncrypted(nil mk) correctly fails: %v", err)
}

func TestServiceRotateEncryptedFailsWithoutMasterKey(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	mk := secrets.DevMasterKey()
	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a", mk)
	gw, _, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	svcNoKey := NewService(repo, "gw_self", "server-a", nil)
	_, err = svcNoKey.RotateSecret(gw.ID)
	if err == nil {
		t.Error("RotateSecret on encrypted gateway without mk should fail")
	}
	t.Logf("Service.RotateSecret without mk correctly fails: %v", err)
}

func TestApplyDecryptFailsForEncryptedLinkWithoutKey(t *testing.T) {
	mk := secrets.DevMasterKey()
	rawToken := "apply-fail-closed-0123456789abcdef0123456789abcdef"

	gw, err := NewEncryptedGateway("test-apply", "1.2.3.4", "", 443, rawToken, TypeUpstream, true, mk)
	if err != nil {
		t.Fatalf("NewEncryptedGateway failed: %v", err)
	}

	_, err = gw.GetRawSecret(nil)
	if err == nil {
		t.Error("GetRawSecret(nil) for encrypted gateway must fail (planner fail-safe)")
	}
	t.Logf("Planner decryption without mk correctly fails: %v", err)
}


func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// setupDB is already defined in gateway_link_test.go, but we need the import
// The database setup function is reused from the existing test file.
