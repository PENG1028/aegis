package serviceauth

import (
	"crypto/rand"
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	claims := NewTicket("svc-a", "svc-b", "createProject")
	ticket := SignTicket(claims, key)

	verified, err := VerifyTicket(ticket, key)
	if err != nil {
		t.Fatalf("VerifyTicket failed: %v", err)
	}
	if verified.CallerService != "svc-a" {
		t.Errorf("CallerService = %q, want %q", verified.CallerService, "svc-a")
	}
	if verified.TargetService != "svc-b" {
		t.Errorf("TargetService = %q, want %q", verified.TargetService, "svc-b")
	}
	if verified.TargetAPI != "createProject" {
		t.Errorf("TargetAPI = %q, want %q", verified.TargetAPI, "createProject")
	}
}

func TestVerifyWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	rand.Read(key1)
	rand.Read(key2)

	claims := NewTicket("a", "b", "c")
	ticket := SignTicket(claims, key1)

	_, err := VerifyTicket(ticket, key2)
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}
}

func TestVerifyExpired(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	claims := TicketClaims{
		CallerService: "a",
		TargetService: "b",
		TargetAPI:     "c",
		ExpiresAt:     time.Now().Add(-1 * time.Hour).Unix(), // expired
	}
	ticket := SignTicket(claims, key)

	_, err := VerifyTicket(ticket, key)
	if err == nil {
		t.Fatal("expected error for expired ticket, got nil")
	}
}

func TestVerifyTampered(t *testing.T) {
	key := make([]byte, 32)
	rand.Read(key)

	claims := NewTicket("a", "b", "c")
	ticket := SignTicket(claims, key)

	// Tamper by appending garbage.
	tampered := ticket + "garbage"
	_, err := VerifyTicket(tampered, key)
	if err == nil {
		t.Fatal("expected error for tampered ticket, got nil")
	}
}

func TestSignTicketRoundTrip(t *testing.T) {
	// Test with various service names and API names.
	key := make([]byte, 32)
	rand.Read(key)

	tests := []struct {
		caller string
		target string
		api    string
	}{
		{"admin-service", "project-service", "createProject"},
		{"svc-a", "svc-b", "health"},
		{"user-svc", "auth-svc", "validateToken"},
		{"short", "x", "y"},
	}

	for _, tt := range tests {
		claims := NewTicket(tt.caller, tt.target, tt.api)
		ticket := SignTicket(claims, key)
		verified, err := VerifyTicket(ticket, key)
		if err != nil {
			t.Errorf("round-trip %+v: %v", tt, err)
			continue
		}
		if verified.CallerService != tt.caller {
			t.Errorf("caller mismatch: got %q want %q", verified.CallerService, tt.caller)
		}
		if verified.TargetService != tt.target {
			t.Errorf("target mismatch: got %q want %q", verified.TargetService, tt.target)
		}
		if verified.TargetAPI != tt.api {
			t.Errorf("api mismatch: got %q want %q", verified.TargetAPI, tt.api)
		}
	}
}
