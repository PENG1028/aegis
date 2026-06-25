package node

import (
	"encoding/json"
	"os/exec"
	"sort"
)

// Capability constants for node capability detection.
const (
	CapGatewayEnabled      = "gateway_enabled"
	CapCaddyInstalled      = "caddy_installed"
	CapHAProxyInstalled    = "haproxy_installed"
	CapTLSSupported        = "tls_supported"
	CapDNSControlAvailable = "dns_control_available"
	CapHotReloadSupported  = "hot_reload_supported"
	CapEdgeMuxSupported    = "edge_mux_supported"
)

// NodeCapabilities represents the detected capabilities of a node.
type NodeCapabilities map[string]bool

// CapabilityDiff tracks changes in node capabilities between two snapshots.
type CapabilityDiff struct {
	Added   []string `json:"added"`
	Removed []string `json:"removed"`
	Changed []string `json:"changed"`
}

// DefaultCapabilities returns a capability map with all capabilities set to false.
func DefaultCapabilities() NodeCapabilities {
	return NodeCapabilities{
		CapGatewayEnabled:      false,
		CapCaddyInstalled:      false,
		CapHAProxyInstalled:    false,
		CapTLSSupported:        false,
		CapDNSControlAvailable: false,
		CapHotReloadSupported:  false,
		CapEdgeMuxSupported:    false,
	}
}

// DetectCapabilities detects node capabilities at runtime.
func DetectCapabilities() NodeCapabilities {
	caps := DefaultCapabilities()

	// Check for Caddy binary
	if _, err := exec.LookPath("caddy"); err == nil {
		caps[CapCaddyInstalled] = true
		caps[CapGatewayEnabled] = true
	}

	// Check for HAProxy binary
	if _, err := exec.LookPath("haproxy"); err == nil {
		caps[CapHAProxyInstalled] = true
		caps[CapGatewayEnabled] = true
		caps[CapEdgeMuxSupported] = true
	}

	// Gateway is enabled if either Caddy or HAProxy is installed
	caps[CapGatewayEnabled] = caps[CapCaddyInstalled] || caps[CapHAProxyInstalled]

	// TLS is supported (Caddy and HAProxy both support it)
	caps[CapTLSSupported] = caps[CapGatewayEnabled]

	// Hot reload is supported if systemctl is available
	if _, err := exec.LookPath("systemctl"); err == nil {
		caps[CapHotReloadSupported] = true
	}

	// DNS control check (dig/nslookup)
	if _, err := exec.LookPath("dig"); err == nil {
		caps[CapDNSControlAvailable] = true
	}
	if _, err := exec.LookPath("nslookup"); err == nil {
		caps[CapDNSControlAvailable] = true
	}

	return caps
}

// DiffCapabilities compares two capability snapshots and returns the diff.
func DiffCapabilities(before, after NodeCapabilities) CapabilityDiff {
	diff := CapabilityDiff{}
	allKeys := make(map[string]bool)
	for k := range before {
		allKeys[k] = true
	}
	for k := range after {
		allKeys[k] = true
	}

	for k := range allKeys {
		bVal := before[k]
		aVal := after[k]
		if !bVal && aVal {
			diff.Added = append(diff.Added, k)
		} else if bVal && !aVal {
			diff.Removed = append(diff.Removed, k)
		} else if bVal != aVal {
			diff.Changed = append(diff.Changed, k)
		}
	}

	sort.Strings(diff.Added)
	sort.Strings(diff.Removed)
	sort.Strings(diff.Changed)

	return diff
}

// HasCapability checks if a capability is present in the map.
func (nc NodeCapabilities) HasCapability(cap string) bool {
	return nc[cap]
}

// ToJSON serializes capabilities to a JSON string for DB storage.
func (nc NodeCapabilities) ToJSON() string {
	data, _ := json.Marshal(nc)
	return string(data)
}

// ParseCapabilities deserializes a JSON string into a NodeCapabilities map.
func ParseCapabilities(raw string) NodeCapabilities {
	if raw == "" || raw == "{}" {
		return DefaultCapabilities()
	}
	caps := make(NodeCapabilities)
	if err := json.Unmarshal([]byte(raw), &caps); err != nil {
		return DefaultCapabilities()
	}
	return caps
}

// HasDiff returns true if there are any changes in the diff.
func (d CapabilityDiff) HasDiff() bool {
	return len(d.Added) > 0 || len(d.Removed) > 0 || len(d.Changed) > 0
}

// DisabledActions returns a list of actions that should be disabled based on capabilities.
func (nc NodeCapabilities) DisabledActions() []map[string]string {
	var actions []map[string]string
	if !nc[CapGatewayEnabled] {
		actions = append(actions, map[string]string{
			"action":  "create_gateway_domain",
			"reason":  "Gateway not installed on this node",
			"missing": CapGatewayEnabled,
		})
	}
	if !nc[CapDNSControlAvailable] {
		actions = append(actions, map[string]string{
			"action":  "bind_domain",
			"reason":  "DNS control not available on this node",
			"missing": CapDNSControlAvailable,
		})
	}
	if !nc[CapHotReloadSupported] {
		actions = append(actions, map[string]string{
			"action":  "hot_reload",
			"reason":  "Hot reload not supported on this node",
			"missing": CapHotReloadSupported,
		})
	}
	if !nc[CapTLSSupported] {
		actions = append(actions, map[string]string{
			"action":  "enable_tls",
			"reason":  "TLS not supported on this node",
			"missing": CapTLSSupported,
		})
	}
	return actions
}
