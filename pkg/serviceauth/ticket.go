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

// TicketClaims is the decoded content of a service ticket.
type TicketClaims struct {
	CallerService string
	TargetService string
	TargetAPI     string
	ExpiresAt     int64
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

// SignTicket produces a self-contained ticket string.
// Algorithm: base64(caller:target:api:expiry:hex_hmac_sha256)
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

	raw := payload + ":" + sig
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// VerifyTicket decodes and validates a ticket string.
// Returns the claims on success, or an error describing why verification failed.
// This is a pure local function — no network, no database.
func VerifyTicket(ticketStr string, key []byte) (*TicketClaims, error) {
	decoded, err := base64.StdEncoding.DecodeString(ticketStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ticket encoding")
	}

	parts := strings.SplitN(string(decoded), ":", 5)
	if len(parts) != 5 {
		return nil, fmt.Errorf("malformed ticket")
	}

	claims := &TicketClaims{
		CallerService: parts[0],
		TargetService: parts[1],
		TargetAPI:     parts[2],
	}

	if _, err := fmt.Sscanf(parts[3], "%d", &claims.ExpiresAt); err != nil {
		return nil, fmt.Errorf("unreadable expiry")
	}

	// Recompute signature.
	payload := strings.Join(parts[:4], ":")
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[4]), []byte(expectedSig)) {
		return nil, fmt.Errorf("signature mismatch")
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("ticket expired")
	}

	return claims, nil
}
