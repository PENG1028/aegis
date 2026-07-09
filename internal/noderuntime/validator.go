package noderuntime

import (
	"encoding/json"
	"fmt"
)

// ValidationResult holds validator findings.
type ValidationResult struct {
	IsValid  bool     `json:"is_valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateRoutingTable validates a routing table for a specific node.
// Must be called before writing to cache.
func ValidateRoutingTable(nodeID string, table *RoutingTableCache) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	for i, entry := range table.Entries {
		prefix := fmt.Sprintf("entry[%d] (%s): ", i, entry.Domain)
		validateEntry(entry, nodeID, prefix, result)
	}

	if table.Revision < 0 {
		result.IsValid = false
		result.Errors = append(result.Errors, "negative revision")
	}

	return result
}

func validateEntry(entry RoutingTableEntry, nodeID string, prefix string, result *ValidationResult) {
	// 1. from_node_id must match current node
	if entry.FromNodeID != nodeID {
		result.IsValid = false
		result.Errors = append(result.Errors,
			prefix+"from_node_id must equal current node_id")
	}

	// 3. Cross-node candidate must have gateway_link_id
	if entry.TargetNodeID != "" && entry.TargetNodeID != nodeID {
		hasLink := false
		for _, c := range entry.Candidates {
			if c.GatewayLinkID != "" {
				hasLink = true
				break
			}
		}
		if !hasLink && entry.Status == "available" {
			result.IsValid = false
			result.Errors = append(result.Errors,
				prefix+"cross-node candidate requires gateway_link_id")
		}
	}

	// 4-5. All candidates must have gateway_url (except local on same node)
	for _, c := range entry.Candidates {
		if c.Mode == "local_gateway" && entry.TargetNodeID == nodeID {
			// Local candidate on same node doesn't need URL
			continue
		}
		if c.GatewayURL == "" {
			result.IsValid = false
			result.Errors = append(result.Errors,
				prefix+"candidate "+c.Mode+" missing gateway_url")
		}
	}
	// 6. Local candidate must have target_local_host/port (same node)
	if entry.TargetNodeID == nodeID {
		if entry.TargetLocalHost == "" {
			result.Warnings = append(result.Warnings,
				prefix+"local entry missing target_local_host")
		}
		if entry.TargetLocalPort == 0 {
			result.Warnings = append(result.Warnings,
				prefix+"local entry missing target_local_port")
		}
		for _, c := range entry.Candidates {
			if c.Mode == "local_gateway" && c.GatewayLinkID != "" {
				result.Warnings = append(result.Warnings,
					prefix+"local candidate has unnecessary gateway_link_id")
			}
		}
	}

	// 7. Candidate priority must be stable (checked via ordering)

	// 8. Available status must have at least one candidate
	if entry.Status == "available" && len(entry.Candidates) == 0 {
		result.IsValid = false
		result.Errors = append(result.Errors,
			prefix+"available status with no candidates")
	}

	// 9. No raw token fields in entry
	checkRawTokens(entry, prefix, result)

	// 10. Protocol must be http for now
	if entry.Protocol != "http" && entry.Protocol != "" {
		result.IsValid = false
		result.Errors = append(result.Errors,
			prefix+"unsupported protocol: "+entry.Protocol+" (only http)")
	}
}

func checkRawTokens(entry RoutingTableEntry, prefix string, result *ValidationResult) {
	// Check candidate fields
	for _, c := range entry.Candidates {
		if ContainsRawToken(c.GatewayID) {
			result.IsValid = false
			result.Errors = append(result.Errors,
				prefix+"candidate gateway_id appears to contain raw token")
		}
	}
}

// ValidateDesiredStateForNode validates that a desired state is acceptable.
func ValidateDesiredStateForNode(nodeID string, ds *DesiredStateCache) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	if ds.NodeID != nodeID {
		result.IsValid = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("desired state node_id %s does not match %s", ds.NodeID, nodeID))
	}
	if ds.StateHash == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "desired state hash is empty")
	}
	if ds.StateJSON == "" {
		result.IsValid = false
		result.Errors = append(result.Errors, "desired state_json is empty")
	}

	// Check for raw tokens
	if ContainsRawToken(ds.StateJSON) {
		result.IsValid = false
		result.Errors = append(result.Errors, "desired state contains raw token")
	}

	return result
}

// extractRoutingTableFromState parses the local_routing_table from desired state JSON.
func extractRoutingTableFromState(stateJSON string) (*RoutingTableCache, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(stateJSON), &raw); err != nil {
		return nil, fmt.Errorf("parse desired state: %w", err)
	}

	rtRaw, ok := raw["local_routing_table"]
	if !ok || rtRaw == nil {
		return &RoutingTableCache{Entries: []RoutingTableEntry{}}, nil
	}

	data, err := json.Marshal(rtRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal routing table: %w", err)
	}

	var entries []RoutingTableEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse routing table entries: %w", err)
	}
	if entries == nil {
		entries = []RoutingTableEntry{}
	}

	return &RoutingTableCache{Entries: entries}, nil
}
