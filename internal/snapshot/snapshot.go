package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Snapshot captures the full system state at a point in time.
type Snapshot struct {
	Version    string              `json:"version"`
	ExportedAt string              `json:"exported_at"`
	ConfigHash ConfigHash          `json:"config_hash"`
	Listeners  []ListenerState     `json:"listeners"`
	EdgeRules  []EdgeRuleState     `json:"edge_rules"`
	Routes     []RouteState        `json:"routes"`
	Providers  []ProviderState     `json:"providers"`
	Ports      []PortState         `json:"ports"`
}

// ConfigHash stores SHA256 hashes of rendered configs.
type ConfigHash struct {
	CaddyConfigHash   string `json:"caddy_config_hash"`
	HAProxyConfigHash string `json:"haproxy_config_hash"`
}

// ListenerState is a snapshot of a listener.
type ListenerState struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	BindIP   string `json:"bind_ip"`
	Port     int    `json:"port"`
	Status   string `json:"status"`
}

// EdgeRuleState is a snapshot of an edge rule.
type EdgeRuleState struct {
	ID       string `json:"id"`
	SNIHost  string `json:"sni_host"`
	Target   string `json:"target"`
	Status   string `json:"status"`
	ManagedBy string `json:"managed_by"`
}

// RouteState is a snapshot of a route.
type RouteState struct {
	ID     string `json:"id"`
	Domain string `json:"domain"`
	Path   string `json:"path_prefix"`
	Status string `json:"status"`
}

// ProviderState is a snapshot of a provider.
type ProviderState struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// PortState captures OS port ownership.
type PortState struct {
	BindIP  string `json:"bind_ip"`
	Port    int    `json:"port"`
	Owner   string `json:"owner"`
	Status  string `json:"status"`
}

// Hash computes SHA256 of a string.
func Hash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// Export writes a snapshot to a file.
func (s *Snapshot) Export(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return nil
}

// Load reads a snapshot from a file.
func Load(path string) (*Snapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read snapshot: %w", err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &s, nil
}

// NewSnapshot creates a timestamped snapshot.
func NewSnapshot() *Snapshot {
	return &Snapshot{
		Version:    "1.0",
		ExportedAt: time.Now().Format(time.RFC3339),
	}
}
