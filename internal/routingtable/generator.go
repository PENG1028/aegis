package routingtable

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Generator produces routing tables for nodes.
type Generator struct{}

// NewGenerator creates a new routing table generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate produces a routing table for fromNodeID based on all inputs.
func (g *Generator) Generate(input GenerateInput) (*RoutingTable, error) {
	var entries []RoutingTableEntry
	var warnings []string

	for _, route := range input.AllRoutes {
		entry := g.buildEntry(route, input)
		entries = append(entries, entry)
	}

	if entries == nil {
		entries = []RoutingTableEntry{}
	}

	return &RoutingTable{
		NodeID:   input.FromNodeID,
		Revision: 0,
		Entries:  entries,
		Warnings: warnings,
	}, nil
}

func (g *Generator) buildEntry(route RouteInfo, input GenerateInput) RoutingTableEntry {
	entry := RoutingTableEntry{
		Domain:       route.Domain,
		RouteID:      route.RouteID,
		ServiceID:    route.ServiceID,
		FromNodeID:   input.FromNodeID,
		TargetNodeID: "",
		Protocol:     "http",
		Status:       StatusAvailable,
	}

	// 1. Resolve policy
	policy, err := input.ResolvePolicy(route.RouteID, route.ServiceID)
	if err != nil {
		entry.Status = StatusPolicyRejected
		entry.UnavailableReason = fmt.Sprintf("policy resolution error: %v", err)
		return entry
	}

	entry.GatewayPolicy = GatewayPolicyInfo{
		Mode:               policy.Mode,
		RequireGatewayLink: policy.RequireGatewayLink,
		RequireRelay:       policy.RequireRelay,
		PreserveHost:       policy.PreserveHost,
		TLSMode:            policy.TLSMode,
	}

	// 2. Check if disabled
	if policy.Mode == "disabled" {
		entry.Status = StatusDisabled
		entry.UnavailableReason = "policy mode is disabled"
		return entry
	}

	// 3. Find endpoint for this route/service
	endpoint := findEndpointForService(route.ServiceID, input.AllEndpoints)
	if endpoint == nil {
		entry.Status = StatusMissingEndpoint
		entry.UnavailableReason = "no endpoint found for service"
		return entry
	}
	entry.EndpointID = endpoint.EndpointID
	entry.TargetNodeID = endpoint.NodeID
	entry.TargetLocalHost, entry.TargetLocalPort = parseEndpointAddress(endpoint.Address)

	// 4. Generate candidates based on policy mode
	switch policy.Mode {
	case "fixed":
		g.buildFixedCandidates(&entry, endpoint, policy, input)
	case "multi":
		g.buildMultiCandidates(&entry, endpoint, policy, input)
	default: // auto
		g.buildAutoCandidates(&entry, endpoint, policy, input)
	}

	// 5. Final status assessment
	g.finalizeStatus(&entry)

	return entry
}

func (g *Generator) buildFixedCandidates(entry *RoutingTableEntry, endpoint *EndpointInfo, policy PolicyInfo, input GenerateInput) {
	if policy.PrimaryGatewayID == "" {
		entry.Status = StatusUnavailable
		entry.UnavailableReason = "fixed mode requires primary_gateway_id"
		return
	}

	candidate, ok := g.buildCandidateFromGateway(policy.PrimaryGatewayID, endpoint, policy, input)
	if !ok {
		entry.Status = StatusUnavailable
		entry.UnavailableReason = fmt.Sprintf("primary gateway %s not found or not allowed", policy.PrimaryGatewayID)
		return
	}

	entry.Candidates = []Candidate{*candidate}
}

func (g *Generator) buildMultiCandidates(entry *RoutingTableEntry, endpoint *EndpointInfo, policy PolicyInfo, input GenerateInput) {
	var candidates []Candidate

	// Primary first
	if policy.PrimaryGatewayID != "" {
		if c, ok := g.buildCandidateFromGateway(policy.PrimaryGatewayID, endpoint, policy, input); ok {
			c.Priority = 1
			candidates = append(candidates, *c)
		}
	}

	// Fallbacks in order
	for i, fbID := range policy.FallbackGatewayIDs {
		if c, ok := g.buildCandidateFromGateway(fbID, endpoint, policy, input); ok {
			c.Priority = i + 2
			candidates = append(candidates, *c)
		}
	}

	if len(candidates) == 0 {
		entry.Status = StatusUnavailable
		entry.UnavailableReason = "no valid candidates in multi mode"
		return
	}

	entry.Candidates = candidates
}

