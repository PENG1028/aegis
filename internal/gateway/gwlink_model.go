// GatewayLink model — trusted gateway-to-gateway authentication.
// Each TrustedGateway represents another Aegis gateway that this gateway
// can securely forward traffic to. Traffic is authenticated via shared secret.
package gateway

import (
	"fmt"
	"time"

	"aegis/internal/secrets"
)

// AuthType constants.
const (
	AuthSharedSecret = "shared_secret"
	AuthNone         = "none"
)

// GatewayType constants.
const (
	TypeUpstream   = "upstream"   // this gateway forwards to it
	TypeDownstream = "downstream" // receives traffic from upstream
	TypePeer       = "peer"       // mutual forwarding
)

// Status constants.
const (
	LinkStatusActive   = "active"
	LinkStatusDisabled = "disabled"
)

// TrustedGateway represents another gateway node that this gateway trusts.
type TrustedGateway struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Host          string    `json:"host"`                 // public IP or hostname
	PrivateIP     string    `json:"private_ip,omitempty"` // private IP for auto-routing
	Port          int       `json:"port"`                 // target port (usually 443)
	AuthType      string    `json:"auth_type"`            // shared_secret | none
	AuthValue     string    `json:"-"`                    // legacy HMAC-hashed secret (v1.7AB)
	GatewayType   string    `json:"gateway_type"`         // upstream | downstream | peer
	AutoRoute     bool      `json:"auto_route"`           // prefer private IP, fallback to public
	Status        string    `json:"status"`
	TargetNodeID  string    `json:"target_node_id,omitempty"` // v1.8B — which Aegis node this gateway links to

	// v1.8B-5: Encrypted secret at rest (replaces HMAC hash for new links)
	EncryptedSecret string `json:"-"` // base64 AES-256-GCM ciphertext
	SecretNonce     string `json:"-"` // base64 nonce for decryption
	SecretVersion   int    `json:"secret_version,omitempty"`   // incremented on rotate
	SecretCreatedAt string `json:"secret_created_at,omitempty"` // RFC3339
	SecretRotatedAt string `json:"secret_rotated_at,omitempty"` // RFC3339

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NewTrustedGateway creates a new trusted gateway with hashed auth (legacy HMAC).
// Used for backward compatibility. New code should use NewEncryptedGateway.
func NewTrustedGateway(name, host, privateIP string, port int, authSecret, gatewayType string, autoRoute bool) *TrustedGateway {
	now := time.Now()
	return &TrustedGateway{
		Name:        name,
		Host:        host,
		PrivateIP:   privateIP,
		Port:        port,
		AuthType:    AuthSharedSecret,
		AuthValue:   hashSecret(authSecret),
		GatewayType: gatewayType,
		AutoRoute:   autoRoute,
		Status:      LinkStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewEncryptedGateway creates a new trusted gateway with encrypted secret (v1.8B-5).
// The mk key is used to encrypt the rawToken before storage.
func NewEncryptedGateway(name, host, privateIP string, port int, rawToken, gatewayType string, autoRoute bool, mk *secrets.MasterKey) (*TrustedGateway, error) {
	now := time.Now()
	encryptedB64, nonceB64, err := secrets.Encrypt(mk, rawToken)
	if err != nil {
		return nil, err
	}

	timeStr := now.Format(time.RFC3339)
	return &TrustedGateway{
		Name:            name,
		Host:            host,
		PrivateIP:       privateIP,
		Port:            port,
		AuthType:        AuthSharedSecret,
		AuthValue:       hashSecret(rawToken), // keep HMAC hash as fallback for backward compat
		GatewayType:     gatewayType,
		AutoRoute:       autoRoute,
		Status:          LinkStatusActive,
		EncryptedSecret: encryptedB64,
		SecretNonce:     nonceB64,
		SecretVersion:   1,
		SecretCreatedAt: timeStr,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// ResolveHost returns the best host to connect to based on auto-routing.
// If private IP is available and auto-route is enabled, prefers private IP.
func (g *TrustedGateway) ResolveHost() string {
	if g.AutoRoute && g.PrivateIP != "" {
		return g.PrivateIP
	}
	return g.Host
}

// CheckAuth verifies a provided secret against the stored auth value.
// This is the legacy HMAC hash comparison path.
func (g *TrustedGateway) CheckAuth(providedSecret string) bool {
	if g.AuthType != AuthSharedSecret || g.AuthValue == "" {
		return false
	}
	return g.AuthValue == hashSecret(providedSecret)
}

// CheckAuthEncrypted verifies a provided secret using encrypted storage (v1.8B-5).
// This is the primary auth check method for relay and cross-node auth.
//
// Fail-safe rules:
//   - Encrypted gateway + nil master key → FAIL CLOSED (auth denied).
//   - Encrypted gateway + wrong master key → auth denied.
//   - Legacy HMAC-only gateway → falls back to HMAC comparison (allowed in legacy mode).
func (g *TrustedGateway) CheckAuthEncrypted(providedSecret string, mk *secrets.MasterKey) bool {
	if g.HasEncryptedSecret() {
		if mk == nil {
			return false // fail closed: encrypted data exists but no key available
		}
		raw, err := secrets.Decrypt(mk, g.EncryptedSecret, g.SecretNonce)
		if err != nil {
			return false // key mismatch or data corrupt
		}
		return raw == providedSecret
	}
	// Legacy fallback: HMAC hash comparison (only for gateways without encrypted data)
	return g.CheckAuth(providedSecret)
}

// GetRawSecret decrypts and returns the raw secret token (v1.8B-5).
// Use this when you need the raw secret for generating auth headers or config.
//
// Fail-safe rules:
//   - Encrypted gateway + nil master key → returns error (fail closed).
//   - Legacy HMAC-only gateway + any key → returns HMAC hash (legacy fallback).
//   - No auth data at all → returns ("", nil).
func (g *TrustedGateway) GetRawSecret(mk *secrets.MasterKey) (string, error) {
	if g.HasEncryptedSecret() {
		if mk == nil {
			return "", fmt.Errorf("cannot decrypt: encrypted secret exists but master key is nil")
		}
		return secrets.Decrypt(mk, g.EncryptedSecret, g.SecretNonce)
	}
	// Legacy fallback: return the HMAC hash (not the original raw secret)
	if g.AuthValue != "" {
		return g.AuthValue, nil
	}
	return "", nil
}

// HasEncryptedSecret returns true if this gateway link uses encrypted storage.
func (g *TrustedGateway) HasEncryptedSecret() bool {
	return g.EncryptedSecret != "" && g.SecretNonce != ""
}

// HasSecret returns true if this gateway link has any usable auth data.
func (g *TrustedGateway) HasSecret() bool {
	return g.HasEncryptedSecret() || g.AuthValue != ""
}

// IsDegraded returns true if this gateway is operating in legacy mode
// (HMAC-only, no encrypted-at-rest protection).
// An encrypted gateway with a missing master key is NOT degraded — it simply fails.
func (g *TrustedGateway) IsDegraded() bool {
	return !g.HasEncryptedSecret() && g.AuthValue != ""
}
func (g *TrustedGateway) RotateSecret(newSecret string) {
	g.AuthValue = hashSecret(newSecret)
	g.UpdatedAt = time.Now()
}

// RotateSecretEncrypted rotates the secret using encrypted storage (v1.8B-5).
// Updates both the encrypted secret (primary) and HMAC hash (fallback).
func (g *TrustedGateway) RotateSecretEncrypted(newSecret string, mk *secrets.MasterKey) error {
	encryptedB64, nonceB64, err := secrets.Encrypt(mk, newSecret)
	if err != nil {
		return err
	}
	now := time.Now()
	g.EncryptedSecret = encryptedB64
	g.SecretNonce = nonceB64
	g.SecretVersion++
	g.AuthValue = hashSecret(newSecret) // keep HMAC fallback in sync
	g.SecretRotatedAt = now.Format(time.RFC3339)
	g.UpdatedAt = now
	return nil
}
