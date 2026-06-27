package localgateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RoutingTableInfo provides routing table status for the status endpoint.
type RoutingTableInfo struct {
	Loaded   bool `json:"loaded"`
	Entries  int  `json:"entries"`
	Revision int  `json:"revision"`
}

// CacheInfo provides cache status for the status endpoint.
type CacheInfo struct {
	DesiredState bool `json:"desired_state"`
	RoutingTable bool `json:"routing_table"`
}

// LocalStatusResponse is the response for GET /__aegis/local/status.
type LocalStatusResponse struct {
	NodeID       string           `json:"node_id"`
	LocalGateway GatewayStatusInfo `json:"local_gateway"`
	RoutingTable RoutingTableInfo `json:"routing_table"`
	Cache        CacheInfo        `json:"cache"`
}

// RoutingTableStatusProvider provides routing table status for the handler.
type RoutingTableStatusProvider interface {
	GetRoutingTableStatus() RoutingTableInfo
}

// Handler handles incoming HTTP requests for the local gateway.
type Handler struct {
	resolver    DomainResolver
	forwarder   *LocalForwarder
	relayClient *RelayClient
	config      *Config
	nodeID      string
	status      *GatewayStatus
	rtProvider  RoutingTableStatusProvider
}

// NewHandler creates a new local gateway handler.
func NewHandler(resolver DomainResolver, forwarder *LocalForwarder, relayClient *RelayClient, config *Config) *Handler {
	return &Handler{
		resolver:    resolver,
		forwarder:   forwarder,
		relayClient: relayClient,
		config:      config,
		nodeID:      config.NodeID,
	}
}

// SetGatewayStatus sets the gateway status tracker for health/status endpoints.
func (h *Handler) SetGatewayStatus(s *GatewayStatus) {
	h.status = s
}

// SetRoutingTableStatusProvider sets the routing table status provider.
func (h *Handler) SetRoutingTableStatusProvider(p RoutingTableStatusProvider) {
	h.rtProvider = p
}

