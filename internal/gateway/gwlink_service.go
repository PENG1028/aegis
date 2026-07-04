package gateway

import (
	"fmt"
	"time"

	"aegis/internal/core"
	"aegis/internal/secrets"
)

// LinkService manages trusted gateway links and auth.
type LinkService struct {
	repo     *LinkRepository
	selfID   string // this gateway's ID for auth header generation
	selfName string // this gateway's name
	mk       *secrets.MasterKey // v1.8B-5: master key for secret-at-rest encryption
}

// NewLinkService creates a gateway link service.
// If mk is nil, the service operates in legacy mode (HMAC hash only).
func NewLinkService(repo *LinkRepository, selfID, selfName string, mk *secrets.MasterKey) *LinkService {
	return &LinkService{
		repo:     repo,
		selfID:   selfID,
		selfName: selfName,
		mk:       mk,
	}
}

// Register adds a new trusted gateway and returns its generated secret.
// v1.8B-5: Uses encrypted storage when master key is available.
func (s *LinkService) Register(name, host, privateIP string, port int, gatewayType string, autoRoute bool) (*TrustedGateway, string, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, "", fmt.Errorf("generate secret: %w", err)
	}

	var gw *TrustedGateway
	if s.mk != nil {
		// v1.8B-5: Use encrypted storage
		gw, err = NewEncryptedGateway(name, host, privateIP, port, secret, gatewayType, autoRoute, s.mk)
		if err != nil {
			return nil, "", fmt.Errorf("create encrypted gateway: %w", err)
		}
	} else {
		// Legacy: HMAC hash only
		gw = NewTrustedGateway(name, host, privateIP, port, secret, gatewayType, autoRoute)
	}
	gw.ID = core.NewID("gw")

	if err := s.repo.Create(gw); err != nil {
		return nil, "", fmt.Errorf("create gateway: %w", err)
	}

	// Return the raw secret once — caller must store it securely
	return gw, secret, nil
}

// List returns all registered gateways (secrets not included).
func (s *LinkService) List() ([]TrustedGateway, error) {
	return s.repo.FindAll()
}

// Get returns a gateway by ID (with auth fields for verification).
// The returned gateway does NOT contain the raw token.
// Use GetDecryptedSecret() separately if raw token is needed.
func (s *LinkService) Get(id string) (*TrustedGateway, error) {
	return s.repo.FindByID(id)
}

// GetDownstreamGateways returns gateways this gateway forwards traffic TO.
// NOTE: Despite the method name, it queries TypeUpstream because the database
// labels gateways by their relationship to the current node:
//
//	- "upstream" gateways = servers this gateway sends traffic TO (downstream direction)
//	- "downstream" gateways = servers that send traffic TO this gateway (upstream direction)
//
// This naming inversion is intentional for database consistency.
// Do NOT "fix" this by changing to TypeDownstream without also updating the DB schema docs.
func (s *LinkService) GetDownstreamGateways() ([]TrustedGateway, error) {
	return s.repo.FindByType(TypeUpstream)
}

// Remove deletes a trusted gateway.
func (s *LinkService) Remove(id string) error {
	return s.repo.Delete(id)
}

// RotateSecret generates a new secret for a gateway.
// v1.8B-5: Uses encrypted storage when master key is available.
func (s *LinkService) RotateSecret(id string) (string, error) {
	gw, err := s.repo.FindByID(id)
	if err != nil {
		return "", err
	}
	if gw == nil {
		return "", fmt.Errorf("gateway %s not found", id)
	}

	secret, err := generateSecret()
	if err != nil {
		return "", err
	}

	if gw.HasEncryptedSecret() {
		if s.mk == nil {
			return "", fmt.Errorf("cannot rotate: encrypted secret exists but master key is nil")
		}
		// v1.8B-5: Encrypted rotation
		if err := gw.RotateSecretEncrypted(secret, s.mk); err != nil {
			return "", fmt.Errorf("rotate encrypted secret: %w", err)
		}
		if err := s.repo.RotateSecretEncrypted(id, gw.EncryptedSecret, gw.SecretNonce,
			gw.SecretVersion, gw.SecretRotatedAt); err != nil {
			return "", err
		}
	} else {
		// Legacy HMAC rotation
		hashed := hashSecret(secret)
		if err := s.repo.RotateSecret(id, hashed); err != nil {
			return "", err
		}
	}

	return secret, nil
}

