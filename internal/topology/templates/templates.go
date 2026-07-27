// Package templates — predefined topology patterns for the Planner.
//
// Each template describes a known middleware combination (e.g., "Single Caddy",
// "HAProxy + Caddy") and knows how to build per-provider Plans from route intents.
//
// Adding a new template:
//  1. Create a file implementing the topology.Template interface
//  2. Add it to the defaultTemplates() list in this file
//  3. That's it — the Planner picks it up automatically
package templates

import (
	"aegis/internal/hostdep/provider"
	"aegis/internal/topology"
)

// Default returns the standard set of topology templates in priority
// order. The Planner tries them in order — first match wins.
func Default() []topology.Template {
	return []topology.Template{
		&SingleCaddy{},
		&HAProxyCaddy{},
		&SingleHAProxy{},
		&DedicatedPorts{},
	}
}

// ============================================================================
// Shared helpers for template implementations
// ============================================================================

// findProvider returns the first provider that has ALL of the given capabilities.
// Used by templates to select which provider handles which role.
func findProvider(available []provider.ProviderState, caps ...provider.Capability) *provider.ProviderState {
	for _, p := range available {
		has := true
		for _, c := range caps {
			if !p.HasCapability(c) {
				has = false
				break
			}
		}
		if has {
			return &p
		}
	}
	return nil
}