// stripAegisHeaders removes all X-Aegis-* headers from the incoming request.
// This prevents external users from injecting relay/internal headers.
// Trusted headers are set later by the handler, forwarder, and relay client.
func stripAegisHeaders(r *http.Request) {
	for key := range r.Header {
		if len(key) >= 8 {
			// Fast-path: check if starts with "X-Aegis-" or "X-AEGIS-"
			if (key[0] == 'X' || key[0] == 'x') && key[1] == '-' {
				upperKey := stringsToUpper(key)
				if stringsHasPrefix(upperKey, "X-AEGIS-") {
					delete(r.Header, key)
				}
			}
		}
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// HARDENING: Strip all X-Aegis-* headers from external requests.
	// Only the local gateway runtime may set these headers.
	stripAegisHeaders(r)

	// INTERNAL: Handle local status/health endpoints
	if stringsHasPrefix(stringsToUpper(r.URL.Path), "/__AEGIS/LOCAL/") {
		h.handleInternal(w, r)
		return
	}

	// Extract domain from Host header
	domain := r.Host
	if domain == "" {
		http.Error(w, "Missing Host header", http.StatusBadRequest)
		return
	}

	// Strip port from domain if present
	domain = stripPort(domain)

	// Resolve domain
	decision := h.resolver.Resolve(domain)
	if decision == nil {
		h.handleError(w, "routing resolution failed", http.StatusInternalServerError)
		return
	}

	// Check if domain is managed
	switch decision.Status {
	case "available":
		h.handleManaged(w, r, decision)
	case "disabled":
		h.handleUnmanaged(w, r, domain)
	case "unavailable":
		h.handleUnmanaged(w, r, domain)
	default:
		h.handleUnmanaged(w, r, domain)
	}
}

// handleInternal processes requests to /__aegis/local/* endpoints.
// Path matching is case-insensitive.
func (h *Handler) handleInternal(w http.ResponseWriter, r *http.Request) {
	// Normalize path to lowercase for matching
	path := stringsToUpper(r.URL.Path)
	switch path {
	case "/__AEGIS/LOCAL/HEALTH":
		h.handleLocalHealth(w, r)
	case "/__AEGIS/LOCAL/STATUS":
		h.handleLocalStatus(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// handleLocalHealth returns a simple health check.
// Never returns secrets or routing data.
func (h *Handler) handleLocalHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"service": "aegis-local-gateway",
	})
}

// handleLocalStatus returns gateway status without tokens or secrets.
func (h *Handler) handleLocalStatus(w http.ResponseWriter, r *http.Request) {
	resp := LocalStatusResponse{
		NodeID: h.nodeID,
	}

	// Gateway status (no raw tokens exposed by GatewayStatusInfo)
	if h.status != nil {
		resp.LocalGateway = h.status.Get()
	}

	// Routing table status
	if h.rtProvider != nil {
		resp.RoutingTable = h.rtProvider.GetRoutingTableStatus()
	} else {
		resp.RoutingTable.Loaded = false
	}

	// Cache status (derived from routing table)
	resp.Cache = CacheInfo{
		DesiredState: resp.RoutingTable.Loaded,
		RoutingTable: resp.RoutingTable.Loaded,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleManaged processes a managed domain request.
func (h *Handler) handleManaged(w http.ResponseWriter, r *http.Request, decision *RoutingDecision) {
	if decision.SelectedCandidate == nil {
		http.Error(w, "no candidate available", http.StatusServiceUnavailable)
		return
	}

	switch decision.SelectedCandidate.Mode {
	case "local_gateway":
		h.handleLocalDispatch(w, r, decision)
	case "private_gateway", "public_gateway":
		h.handleRemoteRelay(w, r, decision)
	default:
		http.Error(w, fmt.Sprintf("unsupported candidate mode: %s", decision.SelectedCandidate.Mode), http.StatusNotImplemented)
	}
}

// handleUnmanaged processes an unmanaged domain request.
func (h *Handler) handleUnmanaged(w http.ResponseWriter, r *http.Request, domain string) {
	switch h.config.UnmanagedMode {
	case UnmanagedReject:
		http.Error(w, "Misdirected Request: domain not managed by Aegis", http.StatusMisdirectedRequest)
	default:
		http.Error(w, "Misdirected Request: domain not managed by Aegis", http.StatusMisdirectedRequest)
	}
}

// handleError writes an error response without leaking internal details.
func (h *Handler) handleError(w http.ResponseWriter, msg string, status int) {
	http.Error(w, msg, status)
}

// handleLocalDispatch forwards a request to a local target.
func (h *Handler) handleLocalDispatch(w http.ResponseWriter, r *http.Request, decision *RoutingDecision) {
	targetHost := decision.TargetLocalHost
	targetPort := decision.TargetLocalPort

	// Target host must come from routing table, not from user request
	if targetHost == "" {
		targetHost = "127.0.0.1"
	}
	if targetPort == 0 {
		h.handleError(w, "local dispatch target port not configured", http.StatusInternalServerError)
		return
	}

	h.forwarder.Forward(w, r, targetHost, targetPort, decision.RouteID)
}

// handleRemoteRelay executes a managed relay request to a remote gateway.
func (h *Handler) handleRemoteRelay(w http.ResponseWriter, r *http.Request, decision *RoutingDecision) {
	candidate := decision.SelectedCandidate

	relayReq := &RelayRequest{
		Method:        r.Method,
		GatewayURL:    candidate.GatewayURL + "/__aegis/relay",
		Path:          r.URL.RequestURI(),
		Body:          r.Body,
		RouteID:       decision.RouteID,
		GatewayLinkID: candidate.GatewayLinkID,
		Headers: map[string]string{
			"X-Aegis-Route-ID":    decision.RouteID,
			"X-Aegis-Source-Node": h.nodeID,
		},
	}

	// Execute relay request
	resp, err := h.relayClient.Execute(relayReq)
	if err != nil {
		errMsg := err.Error()
		if containsAny(errMsg, "connection refused", "no such host", "timeout") {
			h.handleError(w, "remote gateway unavailable", http.StatusBadGateway)
		} else if containsAny(errMsg, "secret not found") {
			h.handleError(w, "gateway link authentication unavailable", http.StatusServiceUnavailable)
		} else {
			h.handleError(w, "relay execution failed", http.StatusInternalServerError)
		}
		return
	}
	defer resp.Body.Close()

	// Check for self-loop or auth errors
	if resp.StatusCode == http.StatusForbidden {
		h.handleError(w, "relay authentication failed", http.StatusBadGateway)
		return
	}

	// Copy response headers
	for key, values := range resp.Header {
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// containsAny checks if a string contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if stringsContains(s, sub) {
			return true
		}
	}
	return false
}

// stringsContains is a wrapper for strings.Contains for use in this package.
func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

// containsSubstring checks if s contains substr without importing strings.
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// stringsToUpper converts ASCII letters in s to uppercase.
func stringsToUpper(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] = b[i] - 32
		}
	}
	return string(b)
}

// stringsHasPrefix checks if s has prefix p.
func stringsHasPrefix(s, p string) bool {
	return len(s) >= len(p) && s[:len(p)] == p
}

// stripPort removes the port from a host string.
func stripPort(host string) string {
	for i := len(host) - 1; i >= 0; i-- {
		if host[i] == ':' {
			// Handle IPv6 [::1]:port
			if i > 0 && host[i-1] == ']' {
				return host[:i]
			}
			// Check if there's a '[' before the ':' - if so, it's IPv6 without port
			hasBracket := false
			for j := 0; j < i; j++ {
				if host[j] == '[' {
					hasBracket = true
					break
				}
			}
			if !hasBracket {
				return host[:i]
			}
		}
	}
	return host
}
