package node

import "strings"

const stableNodePrefix = "node_"

// StableNodeID returns Aegis' persisted node identity for a host/name.
func StableNodeID(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, stableNodePrefix) {
		return name
	}
	return stableNodePrefix + name
}

// LegacyNodeName returns the hostname-style name behind a stable node ID.
func LegacyNodeName(nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	return strings.TrimPrefix(nodeID, stableNodePrefix)
}