func (g *Generator) buildAutoCandidates(entry *RoutingTableEntry, endpoint *EndpointInfo, policy PolicyInfo, input GenerateInput) {
	var candidates []Candidate

	// Priority 1: Local — target endpoint is on the same node
	if policy.AllowLocal && endpoint.NodeID == input.FromNodeID {
		localGW := findLocalGateway(input.FromNodeID, input.AllGateways)
		if localGW != nil {
			candidates = append(candidates, Candidate{
				Mode:              CandidateModeLocal,
				GatewayID:         localGW.GatewayID,
				GatewayURL:        fmt.Sprintf("%s://%s:%d", localGW.Scheme, localGW.Host, localGW.Port),
				Priority:          1,
				RequiresGatewayLink: false,
			})
		}
	}

	// Same node, endpoint has local target but no gateway → just report available with no candidate
	if endpoint.NodeID == input.FromNodeID {
		// For same-node, we still mark as available even without a matching gateway
		// The node can reach its own endpoint directly
	}

	// Cross-node candidates
	if endpoint.NodeID != input.FromNodeID {
		// Find gateway link for this pair
		gwLink := findGatewayLink(input.FromNodeID, endpoint.NodeID, input.GatewayLinks)
		topoEdge := findTopologyEdge(input.FromNodeID, endpoint.NodeID, input.TopologyEdges)

		// Private gateway candidate
		if policy.AllowPrivate {
			privateGW := findBestPrivateGateway(endpoint.NodeID, input.AllGateways)
			if privateGW != nil && topoEdge != nil && topoEdge.PrivateReachable {
				linkID := ""
				if gwLink != nil {
					linkID = gwLink.ID
				} else if topoEdge.GatewayLinkID != "" {
					linkID = topoEdge.GatewayLinkID
				}

				// Private candidate requires gateway link if policy says so
				if !policy.RequireGatewayLink || linkID != "" {
					candidates = append(candidates, Candidate{
						Mode:              CandidateModePrivate,
						GatewayID:         privateGW.GatewayID,
						GatewayURL:        fmt.Sprintf("%s://%s:%d", privateGW.Scheme, privateGW.Host, privateGW.Port),
						Priority:          2,
						RequiresGatewayLink: policy.RequireGatewayLink,
						GatewayLinkID:     linkID,
					})
				}
			}
		}

		// Public gateway candidate
		if policy.AllowPublic {
			publicGW := findBestPublicGateway(endpoint.NodeID, input.AllGateways)
			if publicGW != nil {
				linkID := ""
				if gwLink != nil {
					linkID = gwLink.ID
				} else if topoEdge != nil && topoEdge.GatewayLinkID != "" {
					linkID = topoEdge.GatewayLinkID
				}

				if !policy.RequireGatewayLink || linkID != "" {
					candidates = append(candidates, Candidate{
						Mode:              CandidateModePublic,
						GatewayID:         publicGW.GatewayID,
						GatewayURL:        fmt.Sprintf("%s://%s:%d", publicGW.Scheme, publicGW.Host, publicGW.Port),
						Priority:          3,
						RequiresGatewayLink: policy.RequireGatewayLink,
						GatewayLinkID:     linkID,
					})
				}
			}
		}
	}

	// Sort candidates by priority
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Priority < candidates[j].Priority
	})

	entry.Candidates = candidates
}

func (g *Generator) buildCandidateFromGateway(gatewayID string, endpoint *EndpointInfo, policy PolicyInfo, input GenerateInput) (*Candidate, bool) {
	for _, gw := range input.AllGateways {
		if gw.GatewayID != gatewayID || !gw.Enabled {
			continue
		}

		// Verify the gateway belongs to the target node
		if gw.NodeID != endpoint.NodeID {
			continue
		}

		// Check accessibility against policy
		if gw.Type == "public" && !policy.AllowPublic {
			return nil, false
		}
		if gw.Type == "private" && !policy.AllowPrivate {
			return nil, false
		}

		// Determine mode based on gateway type
		mode := CandidateModeLocal
		if gw.Type == "public" {
			mode = CandidateModePublic
		} else if gw.Type == "private" {
			mode = CandidateModePrivate
		}

		// Find gateway link for cross-node
		linkID := ""
		if endpoint.NodeID != input.FromNodeID && policy.RequireGatewayLink {
			link := findGatewayLink(input.FromNodeID, endpoint.NodeID, input.GatewayLinks)
			if link == nil {
				return nil, false
			}
			linkID = link.ID
		}

		return &Candidate{
			Mode:                mode,
			GatewayID:           gw.GatewayID,
			GatewayURL:          fmt.Sprintf("%s://%s:%d", gw.Scheme, gw.Host, gw.Port),
			Priority:            1,
			RequiresGatewayLink: policy.RequireGatewayLink && endpoint.NodeID != input.FromNodeID,
			GatewayLinkID:       linkID,
		}, true
	}
	return nil, false
}

