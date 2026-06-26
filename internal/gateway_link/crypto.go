package gatewaylink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// hashSecret creates an HMAC-SHA256 hash of the secret.
// Uses a fixed key for deterministic hashing (not for encryption — just storage).
func hashSecret(secret string) string {
	if secret == "" {
		return ""
	}
	// Use a domain-separated HMAC key for gateway link auth
	h := hmac.New(sha256.New, []byte("aegis-gateway-link-v1"))
	h.Write([]byte(secret))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateAuthHeader creates the auth header value for gateway-to-gateway requests.
// Format: "Aegis <gateway_id>:<timestamp>:<signature>"
// Where signature = HMAC-SHA256(secret, gateway_id + ":" + timestamp)
func GenerateAuthHeader(gatewayID, secret string) string {
	if secret == "" {
		return ""
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gatewayID + ":" + ts))
	sig := hex.EncodeToString(mac.Sum(nil))
	return "Aegis " + gatewayID + ":" + ts + ":" + sig
}

// DefaultMaxAge is the default maximum age for a gateway link auth header.
const DefaultMaxAge = 5 * time.Minute

// VerifyAuthHeader validates an auth header against the expected secret.
// Uses DefaultMaxAge for replay protection.
func VerifyAuthHeader(header, gatewayID, secret string) bool {
	return VerifyAuthHeaderWithExpiry(header, gatewayID, secret, DefaultMaxAge)
}

// VerifyAuthHeaderWithExpiry validates an auth header with a custom maxAge.
// Rejects headers older than maxAge to prevent replay attacks.
// Allows 30 seconds of clock skew grace.
func VerifyAuthHeaderWithExpiry(header, gatewayID, secret string, maxAge time.Duration) bool {
	if header == "" || secret == "" || gatewayID == "" {
		return false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || parts[0] != "Aegis" {
		return false
	}
	payload := strings.Split(parts[1], ":")
	if len(payload) != 3 {
		// Fall back to legacy format: "Aegis <gateway_id>:<signature>" (no timestamp)
		return verifyLegacyHeader(header, gatewayID, secret)
	}
	headerGWID := payload[0]
	tsStr := payload[1]
	sig := payload[2]

	if headerGWID != gatewayID {
		return false
	}

	// Parse and validate timestamp
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return false
	}
	headerTime := time.Unix(ts, 0)
	now := time.Now()

	// Reject if too old (with 30s clock skew grace)
	if now.Sub(headerTime) > maxAge+30*time.Second {
		return false
	}
	// Reject if from the future (more than 30s ahead)
	if headerTime.Sub(now) > 30*time.Second {
		return false
	}

	// Verify HMAC
	expected := generateHeaderSignature(gatewayID, tsStr, secret)
	return hmac.Equal([]byte(sig), []byte(expected))
}

// generateHeaderSignature creates the HMAC signature for a gateway link auth header.
func generateHeaderSignature(gatewayID, timestamp, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gatewayID + ":" + timestamp))
	return hex.EncodeToString(mac.Sum(nil))
}

// verifyLegacyHeader checks the old format: "Aegis <gateway_id>:<sig>" without timestamp.
// Only used for backward compatibility during transition.
func verifyLegacyHeader(header, gatewayID, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gatewayID))
	oldSig := hex.EncodeToString(mac.Sum(nil))
	expected := "Aegis " + gatewayID + ":" + oldSig
	return hmac.Equal([]byte(header), []byte(expected))
}
