package serviceauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// TicketValidity is how long a ticket remains valid after issuance.
const TicketValidity = 5 * time.Minute

// ed25519GenerateKey creates a new Ed25519 key pair.
func ed25519GenerateKey() (pubKey, privKey string, err error) {
	pub, priv, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		return "", "", fmt.Errorf("generate ed25519 key: %w", genErr)
	}
	return base64.StdEncoding.EncodeToString(pub),
		base64.StdEncoding.EncodeToString(priv),
		nil
}

// SignTicket produces a self-contained ticket string using Ed25519.
// Format: base64(caller:target:api:expiry:base64_signature)
func SignTicket(claims TicketClaims, privateKeyB64 string) string {
	payload := fmt.Sprintf("%s:%s:%s:%d",
		claims.CallerService, claims.TargetService, claims.TargetAPI, claims.ExpiresAt)

	privBytes, _ := base64.StdEncoding.DecodeString(privateKeyB64)
	sig := ed25519.Sign(privBytes, []byte(payload))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	raw := payload + ":" + sigB64
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// VerifyTicket decodes and validates an Ed25519-signed ticket string.
// publicKeyB64 is the caller's Ed25519 public key (base64-encoded).
// This is a pure function — no network, no database.
func VerifyTicket(ticketStr, publicKeyB64 string) (*TicketClaims, error) {
	decoded, err := base64.StdEncoding.DecodeString(ticketStr)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed", ErrTicketInvalid)
	}

	parts := strings.SplitN(string(decoded), ":", 5)
	if len(parts) != 5 {
		return nil, fmt.Errorf("%w: expected 5 parts, got %d", ErrTicketInvalid, len(parts))
	}

	claims := &TicketClaims{
		CallerService: parts[0],
		TargetService: parts[1],
		TargetAPI:     parts[2],
	}
	if _, err := fmt.Sscanf(parts[3], "%d", &claims.ExpiresAt); err != nil {
		return nil, fmt.Errorf("%w: unreadable expiry", ErrTicketInvalid)
	}

	// Verify Ed25519 signature.
	payload := strings.Join(parts[:4], ":")
	sig, err := base64.StdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature encoding", ErrTicketInvalid)
	}

	pubKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid public key", ErrTicketInvalid)
	}

	if !ed25519.Verify(pubKey, []byte(payload), sig) {
		return nil, fmt.Errorf("%w: signature mismatch", ErrTicketInvalid)
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTicketExpired
	}

	return claims, nil
}

// NewTicket creates a TicketClaims with ExpiresAt set to now + TicketValidity.
func NewTicket(callerService, targetService, targetAPI string) TicketClaims {
	return TicketClaims{
		CallerService: callerService,
		TargetService: targetService,
		TargetAPI:     targetAPI,
		ExpiresAt:     time.Now().Add(TicketValidity).Unix(),
	}
}
