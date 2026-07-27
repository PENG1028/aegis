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

// TicketClaims is the decoded content of a service ticket.
type TicketClaims struct {
	CallerService string
	ExpiresAt     int64
}

// NewTicket creates a TicketClaims with ExpiresAt set to now + TicketValidity.
func NewTicket(callerService string) TicketClaims {
	return TicketClaims{
		CallerService: callerService,
		ExpiresAt:     time.Now().Add(TicketValidity).Unix(),
	}
}

// GenerateKeyPair creates a new Ed25519 key pair, returning base64-encoded strings.
func GenerateKeyPair() (pubKey, privKey string, err error) {
	pub, priv, err := ed25519GenerateKey()
	if err != nil {
		return "", "", err
	}
	return pub, priv, nil
}

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
// Format: base64(caller:expiry:base64_signature)
func SignTicket(claims TicketClaims, privateKeyB64 string) string {
	payload := fmt.Sprintf("%s:%d", claims.CallerService, claims.ExpiresAt)

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

	parts := strings.SplitN(string(decoded), ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: expected 3 parts, got %d", ErrTicketInvalid, len(parts))
	}

	claims := &TicketClaims{
		CallerService: parts[0],
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &claims.ExpiresAt); err != nil {
		return nil, fmt.Errorf("%w: unreadable expiry", ErrTicketInvalid)
	}

	payload := strings.Join(parts[:2], ":")
	sig, err := base64.StdEncoding.DecodeString(parts[2])
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
