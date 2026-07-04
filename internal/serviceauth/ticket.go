package serviceauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// TicketValidity is how long a ticket remains valid after issuance.
const TicketValidity = 5 * time.Minute

// SignTicket produces a self-contained ticket string suitable for the
// X-Service-Ticket header. The ticket is base64-encoded and carries
// caller, target, API name, expiry, and an HMAC-SHA256 signature.
//
// Both the central server (with masterKey) and the SDK (with clusterSecret)
// call this function. The key material differs but the algorithm is identical.
func SignTicket(claims TicketClaims, key []byte) string {
	payload := fmt.Sprintf("%s:%s:%s:%d",
		claims.CallerService,
		claims.TargetService,
		claims.TargetAPI,
		claims.ExpiresAt,
	)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))

	// Format: base64(payload:sig)
	raw := payload + ":" + sig
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// VerifyTicket decodes and validates a ticket string.
// Returns the claims on success, or an error describing why verification failed.
//
// This is a pure function — no network access, no database lookups.
// Any process that holds the correct key can verify tickets locally.
func VerifyTicket(ticketStr string, key []byte) (*TicketClaims, error) {
	decoded, err := base64.StdEncoding.DecodeString(ticketStr)
	if err != nil {
		return nil, fmt.Errorf("%w: base64 decode failed", ErrTicketInvalid)
	}

	parts := strings.SplitN(string(decoded), ":", 5)
	if len(parts) != 5 {
		return nil, fmt.Errorf("%w: expected 5 colon-separated parts, got %d", ErrTicketInvalid, len(parts))
	}

	claims := &TicketClaims{
		CallerService: parts[0],
		TargetService: parts[1],
		TargetAPI:     parts[2],
	}

	if _, err := fmt.Sscanf(parts[3], "%d", &claims.ExpiresAt); err != nil {
		return nil, fmt.Errorf("%w: unreadable expiry", ErrTicketInvalid)
	}

	// Recompute signature over the first 4 parts.
	payload := strings.Join(parts[:4], ":")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[4]), []byte(expectedSig)) {
		return nil, fmt.Errorf("%w: signature mismatch", ErrTicketInvalid)
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTicketExpired
	}

	return claims, nil
}

// NewTicket creates a TicketClaims with ExpiresAt set to now + validity.
// Convenience helper so callers don't compute the expiry by hand.
func NewTicket(callerService, targetService, targetAPI string) TicketClaims {
	return TicketClaims{
		CallerService: callerService,
		TargetService: targetService,
		TargetAPI:     targetAPI,
		ExpiresAt:     time.Now().Add(TicketValidity).Unix(),
	}
}
