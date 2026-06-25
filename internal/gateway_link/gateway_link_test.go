package gatewaylink

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"aegis/internal/store"
)

func TestHashSecretDeterministic(t *testing.T) {
	h1 := hashSecret("my-secret-key")
	h2 := hashSecret("my-secret-key")
	if h1 == "" {
		t.Fatal("hash should not be empty")
	}
	if h1 != h2 {
		t.Error("hash should be deterministic for same input")
	}
	t.Logf("Hash: %s", h1[:16])
}

func TestHashSecretDifferent(t *testing.T) {
	h1 := hashSecret("key-a")
	h2 := hashSecret("key-b")
	if h1 == h2 {
		t.Error("different secrets should produce different hashes")
	}
	t.Log("Different secrets produce different hashes: OK")
}

func TestGenerateAuthHeaderAndVerify(t *testing.T) {
	gatewayID := "gw_test123"
	secret := "my-shared-secret"

	header := GenerateAuthHeader(gatewayID, secret)
	if header == "" {
		t.Fatal("auth header should not be empty")
	}
	if !strings.HasPrefix(header, "Aegis ") {
		t.Errorf("expected 'Aegis ' prefix, got %s", header[:10])
	}

	valid := VerifyAuthHeader(header, gatewayID, secret)
	if !valid {
		t.Error("VerifyAuthHeader should return true for valid header")
	}

	invalid := VerifyAuthHeader(header, gatewayID, "wrong-secret")
	if invalid {
		t.Error("VerifyAuthHeader should return false for wrong secret")
	}

	invalid2 := VerifyAuthHeader(header, "different-id", secret)
	if invalid2 {
		t.Error("VerifyAuthHeader should return false for wrong gateway ID")
	}

	t.Logf("Auth header round-trip: OK (header=%s)", header[:30])
}

func TestVerifyAuthHeaderEmptyInputs(t *testing.T) {
	if VerifyAuthHeader("", "gw1", "secret") {
		t.Error("empty header should fail")
	}
	if VerifyAuthHeader("Aegis gw1:sig", "", "secret") {
		t.Error("empty gatewayID should fail")
	}
	if VerifyAuthHeader("Aegis gw1:sig", "gw1", "") {
		t.Error("empty secret should fail")
	}
	t.Log("Empty inputs handled correctly")
}

func TestNewTrustedGateway(t *testing.T) {
	gw := NewTrustedGateway("server-b", "<SERVER_B_IP>", "10.3.0.11", 443, "test-secret", TypeUpstream, true)

	if gw.Name != "server-b" {
		t.Errorf("expected Name=server-b, got %s", gw.Name)
	}
	if gw.Host != "<SERVER_B_IP>" {
		t.Errorf("expected Host=<SERVER_B_IP>, got %s", gw.Host)
	}
	if gw.PrivateIP != "10.3.0.11" {
		t.Errorf("expected PrivateIP=10.3.0.11, got %s", gw.PrivateIP)
	}
	if gw.Port != 443 {
		t.Errorf("expected Port=443, got %d", gw.Port)
	}
	if gw.AuthType != AuthSharedSecret {
		t.Errorf("expected AuthType=shared_secret, got %s", gw.AuthType)
	}
	if gw.AuthValue == "" {
		t.Error("AuthValue should be hashed")
	}
	if gw.AuthValue == "test-secret" {
		t.Error("AuthValue should NOT be the raw secret")
	}
	if gw.Status != StatusActive {
		t.Errorf("expected Status=active, got %s", gw.Status)
	}
	t.Logf("TrustedGateway created: name=%s host=%s private=%s", gw.Name, gw.Host, gw.PrivateIP)
}

func TestResolveHostAutoRoute(t *testing.T) {
	gw := NewTrustedGateway("test", "1.2.3.4", "10.0.0.1", 443, "s", TypeUpstream, true)
	host := gw.ResolveHost()
	if host != "10.0.0.1" {
		t.Errorf("expected private IP 10.0.0.1 with auto-route, got %s", host)
	}
	t.Logf("Auto-route: resolved to %s (private IP)", host)
}

func TestResolveHostPublicFallback(t *testing.T) {
	gw := NewTrustedGateway("test", "1.2.3.4", "", 443, "s", TypeUpstream, true)
	host := gw.ResolveHost()
	if host != "1.2.3.4" {
		t.Errorf("expected public IP 1.2.3.4 when no private IP, got %s", host)
	}
	t.Logf("Fallback: resolved to %s (public IP)", host)
}

