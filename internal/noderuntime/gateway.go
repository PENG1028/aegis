package noderuntime

// GatewayStatusProvider provides local gateway status for heartbeat reporting.
type GatewayStatusProvider interface {
	// LocalGatewayStatus returns the current status of the local HTTP gateway.
	LocalGatewayStatus() *LocalGatewayInfo
}

// LocalGatewayInfo describes the local gateway for heartbeat payloads.
type LocalGatewayInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Provider  string `json:"provider"`
	BindAddr  string `json:"bind_addr"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Scheme    string `json:"scheme"`
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	LastError string `json:"last_error,omitempty"`
}

// APISecretProvider fetches GatewayLink tokens from the control plane API.
// Tokens are fetched on-demand and returned in-memory only.
// This is the production path: the control plane decrypts the encrypted
// GatewayLink secret using the MasterKey and returns the raw token.
type APISecretProvider struct {
	client *Client
}

// NewAPISecretProvider creates a new API-based secret provider.
func NewAPISecretProvider(client *Client) *APISecretProvider {
	return &APISecretProvider{client: client}
}

// GetGatewayLinkToken fetches a token from the control plane API.
// The token is returned in plaintext (only in memory, never on disk).
func (p *APISecretProvider) GetGatewayLinkToken(gatewayLinkID string) (string, error) {
	return p.client.GetGatewayLinkToken(gatewayLinkID)
}

// SyncGatewayLinkSecrets iterates the routing table, fetches tokens for all
// unique gateway_link_ids, and populates a map for use in relay requests.
// This is called during the reconcile cycle to pre-warm the secret cache.
// All tokens exist only in memory - never written to disk cache.
func SyncGatewayLinkSecrets(client *Client, entries []RoutingTableEntry) map[string]string {
	seen := make(map[string]bool)
	secrets := make(map[string]string)
	for _, entry := range entries {
		for _, c := range entry.Candidates {
			if c.GatewayLinkID == "" || seen[c.GatewayLinkID] {
				continue
			}
			seen[c.GatewayLinkID] = true
			token, err := client.GetGatewayLinkToken(c.GatewayLinkID)
			if err != nil {
				// Log but continue - token fetch failure means relay won't work for this link
				continue
			}
			if token != "" {
				secrets[c.GatewayLinkID] = token
			}
		}
	}
	return secrets
}
