package nodeauth

import (
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *Repository {
	// We use a real SQLite via modernc.org/sqlite through the store package.
	// For unit tests, use the existing store.Initialize with :memory:.
	// However, this test file tests the repository logic directly.
	// Since we can't import store without circular deps in some cases,
	// we'll use an approach that creates the tables directly.

	// Use the test helper from store package — tests are in an integration test.
	// For pure unit tests, we test the model/service logic separately.
	t.Helper()
	return nil // placeholder: real integration test in nodeauth_test.go
}

func TestJoinTokenIsValid(t *testing.T) {
	now := time.Now()

	// Valid token
	tok := &JoinToken{
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if !tok.IsValid() {
		t.Error("expected valid join token")
	}

	// Expired token
	tok2 := &JoinToken{
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	if tok2.IsValid() {
		t.Error("expected expired token to be invalid")
	}

	// Used token
	tok3 := &JoinToken{
		ExpiresAt: now.Add(1 * time.Hour),
		UsedAt:    now,
	}
	if tok3.IsValid() {
		t.Error("expected used token to be invalid")
	}

	// Revoked token
	tok4 := &JoinToken{
		ExpiresAt: now.Add(1 * time.Hour),
		RevokedAt: now,
	}
	if tok4.IsValid() {
		t.Error("expected revoked token to be invalid")
	}

	// Used AND revoked (used takes precedence)
	tok5 := &JoinToken{
		ExpiresAt: now.Add(1 * time.Hour),
		UsedAt:    now,
		RevokedAt: now,
	}
	if tok5.IsValid() {
		t.Error("expected used+revoked token to be invalid")
	}
}

func TestNodeCredentialIsRevoked(t *testing.T) {
	cred := &NodeCredential{}
	if cred.IsRevoked() {
		t.Error("expected fresh credential to not be revoked")
	}

	cred.RevokedAt = time.Now()
	if !cred.IsRevoked() {
		t.Error("expected revoked credential to be revoked")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	token := "test-token-value-12345"
	h1 := hashToken(token)
	h2 := hashToken(token)
	if h1 != h2 {
		t.Error("expected deterministic hash")
	}
	if h1 == "" {
		t.Error("expected non-empty hash")
	}
}

func TestHashTokenDifferent(t *testing.T) {
	h1 := hashToken("token-a")
	h2 := hashToken("token-b")
	if h1 == h2 {
		t.Error("expected different tokens to produce different hashes")
	}
}