func (g *Generator) finalizeStatus(entry *RoutingTableEntry) {
	if entry.Status != StatusAvailable {
		return // already set
	}

	// Same-node routing: always available (node can reach its own endpoints directly)
	if entry.TargetNodeID != "" && entry.TargetNodeID == entry.FromNodeID {
		entry.Status = StatusAvailable
		return
	}

	if len(entry.Candidates) == 0 {
		// Check if cross-node without gateway link
		if entry.TargetNodeID != "" && entry.TargetNodeID != entry.FromNodeID && entry.GatewayPolicy.RequireGatewayLink {
			entry.Status = StatusMissingGatewayLink
			entry.UnavailableReason = "no gateway link for cross-node routing"
			return
		}
		entry.Status = StatusUnavailable
		entry.UnavailableReason = "no candidates available"
		return
	}

	// Check cross-node candidates have gateway link
	if entry.TargetNodeID != "" && entry.TargetNodeID != entry.FromNodeID {
		hasLink := false
		for _, c := range entry.Candidates {
			if c.GatewayLinkID != "" {
				hasLink = true
				break
			}
		}
		if entry.GatewayPolicy.RequireGatewayLink && !hasLink {
			entry.Status = StatusMissingGatewayLink
			entry.UnavailableReason = "cross-node candidate missing gateway link"
			return
		}
	}

	entry.Status = StatusAvailable
}

// ============================================================================
// Helper functions
// ============================================================================

func findEndpointForService(serviceID string, endpoints []EndpointInfo) *EndpointInfo {
	for _, ep := range endpoints {
		if ep.ServiceID == serviceID {
			return &ep
		}
	}
	return nil
}

func findLocalGateway(nodeID string, gateways []GatewayInfo) *GatewayInfo {
	var best *GatewayInfo
	for _, gw := range gateways {
		if gw.NodeID == nodeID && gw.Enabled {
			if best == nil || gw.Priority < best.Priority {
				g := gw
				best = &g
			}
		}
	}
	return best
}

func findBestPrivateGateway(nodeID string, gateways []GatewayInfo) *GatewayInfo {
	var best *GatewayInfo
	for _, gw := range gateways {
		if gw.NodeID == nodeID && gw.Enabled && gw.PrivateAccessible {
			if best == nil || gw.Priority < best.Priority {
				g := gw
				best = &g
			}
		}
	}
	return best
}

func findBestPublicGateway(nodeID string, gateways []GatewayInfo) *GatewayInfo {
	var best *GatewayInfo
	for _, gw := range gateways {
		if gw.NodeID == nodeID && gw.Enabled && gw.PublicAccessible {
			if best == nil || gw.Priority < best.Priority {
				g := gw
				best = &g
			}
		}
	}
	return best
}

func findGatewayLink(fromNodeID, toNodeID string, links []GatewayLinkInfo) *GatewayLinkInfo {
	for _, l := range links {
		// Match by target_node_id; source_node_id is inferred from context
		if l.TargetNodeID == toNodeID {
			return &l
		}
	}
	return nil
}

func findTopologyEdge(fromNodeID, toNodeID string, edges []TopologyEdgeInfo) *TopologyEdgeInfo {
	for _, e := range edges {
		if e.FromNodeID == fromNodeID && e.ToNodeID == toNodeID {
			return &e
		}
	}
	return nil
}

func parseEndpointAddress(address string) (host string, port int) {
	if address == "" {
		return "127.0.0.1", 0
	}
	host = address
	port = 0

	// Try to split host:port
	idx := strings.LastIndex(address, ":")
	if idx > 0 {
		p, err := strconv.Atoi(address[idx+1:])
		if err == nil && p > 0 && p < 65536 {
			port = p
			host = address[:idx]
			// Handle IPv6: [::1]:port
			if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
				host = host[1 : len(host)-1]
			}
		}
	}
	return host, port
}