// GetAuthHeader generates the auth header for forwarding to a downstream gateway.
// v1.8B-5: Decrypts the raw secret when encrypted storage is used.
func (s *LinkService) GetAuthHeader(gatewayID string) (string, error) {
	gw, err := s.repo.FindByID(gatewayID)
	if err != nil {
		return "", err
	}
	if gw == nil {
		return "", fmt.Errorf("gateway %s not found", gatewayID)
	}

	secret, err := gw.GetRawSecret(s.mk)
	if err != nil {
		return "", fmt.Errorf("get secret: %w", err)
	}
	if secret == "" {
		return "", nil
	}
	return GenerateAuthHeader(s.selfID, secret), nil
}

// GetDecryptedSecret decrypts and returns the raw secret for a gateway (v1.8B-5).
// This is used by the apply/planner to inject auth headers into rendered config.
// Returns empty string if no secret is available.
func (s *LinkService) GetDecryptedSecret(gatewayID string) (string, error) {
	gw, err := s.repo.FindByID(gatewayID)
	if err != nil {
		return "", err
	}
	if gw == nil {
		return "", fmt.Errorf("gateway %s not found", gatewayID)
	}
	return gw.GetRawSecret(s.mk)
}

// VerifyRequest checks if an incoming request is from a trusted upstream.
func (s *LinkService) VerifyRequest(authHeader string) bool {
	if authHeader == "" {
		return false
	}

	gateways, err := s.repo.FindByType(TypeDownstream)
	if err != nil || len(gateways) == 0 {
		return false
	}

	// Check against all downstream gateways
	for _, gw := range gateways {
		if gw.HasEncryptedSecret() {
			if s.mk == nil {
				// Encrypted data exists but no master key — fail closed
				continue
			}
			raw, err := gw.GetRawSecret(s.mk)
			if err != nil {
				continue
			}
			if VerifyAuthHeader(authHeader, s.selfID, raw) {
				return true
			}
		} else if gw.AuthValue != "" {
			// Legacy HMAC fallback (degraded mode allowed)
			if VerifyAuthHeader(authHeader, s.selfID, gw.AuthValue) {
				return true
			}
		}
	}
	return false
}

// BackfillEncrypted converts a legacy HMAC-hashed gateway to encrypted storage.
// This is a one-way operation: after backfill, the raw secret is still unrecoverable
// from the HMAC hash, but new rotations will use encryption.
// Returns true if the gateway was backfilled, false if already encrypted.
func (s *LinkService) BackfillEncrypted(id string) (bool, error) {
	if s.mk == nil {
		return false, fmt.Errorf("master key not available — cannot backfill")
	}

	gw, err := s.repo.FindByID(id)
	if err != nil {
		return false, err
	}
	if gw == nil {
		return false, fmt.Errorf("gateway %s not found", id)
	}
	if gw.HasEncryptedSecret() {
		return false, nil // already encrypted
	}

	// Legacy HMAC hash exists but no encrypted data and no raw secret available.
	// We cannot recover the raw secret from an HMAC hash.
	// The solution: generate a new secret, encrypt it, and store it.
	// The old HMAC hash is kept as fallback for existing connections.
	secret, err := generateSecret()
	if err != nil {
		return false, fmt.Errorf("generate secret: %w", err)
	}

	encryptedB64, nonceB64, err := secrets.Encrypt(s.mk, secret)
	if err != nil {
		return false, fmt.Errorf("encrypt: %w", err)
	}

	now := time.Now()
	timeStr := now.Format(time.RFC3339)
	gw.EncryptedSecret = encryptedB64
	gw.SecretNonce = nonceB64
	gw.SecretVersion = 1
	gw.SecretCreatedAt = timeStr
	gw.AuthValue = hashSecret(secret) // update HMAC hash to match new secret

	if err := s.repo.RotateSecretEncrypted(id, encryptedB64, nonceB64, 1, timeStr); err != nil {
		return false, fmt.Errorf("store encrypted: %w", err)
	}
	// Also update the HMAC hash to match
	if err := s.repo.RotateSecret(id, gw.AuthValue); err != nil {
		return false, fmt.Errorf("update hm ac: %w", err)
	}

	return true, nil
}

// generateSecret creates a cryptographically random 32-byte hex secret.
// Delegates to core.GenerateRandomHex — the project's canonical random-hex generator.
func generateSecret() (string, error) {
	return core.GenerateRandomHex(32), nil
}
