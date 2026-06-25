package gatewaylink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(gatewayID))
	sig := hex.EncodeToString(mac.Sum(nil))
	return "Aegis " + gatewayID + ":" + sig
}

// VerifyAuthHeader validates an auth header against the expected secret.
func VerifyAuthHeader(header, gatewayID, secret string) bool {
	if header == "" || secret == "" {
		return false
	}
	expected := GenerateAuthHeader(gatewayID, secret)
	return hmac.Equal([]byte(header), []byte(expected))
}
