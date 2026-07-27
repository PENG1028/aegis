package serviceauth

import (
	"testing"
	"time"
)

func genKeyPair(t *testing.T) (pub, priv string) {
	t.Helper()
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair: %v", err)
	}
	return pub, priv
}

func TestSignAndVerify(t *testing.T) {
	pub, priv := genKeyPair(t)

	claims := NewTicket("svc-a")
	ticket := SignTicket(claims, priv)

	verified, err := VerifyTicket(ticket, pub)
	if err != nil {
		t.Fatalf("VerifyTicket failed: %v", err)
	}
	if verified.CallerService != "svc-a" {
		t.Errorf("CallerService = %q, want %q", verified.CallerService, "svc-a")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	_, priv1 := genKeyPair(t)
	pub2, _ := genKeyPair(t)

	claims := NewTicket("a")
	ticket := SignTicket(claims, priv1)

	_, err := VerifyTicket(ticket, pub2)
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}
}

func TestVerifyExpired(t *testing.T) {
	pub, priv := genKeyPair(t)

	claims := TicketClaims{
		CallerService: "a",
		ExpiresAt:     time.Now().Add(-1 * time.Hour).Unix(),
	}
	ticket := SignTicket(claims, priv)

	_, err := VerifyTicket(ticket, pub)
	if err == nil {
		t.Fatal("expected error for expired ticket, got nil")
	}
}

func TestVerifyTampered(t *testing.T) {
	pub, priv := genKeyPair(t)

	claims := NewTicket("a")
	ticket := SignTicket(claims, priv)

	tampered := ticket + "garbage"
	_, err := VerifyTicket(tampered, pub)
	if err == nil {
		t.Fatal("expected error for tampered ticket, got nil")
	}
}

func TestSignTicketRoundTrip(t *testing.T) {
	pub, priv := genKeyPair(t)

	tests := []struct {
		caller string
	}{
		{"admin-service"},
		{"svc-a"},
		{"user-svc"},
		{"short"},
	}

	for _, tt := range tests {
		claims := NewTicket(tt.caller)
		ticket := SignTicket(claims, priv)
		verified, err := VerifyTicket(ticket, pub)
		if err != nil {
			t.Errorf("round-trip %+v: %v", tt, err)
			continue
		}
		if verified.CallerService != tt.caller {
			t.Errorf("caller mismatch: got %q want %q", verified.CallerService, tt.caller)
		}
	}
}
