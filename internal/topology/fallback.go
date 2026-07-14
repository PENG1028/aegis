package topology

import (
	"fmt"
	"strings"

	"aegis/internal/hostdep/provider"
)

// ============================================================================
// 4-level fallback strategy
// ============================================================================
//
// When no template perfectly matches, the Planner generates fallback solutions
// at decreasing levels of quality:
//
//   Level 0 — Equivalent:     same function, different middleware
//                              e.g. HAProxy SNI → Nginx stream_ssl_preread
//   Level 1 — Same function,   worse operational characteristics
//                              e.g. Caddy auto_cert → Nginx + certbot (manual)
//   Level 2 — Degraded:        works but with reduced capability
//                              e.g. No SNI preread → TCP on dedicated ports
//   Level 3 — Impossible:      cannot be fulfilled without installing new middleware
//                              e.g. HTTP/3 QUIC but no QUIC-capable provider

// EvaluateFallback assesses how far a set of available providers is from
// satisfying the required capabilities. Returns the fallback level and a
// human-readable explanation.
func EvaluateFallback(required []provider.Capability, available []provider.ProviderState) (level int, explanation string) {
	missing := missingCapabilities(required, available)
	if len(missing) == 0 {
		return 0, "all capabilities satisfied"
	}

	// Classify missing capabilities
	var coreMissing, operationalMissing, niceToHaveMissing []string
	for _, cap := range missing {
		switch {
		case cap.IsIngress():
			coreMissing = append(coreMissing, string(cap))
		case cap == provider.CapAutoCert || cap == provider.CapHealthCheck ||
			cap == provider.CapLoadBalance || cap == provider.CapRateLimit:
			operationalMissing = append(operationalMissing, string(cap))
		default:
			niceToHaveMissing = append(niceToHaveMissing, string(cap))
		}
	}

	switch {
	case len(coreMissing) > 0:
		level = 2
		explanation = fmt.Sprintf("degraded — missing core ingress capabilities: %s. "+
			"Consider installing additional middleware.",
			strings.Join(coreMissing, ", "))
	case len(operationalMissing) > 0:
		level = 1
		explanation = fmt.Sprintf("functional but suboptimal — missing operational capabilities: %s. "+
			"Same routing works but with reduced automation.",
			strings.Join(operationalMissing, ", "))
	case len(niceToHaveMissing) > 0:
		level = 0
		explanation = fmt.Sprintf("minor gap — missing non-critical capabilities: %s",
			strings.Join(niceToHaveMissing, ", "))
	default:
		level = 3
		explanation = fmt.Sprintf("impossible — missing: %s. Must install new middleware.",
			strings.Join(missingStrings(missing), ", "))
	}

	return level, explanation
}

// FallbackSolution generates a degraded solution description for a set of
// routes that cannot be perfectly satisfied.
func FallbackSolution(intents []RouteIntent, available []provider.ProviderState) Solution {
	var allCaps []provider.Capability
	for _, ri := range intents {
		allCaps = append(allCaps, ri.RequirementsOf()...)
	}
	// Deduplicate
	seen := make(map[provider.Capability]bool)
	var unique []provider.Capability
	for _, c := range allCaps {
		if !seen[c] {
			seen[c] = true
			unique = append(unique, c)
		}
	}

	level, explanation := EvaluateFallback(unique, available)
	return Solution{
		TemplateName: "fallback",
		Level:        level,
		Description:  explanation,
		Providers:    providerIDs(available),
	}
}

// missingStrings converts Capability slice to string slice for messages.
func missingStrings(caps []provider.Capability) []string {
	s := make([]string, len(caps))
	for i, c := range caps {
		s[i] = string(c)
	}
	return s
}
