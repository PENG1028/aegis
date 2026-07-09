package noderuntime

import "fmt"

// GatewayLinkSecretProvider provides runtime access to GatewayLink secrets.
// Raw tokens exist only in memory and are never cached to disk.
type GatewayLinkSecretProvider interface {
	// GetGatewayLinkToken returns the raw token for a gateway link.
	// Returns error if the secret is not available or cannot be decrypted.
	GetGatewayLinkToken(gatewayLinkID string) (string, error)
}

// InMemorySecretProvider is a simple in-memory secret provider for testing.
type InMemorySecretProvider struct {
	secrets map[string]string
}

// NewInMemorySecretProvider creates a new in-memory secret provider.
func NewInMemorySecretProvider() *InMemorySecretProvider {
	return &InMemorySecretProvider{
		secrets: make(map[string]string),
	}
}

// AddSecret adds a secret for a gateway link.
func (p *InMemorySecretProvider) AddSecret(gatewayLinkID, token string) {
	p.secrets[gatewayLinkID] = token
}

// GetGatewayLinkToken returns the token for a gateway link.
func (p *InMemorySecretProvider) GetGatewayLinkToken(gatewayLinkID string) (string, error) {
	token, ok := p.secrets[gatewayLinkID]
	if !ok {
		return "", fmt.Errorf("gateway link secret not found: %s", gatewayLinkID)
	}
	return token, nil
}
