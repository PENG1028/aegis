package secrets

import (
	"strings"
	"testing"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	mk := DevMasterKey()
	if mk == nil {
		t.Fatal("DevMasterKey returned nil")
	}

	original := "my-64-char-hex-secret-0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	enc, nonce, err := Encrypt(mk, original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if enc == "" {
		t.Fatal("encrypted output is empty")
	}
	if nonce == "" {
		t.Fatal("nonce is empty")
	}

	decrypted, err := Decrypt(mk, enc, nonce)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != original {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decrypted, original)
	}
	t.Logf("Encrypt/Decrypt roundtrip: OK (enc=%d chars, nonce=%d chars)", len(enc), len(nonce))
}

func TestWrongKeyFails(t *testing.T) {
	mk1 := DevMasterKey()
	mk2 := DevMasterKey()

	original := "test-secret-value-12345"
	enc, nonce, err := Encrypt(mk1, original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(mk2, enc, nonce)
	if err == nil {
		t.Fatal("Decrypt with wrong key should fail")
	}
	t.Logf("Wrong key correctly rejected: %v", err)
}

func TestNonceUniqueness(t *testing.T) {
	mk := DevMasterKey()
	nonces := make(map[string]bool)

	for i := 0; i < 100; i++ {
		_, nonce, err := Encrypt(mk, "test")
		if err != nil {
			t.Fatalf("Encrypt failed at iteration %d: %v", i, err)
		}
		if nonces[nonce] {
			t.Fatalf("duplicate nonce at iteration %d: %s", i, nonce)
		}
		nonces[nonce] = true
	}
	t.Logf("100 nonces generated, all unique: OK")
}

func TestEmptyPlaintextFails(t *testing.T) {
	mk := DevMasterKey()
	_, _, err := Encrypt(mk, "")
	if err == nil {
		t.Fatal("Encrypt with empty plaintext should fail")
	}
	t.Logf("Empty plaintext rejected: %v", err)
}

func TestNilKeyFails(t *testing.T) {
	_, _, err := Encrypt(nil, "test")
	if err == nil {
		t.Error("Encrypt with nil key should fail")
	}
	_, err = Decrypt(nil, "enc", "nonce")
	if err == nil {
		t.Error("Decrypt with nil key should fail")
	}
}

func TestEncryptDecryptFromStorage(t *testing.T) {
	mk := DevMasterKey()
	original := "my-secret-value"

	storage, err := EncryptToStorage(mk, original)
	if err != nil {
		t.Fatalf("EncryptToStorage failed: %v", err)
	}

	// Verify format: nonce:encrypted
	parts := strings.SplitN(storage, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("expected format nonce:encrypted, got %q", storage)
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		t.Fatalf("nonce or encrypted part is empty")
	}

	decrypted, err := DecryptFromStorage(mk, storage)
	if err != nil {
		t.Fatalf("DecryptFromStorage failed: %v", err)
	}
	if decrypted != original {
		t.Fatalf("roundtrip mismatch: got %q, want %q", decrypted, original)
	}
	t.Logf("EncryptToStorage/DecryptFromStorage roundtrip: OK")
}

func TestEmptyStorageFails(t *testing.T) {
	mk := DevMasterKey()

	_, err := DecryptFromStorage(mk, "")
	if err == nil {
		t.Error("DecryptFromStorage with empty storage should fail")
	}

	_, err = DecryptFromStorage(mk, "only-once-part")
	if err == nil {
		t.Error("DecryptFromStorage without separator should fail")
	}
}

func TestKeyHashDeterministic(t *testing.T) {
	mk1 := DevMasterKey()
	mk2 := DevMasterKey()

	h1 := mk1.KeyHash()
	h2 := mk1.KeyHash()
	if h1 != h2 {
		t.Error("KeyHash should be deterministic for same key")
	}

	h3 := mk2.KeyHash()
	if h1 == h3 {
		t.Log("Note: two randomly generated keys happened to collide — extremely unlikely")
	}
	t.Logf("KeyHash: %s", h1)
}

func TestLoadMasterKeyFromEnv(t *testing.T) {
	keyStr, err := GenerateKeyString()
	if err != nil {
		t.Fatalf("GenerateKeyString failed: %v", err)
	}

	t.Setenv(EnvVar, keyStr)
	mk, err := LoadMasterKey(true)
	if err != nil {
		t.Fatalf("LoadMasterKey from env failed: %v", err)
	}
	if mk == nil {
		t.Fatal("master key is nil")
	}
	if !strings.Contains(mk.Source(), EnvVar) {
		t.Errorf("expected source to mention env var, got %s", mk.Source())
	}
	t.Logf("MasterKey loaded from env: source=%s", mk.Source())
}

func TestLoadMasterKeyDevMode(t *testing.T) {
	// Ensure env var is NOT set
	t.Setenv(EnvVar, "")
	mk, err := LoadMasterKey(true)
	if err != nil {
		t.Fatalf("LoadMasterKey dev mode failed: %v", err)
	}
	if mk == nil {
		t.Fatal("master key is nil")
	}
	t.Logf("MasterKey created in dev mode: source=%s", mk.Source())
}

func TestLoadMasterKeyNoSource(t *testing.T) {
	t.Setenv(EnvVar, "")
	// Skip file check by ensuring env only path

	_, err := LoadMasterKey(false)
	if err == nil {
		t.Fatal("LoadMasterKey should fail without any source")
	}
	t.Logf("No key source correctly reported: %v", err)
}

func TestInvalidHexKey(t *testing.T) {
	t.Setenv(EnvVar, "not-a-hex-key-short")
	_, err := LoadMasterKey(false)
	if err == nil {
		t.Error("LoadMasterKey should fail with invalid key")
	}
	t.Logf("Invalid key rejected: %v", err)
}

func TestTamperedCiphertext(t *testing.T) {
	mk := DevMasterKey()

	enc, nonce, err := Encrypt(mk, "test-value")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with ciphertext
	tampered := "A" + enc[1:]
	_, err = Decrypt(mk, tampered, nonce)
	if err == nil {
		t.Fatal("Decrypt with tampered ciphertext should fail")
	}
	t.Logf("Tampered ciphertext correctly rejected: %v", err)
}

func TestTamperedNonce(t *testing.T) {
	mk := DevMasterKey()

	enc, nonce, err := Encrypt(mk, "test-value")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	tamperedNonce := "B" + nonce[1:]
	_, err = Decrypt(mk, enc, tamperedNonce)
	if err == nil {
		t.Fatal("Decrypt with tampered nonce should fail")
	}
	t.Logf("Tampered nonce correctly rejected: %v", err)
}

func TestGenerateKeyString(t *testing.T) {
	ks, err := GenerateKeyString()
	if err != nil {
		t.Fatalf("GenerateKeyString failed: %v", err)
	}
	if len(ks) != KeySize*2 {
		t.Errorf("expected %d hex chars, got %d", KeySize*2, len(ks))
	}

	ks2, _ := GenerateKeyString()
	if ks == ks2 {
		t.Error("consecutive keys should be different")
	}
	t.Logf("Generated key string: length=%d", len(ks))
}
