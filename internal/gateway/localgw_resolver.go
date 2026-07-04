package gateway

// DomainResolver resolves domains to routing decisions for the local gateway.
type DomainResolver interface {
	// Resolve resolves a domain against the local routing table.
	Resolve(domain string) *RoutingDecision
}

// RoutingDecision is the result of resolving a domain.
type RoutingDecision struct {
	Domain             string           `json:"domain"`
	Status             string           `json:"status"`
	RouteID            string           `json:"route_id,omitempty"`
	ServiceID          string           `json:"service_id,omitempty"`
	EndpointID         string           `json:"endpoint_id,omitempty"`
	TargetNodeID       string           `json:"target_node_id,omitempty"`
	TargetLocalHost    string           `json:"target_local_host,omitempty"`
	TargetLocalPort    int              `json:"target_local_port,omitempty"`
	SelectedCandidate  *CandidateEntry  `json:"selected_candidate,omitempty"`
	FallbackCandidates []CandidateEntry `json:"fallback_candidates,omitempty"`
	UnavailableReason  string           `json:"unavailable_reason,omitempty"`
}

// CandidateEntry represents a single candidate in the routing decision.
type CandidateEntry struct {
	Mode              string `json:"mode"`
	GatewayID         string `json:"gateway_id"`
	GatewayURL        string `json:"gateway_url"`
	Priority          int    `json:"priority"`
	RequiresGatewayLink bool `json:"requires_gateway_link"`
	GatewayLinkID     string `json:"gateway_link_id,omitempty"`
}
