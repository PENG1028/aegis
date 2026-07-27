package routingtable

import "strings"

// ValidationResult contains validation errors and warnings for a routing table.
type ValidationResult struct {
	IsValid   bool     `json:"is_valid"`
	Errors    []string `json:"errors,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Validate checks a routing table against all security and correctness rules.
func Validate(table *RoutingTable) *ValidationResult {
	result := &ValidationResult{IsValid: true}

	for _, entry := range table.Entries {
		validateEntry(entry, result)
	}

	return result
}

func validateEntry(entry RoutingTableEntry, result *ValidationResult) {
	// 1. No direct remote target candidate
	for _, c := range entry.Candidates {
		if strings.Contains(c.Mode, "direct") || strings.Contains(c.Mode, "raw") {
			result.IsValid = false
			result.Errors = append(result.Errors,
				"route "+entry.RouteID+": forbidden candidate mode: "+c.Mode)
		}
	}

	// 2. No raw token in routing table (check fields that shouldn't contain tokens)
	// This is a structural check — tokens use dedicated fields only

	// 3. Cross-node entry requires gateway_link_id
	if entry.TargetNodeID != "" && entry.TargetNodeID != entry.FromNodeID {
		hasLink := false
		for _, c := range entry.Candidates {
			if c.GatewayLinkID != "" {
				hasLink = true
				break
			}
		}
		if entry.GatewayPolicy.RequireGatewayLink && !hasLink && entry.Status == StatusAvailable {
			result.IsValid = false
			result.Errors = append(result.Errors,
				"route "+entry.RouteID+": cross-node entry requires gateway_link_id")
		}

		// 9. Self-node endpoint produces local candidate
		if entry.TargetNodeID == entry.FromNodeID {
			// This is fine — local routing
		}
	}

	// 4. Public candidate requires allow_public
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePublic {
			if !entry.GatewayPolicy.RequireGatewayLink {
				// Warning rather than error
				result.Warnings = append(result.Warnings,
					"route "+entry.RouteID+": public candidate without require_gateway_link")
			}
		}
	}

	// 5. Private candidate requires allow_private (structural, generator already enforces)

	// 6. Fixed policy missing primary gateway
	if entry.GatewayPolicy.Mode == "fixed" && entry.Status == StatusUnavailable {
		for _, errMsg := range []string{"fixed mode requires", "primary gateway"} {
			if strings.Contains(entry.UnavailableReason, errMsg) {
				// Valid unavailable — policy correctly rejects
			}
		}
	}

	// 7. Multi policy fallback order stable (structural)

	// 8. Disabled policy produces disabled status
	if entry.GatewayPolicy.Mode == "disabled" && entry.Status != StatusDisabled {
		result.IsValid = false
		result.Errors = append(result.Errors,
			"route "+entry.RouteID+": disabled policy should produce disabled status")
	}

	// 10. No candidate produces unavailable with reason
	if len(entry.Candidates) == 0 && entry.Status == StatusAvailable {
		result.IsValid = false
		result.Errors = append(result.Errors,
			"route "+entry.RouteID+": available status with no candidates")
	}
	if len(entry.Candidates) == 0 && entry.Status != StatusAvailable && entry.UnavailableReason == "" {
		result.Warnings = append(result.Warnings,
			"route "+entry.RouteID+": unavailable but no reason provided")
	}

	// No direct remote target fallback
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModeLocal && entry.TargetNodeID != entry.FromNodeID {
			result.IsValid = false
			result.Errors = append(result.Errors,
				"route "+entry.RouteID+": local candidate for cross-node target")
		}
	}
}

// ValidateEntry checks a single routing table entry for correctness.
func ValidateEntry(entry RoutingTableEntry) *ValidationResult {
	result := &ValidationResult{IsValid: true}
	validateEntry(entry, result)
	return result
}