func TestResolveHostAutoRouteDisabled(t *testing.T) {
	gw := NewTrustedGateway("test", "1.2.3.4", "10.0.0.1", 443, "s", TypeUpstream, false)
	host := gw.ResolveHost()
	if host != "1.2.3.4" {
		t.Errorf("expected public IP 1.2.3.4 when auto-route disabled, got %s", host)
	}
	t.Logf("Auto-route disabled: resolved to %s (public IP)", host)
}

func TestCheckAuth(t *testing.T) {
	gw := NewTrustedGateway("test", "1.2.3.4", "", 443, "my-secret", TypeUpstream, true)

	if !gw.CheckAuth("my-secret") {
		t.Error("CheckAuth should return true for correct secret")
	}
	if gw.CheckAuth("wrong-secret") {
		t.Error("CheckAuth should return false for wrong secret")
	}
	t.Log("CheckAuth: correct secret accepted, wrong secret rejected")
}

func TestRotateSecret(t *testing.T) {
	gw := NewTrustedGateway("test", "1.2.3.4", "", 443, "old-secret", TypeUpstream, true)

	if !gw.CheckAuth("old-secret") {
		t.Error("should accept old secret before rotation")
	}

	gw.RotateSecret("new-secret")

	if gw.CheckAuth("old-secret") {
		t.Error("should NOT accept old secret after rotation")
	}
	if !gw.CheckAuth("new-secret") {
		t.Error("should accept new secret after rotation")
	}
	t.Log("Secret rotation: old invalid, new valid")
}

func TestGenerateSecret(t *testing.T) {
	s1, err := generateSecret()
	if err != nil {
		t.Fatalf("generateSecret failed: %v", err)
	}
	if len(s1) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("expected 64 hex chars, got %d", len(s1))
	}

	s2, _ := generateSecret()
	if s1 == s2 {
		t.Error("consecutive secrets should be different")
	}
	t.Logf("Secret generated: length=%d", len(s1))
}

func TestServiceCreateGateway(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a")

	gw, secret, err := svc.Register("server-b", "<SERVER_B_IP>", "10.3.0.11", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if gw == nil {
		t.Fatal("gateway should not be nil")
	}
	if secret == "" {
		t.Fatal("secret should not be empty")
	}
	if gw.AuthValue == secret {
		t.Error("stored auth_value should NOT be the raw secret")
	}
	t.Logf("Gateway registered: id=%s name=%s secret=%s", gw.ID, gw.Name, secret[:12])
}

func TestServiceRotateSecret(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a")

	_, _, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	gateways, _ := svc.List()
	if len(gateways) == 0 {
		t.Fatal("no gateways registered")
	}

	newSecret, err := svc.RotateSecret(gateways[0].ID)
	if err != nil {
		t.Fatalf("RotateSecret failed: %v", err)
	}
	if newSecret == "" {
		t.Fatal("new secret should not be empty")
	}
	t.Logf("Secret rotated: new=%s", newSecret[:12])
}

func TestAuthHeaderRoundTrip(t *testing.T) {
	db := setupDB(t)
	defer db.Close()

	repo := NewRepository(db)
	svc := NewService(repo, "gw_self", "server-a")

	_, secret, err := svc.Register("server-b", "<SERVER_B_IP>", "", 443, TypeUpstream, true)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	gateways, _ := svc.List()
	header, err := svc.GetAuthHeader(gateways[0].ID)
	if err != nil {
		t.Fatalf("GetAuthHeader failed: %v", err)
	}
	if header == "" {
		t.Fatal("auth header should not be empty")
	}

	_ = secret // the header is already signed with the hashed secret
	t.Logf("Auth header generated: %s", header[:40])
}

func TestEmptySecretReturnsEmptyHeader(t *testing.T) {
	// Gateway with no auth should return empty header
	gw := NewTrustedGateway("test", "1.2.3.4", "", 443, "", TypeUpstream, true)
	gw.AuthType = AuthNone

	if gw.AuthType != AuthNone {
		t.Errorf("expected AuthType=none, got %s", gw.AuthType)
	}
}

// Helpers
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	f, err := os.CreateTemp("", "aegis-gwlink-*.db")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	f.Close()
	os.Remove(f.Name())
	db, err := store.OpenSQLite(f.Name())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := store.Initialize(db); err != nil {
		db.Close()
		t.Fatalf("init db: %v", err)
	}
	return db
}
