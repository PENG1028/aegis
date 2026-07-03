package noderuntime

import (
	"aegis/internal/provider"
)

// GatewayStatusProvider provides gateway status for heartbeat reporting.
// Each implementation returns a slice of gateway infos — one per discovered/running
// gateway on this node. The control plane uses this to populate the gateways table.
//
// Implementations:
//   - localgateway.Gateway: returns the Aegis embedded HTTP gateway status
//   - provider.ProviderGatewayCollector: returns status from all registered providers
//     (Caddy HTTP, HAProxy EdgeMux, HAProxy TCP, etc.) discovered on the node
//
// To add a new gateway type, implement this interface and register it in the
// provider registry (see internal/provider/registry.go).
type GatewayStatusProvider interface {
	// LocalGatewayStatuses returns the current status of all local gateways.
	// Each entry represents a single gateway instance on this node.
	// Returns nil if no gateways are available.
	LocalGatewayStatuses() []*LocalGatewayInfo
}

// LocalGatewayInfo describes a single gateway for heartbeat payloads.
// Each field maps directly to gateway.GatewayInventory for DB upsert.
type LocalGatewayInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`     // local | private | public (gateway.GWType constants)
	Provider  string `json:"provider"` // caddy | haproxy | aegis (gateway.GWProvider constants)
	BindAddr  string `json:"bind_addr"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Scheme    string `json:"scheme"` // http | https
	Enabled   bool   `json:"enabled"`
	Status    string `json:"status"`
	LastError string `json:"last_error,omitempty"`
}

// ProviderGatewayStatusProvider implements GatewayStatusProvider by running
// provider discovery on each call. This is the production implementation used
// by the node agent to report all installed gateway programs (Caddy, HAProxy,
// etc.) plus the embedded local gateway via heartbeat.
//
// Usage:
//
//	reg := provider.NewRegistry()
//	reg.RegisterBuiltin(provider.NewCaddyHTTPProvider(cfg))
//	reg.RegisterBuiltin(provider.NewHAProxyEdgeMuxProvider(...))
//	collector := noderuntime.NewProviderGatewayStatusProvider(reg)
//	reconciler.SetGatewayStatusProvider(collector)
type ProviderGatewayStatusProvider struct {
	registry *provider.Registry
	// Fallback provider for the embedded local gateway (optional).
	// If set, its status is appended after the provider-based statuses.
	localGW GatewayStatusProvider
}

// NewProviderGatewayStatusProvider creates a provider-based gateway status provider.
func NewProviderGatewayStatusProvider(registry *provider.Registry) *ProviderGatewayStatusProvider {
	return &ProviderGatewayStatusProvider{registry: registry}
}

// SetLocalGateway sets a fallback provider for the embedded local HTTP gateway.
// Its status is appended after the provider-based statuses in LocalGatewayStatuses().
func (p *ProviderGatewayStatusProvider) SetLocalGateway(gw GatewayStatusProvider) {
	p.localGW = gw
}

// LocalGatewayStatuses implements GatewayStatusProvider.
// Runs fresh provider discovery on each call, then converts results to heartbeat format.
func (p *ProviderGatewayStatusProvider) LocalGatewayStatuses() []*LocalGatewayInfo {
	var all []*LocalGatewayInfo

	// 1. Collect status from all registered providers (Caddy, HAProxy, etc.)
	if p.registry != nil {
		discovered := provider.DiscoverProviders(p.registry)
		infos := CollectGatewayStatuses(discovered)
		all = append(all, infos...)
	}

	// 2. Append local gateway status if available
	if p.localGW != nil {
		localInfos := p.localGW.LocalGatewayStatuses()
		all = append(all, localInfos...)
	}

	return all
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

// ============================================================================
// Provider-based gateway status collection
// ============================================================================

// CollectGatewayStatuses converts provider discovery results into heartbeat-ready
// LocalGatewayInfo entries. Each discovered provider becomes one gateway row
// in the control plane's gateway inventory table.
//
// This is the bridge between the provider layer and the node runtime layer.
// The node agent calls provider.DiscoverProviders() to detect installed gateway
// programs, then passes the results to this function to convert them into the
// format expected by the heartbeat API.
//
// Mapping rules:
//   - Detected + Running → status "online", Enabled=true
//   - Detected + NotRunning → status "offline", Enabled=false
//   - Not detected → status "unavailable", Enabled=false
//
// GatewayType → inventory type mapping:
//   - TypeHTTPTerm, TypeSNIPass → "public" (internet-facing)
//   - TypeTCPForward, TypeUDPForward → "private" (internal)
//   - others → "local"
func CollectGatewayStatuses(discovered []provider.DiscoveredProvider) []*LocalGatewayInfo {
	if len(discovered) == 0 {
		return nil
	}

	result := make([]*LocalGatewayInfo, 0, len(discovered))
	for _, d := range discovered {
		info := discoveredToGatewayInfo(d)
		result = append(result, info)
	}
	return result
}

// discoveredToGatewayInfo converts a single discovered provider to gateway info.
func discoveredToGatewayInfo(d provider.DiscoveredProvider) *LocalGatewayInfo {
	// Map GatewayType → inventory type
	invType := "local"
	switch d.GatewayType {
	case provider.TypeHTTPTerm, provider.TypeSNIPass:
		invType = "public"
	case provider.TypeTCPForward, provider.TypeUDPForward:
		invType = "private"
	}

	// Map GatewayType → URL scheme
	scheme := "tcp"
	switch d.GatewayType {
	case provider.TypeHTTPTerm:
		scheme = "http"
	case provider.TypeSNIPass:
		scheme = "https"
	}

	// Map GatewayType → inventory provider name
	provName := "unknown"
	switch d.GatewayType {
	case provider.TypeHTTPTerm:
		provName = "caddy"
	case provider.TypeSNIPass, provider.TypeTCPForward:
		provName = "haproxy"
	case provider.TypeUDPForward, provider.TypeTransparent:
		provName = "aegis"
	}

	// Status & enabled
	status := "unavailable"
	enabled := false
	lastError := d.StatusMessage
	if d.Detected {
		if d.Running {
			status = "online"
			enabled = true
			lastError = ""
		} else {
			status = "offline"
		}
	}

	// Bind address & port
	bindAddr := "0.0.0.0"
	port := 0
	if len(d.ListeningPorts) > 0 {
		port = d.ListeningPorts[0]
	}

	return &LocalGatewayInfo{
		Name:      d.ProviderID,
		Type:      invType,
		Provider:  provName,
		BindAddr:  bindAddr,
		Host:      bindAddr,
		Port:      port,
		Scheme:    scheme,
		Enabled:   enabled,
		Status:    status,
		LastError: lastError,
	}
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
