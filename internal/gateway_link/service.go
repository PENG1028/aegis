package gatewaylink

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"aegis/internal/id"
)

// Service manages trusted gateway links and auth.
type Service struct {
	repo     *Repository
	selfID   string // this gateway's ID for auth header generation
	selfName string // this gateway's name
}

// NewService creates a gateway link service.
func NewService(repo *Repository, selfID, selfName string) *Service {
	return &Service{
		repo:     repo,
		selfID:   selfID,
		selfName: selfName,
	}
}

// Register adds a new trusted gateway and returns its generated secret.
func (s *Service) Register(name, host, privateIP string, port int, gatewayType string, autoRoute bool) (*TrustedGateway, string, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, "", fmt.Errorf("generate secret: %w", err)
	}

	gw := NewTrustedGateway(name, host, privateIP, port, secret, gatewayType, autoRoute)
	gw.ID = id.New("gw")

	if err := s.repo.Create(gw); err != nil {
		return nil, "", fmt.Errorf("create gateway: %w", err)
	}

	// Return the raw secret once — caller must store it securely
	return gw, secret, nil
}

// List returns all registered gateways (secrets not included).
func (s *Service) List() ([]TrustedGateway, error) {
	return s.repo.FindAll()
}

// Get returns a gateway by ID (with auth_value for verification).
func (s *Service) Get(id string) (*TrustedGateway, error) {
	return s.repo.FindByID(id)
}

// GetDownstreamGateways returns gateways of type "downstream".
// These are the gateways this gateway forwards traffic TO.
func (s *Service) GetDownstreamGateways() ([]TrustedGateway, error) {
	return s.repo.FindByType(TypeUpstream)
}

// Remove deletes a trusted gateway.
func (s *Service) Remove(id string) error {
	return s.repo.Delete(id)
}

// RotateSecret generates a new secret for a gateway.
func (s *Service) RotateSecret(id string) (string, error) {
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

	hashed := hashSecret(secret)
	if err := s.repo.RotateSecret(id, hashed); err != nil {
		return "", err
	}

	return secret, nil
}

// GetAuthHeader generates the auth header for forwarding to a downstream gateway.
func (s *Service) GetAuthHeader(gatewayID string) (string, error) {
	gw, err := s.repo.FindByID(gatewayID)
	if err != nil {
		return "", err
	}
	if gw == nil {
		return "", fmt.Errorf("gateway %s not found", gatewayID)
	}
	if gw.AuthValue == "" {
		return "", nil
	}
	return GenerateAuthHeader(s.selfID, gw.AuthValue), nil
}

// VerifyRequest checks if an incoming request is from a trusted upstream.
func (s *Service) VerifyRequest(authHeader string) bool {
	if authHeader == "" {
		return false
	}

	gateways, err := s.repo.FindByType(TypeDownstream)
	if err != nil || len(gateways) == 0 {
		return false
	}

	// Check against all downstream gateways
	for _, gw := range gateways {
		if gw.AuthValue == "" {
			continue
		}
		if VerifyAuthHeader(authHeader, s.selfID, gw.AuthValue) {
			return true
		}
		// Also check old secret if this gateway was rekeyed
	}
	return false
}

// generateSecret creates a cryptographically random 32-byte hex secret.
func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
