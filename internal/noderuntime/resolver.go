package noderuntime

// RoutingDecision is the result of resolving a domain against the routing table.
type RoutingDecision struct {
	Domain             string            `json:"domain"`
	Status             string            `json:"status"`
	RouteID            string            `json:"route_id,omitempty"`
	ServiceID          string            `json:"service_id,omitempty"`
	EndpointID         string            `json:"endpoint_id,omitempty"`
	TargetNodeID       string            `json:"target_node_id,omitempty"`
	SelectedCandidate  *CandidateEntry   `json:"selected_candidate,omitempty"`
	FallbackCandidates []CandidateEntry  `json:"fallback_candidates,omitempty"`
	UnavailableReason  string            `json:"unavailable_reason,omitempty"`
}

// Resolver resolves domains against the local routing table cache.
type Resolver struct {
	table *RoutingTableCache
}

// NewResolver creates a new resolver with the given routing table.
func NewResolver(table *RoutingTableCache) *Resolver {
	return &Resolver{table: table}
}

// Resolve resolves a domain to a routing decision.
func (r *Resolver) Resolve(domain string) *RoutingDecision {
	decision := &RoutingDecision{
		Domain: domain,
		Status: "unavailable",
	}

	// Find exact domain match
	var entry *RoutingTableEntry
	for i := range r.table.Entries {
		if r.table.Entries[i].Domain == domain {
			entry = &r.table.Entries[i]
			break
		}
	}

	if entry == nil {
		decision.UnavailableReason = "domain not found in routing table"
		return decision
	}

	decision.RouteID = entry.RouteID
	decision.ServiceID = entry.ServiceID
	decision.EndpointID = entry.EndpointID
	decision.TargetNodeID = entry.TargetNodeID

	// Check if entry is disabled or unavailable
	if entry.Status == "disabled" {
		decision.Status = "disabled"
		decision.UnavailableReason = "routing entry is disabled by policy"
		return decision
	}

	if entry.Status != "available" {
		decision.Status = entry.Status
		decision.UnavailableReason = "routing entry is not available"
		return decision
	}

	if len(entry.Candidates) == 0 {
		decision.UnavailableReason = "no candidates available for domain"
		return decision
	}

	// Select the best candidate (first = highest priority)
	selected := entry.Candidates[0]
	decision.SelectedCandidate = &selected

	// Build fallback list (remaining candidates)
	if len(entry.Candidates) > 1 {
		fallbacks := make([]CandidateEntry, len(entry.Candidates)-1)
		copy(fallbacks, entry.Candidates[1:])
		decision.FallbackCandidates = fallbacks
	}


	decision.Status = "available"
	return decision
}
