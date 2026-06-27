package noderuntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Cache file names.
const (
	DesiredStateCacheFile = "desired-state.json"
	RoutingTableCacheFile = "routing-table.json"
	ActualStateCacheFile  = "actual-state.json"
	RuntimeStatusFile     = "runtime-status.json"
)

// DesiredStateCache is the cached desired state.
type DesiredStateCache struct {
	NodeID    string `json:"node_id"`
	Revision  int    `json:"revision"`
	StateHash string `json:"state_hash"`
	StateJSON string `json:"state_json"`
}

// RoutingTableCache is the cached routing table.
type RoutingTableCache struct {
	NodeID   string                 `json:"node_id"`
	Revision int                    `json:"revision"`
	Entries  []RoutingTableEntry    `json:"entries"`
}

// RoutingTableEntry is a single routing table entry from desired state.
type RoutingTableEntry struct {
	Domain       string              `json:"domain"`
	RouteID      string              `json:"route_id"`
	ServiceID    string              `json:"service_id"`
	EndpointID   string              `json:"endpoint_id"`
	FromNodeID   string              `json:"from_node_id"`
	TargetNodeID string              `json:"target_node_id"`
	TargetLocalHost string          `json:"target_local_host,omitempty"`
	TargetLocalPort int             `json:"target_local_port,omitempty"`
	Protocol     string              `json:"protocol"`
	Status       string              `json:"status"`
	Candidates   []CandidateEntry    `json:"candidates,omitempty"`
}

// CandidateEntry is a single candidate in the routing table.
type CandidateEntry struct {
	Mode              string `json:"mode"`
	GatewayID         string `json:"gateway_id"`
	GatewayURL        string `json:"gateway_url"`
	Priority          int    `json:"priority"`
	RequiresGatewayLink bool `json:"requires_gateway_link"`
	GatewayLinkID     string `json:"gateway_link_id,omitempty"`
}

// ActualStateCache is the cached actual state.
type ActualStateCache struct {
	AppliedRevision int    `json:"applied_revision"`
	StateHash       string `json:"state_hash"`
	Status          string `json:"status"`
	LastError       string `json:"last_error,omitempty"`
	ReportedAt      string `json:"reported_at"`
}

// CacheManager manages local node cache files.
type CacheManager struct {
	cacheDir string
}

// NewCacheManager creates a new cache manager.
func NewCacheManager(cacheDir string) *CacheManager {
	return &CacheManager{cacheDir: cacheDir}
}

// EnsureDir ensures the cache directory exists.
func (m *CacheManager) EnsureDir() error {
	return os.MkdirAll(m.cacheDir, 0755)
}

// WriteDesiredState writes the desired state cache atomically.
func (m *CacheManager) WriteDesiredState(cache *DesiredStateCache) error {
	return m.atomicWrite(DesiredStateCacheFile, cache)
}

// ReadDesiredState reads the desired state cache.
func (m *CacheManager) ReadDesiredState() (*DesiredStateCache, error) {
	var cache DesiredStateCache
	if err := m.readJSON(DesiredStateCacheFile, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// WriteRoutingTable writes the routing table cache atomically.
func (m *CacheManager) WriteRoutingTable(cache *RoutingTableCache) error {
	return m.atomicWrite(RoutingTableCacheFile, cache)
}

// ReadRoutingTable reads the routing table cache.
func (m *CacheManager) ReadRoutingTable() (*RoutingTableCache, error) {
	var cache RoutingTableCache
	if err := m.readJSON(RoutingTableCacheFile, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// WriteActualState writes the actual state cache atomically.
func (m *CacheManager) WriteActualState(cache *ActualStateCache) error {
	return m.atomicWrite(ActualStateCacheFile, cache)
}

// ReadActualState reads the actual state cache.
func (m *CacheManager) ReadActualState() (*ActualStateCache, error) {
	var cache ActualStateCache
	if err := m.readJSON(ActualStateCacheFile, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// CacheFileExists returns true if a cache file exists.
func (m *CacheManager) CacheFileExists(name string) bool {
	_, err := os.Stat(m.path(name))
	return err == nil
}

// atomicWrite writes data to a file atomically using a temp file + rename.
func (m *CacheManager) atomicWrite(name string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", name, err)
	}

	path := m.path(name)
	tmpPath := path + ".tmp"

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", name, err)
	}

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("write tmp %s: %w", name, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", name, err)
	}

	return nil
}

// readJSON reads and unmarshals a JSON cache file.
func (m *CacheManager) readJSON(name string, v interface{}) error {
	data, err := os.ReadFile(m.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("cache %s not found", name)
		}
		return fmt.Errorf("read cache %s: %w", name, err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("parse cache %s: %w", name, err)
	}
	return nil
}

func (m *CacheManager) path(name string) string {
	return filepath.Join(m.cacheDir, name)
}

// ============================================================================
// Token safety check helpers
// ============================================================================

// ContainsRawToken checks if a string contains raw token-like patterns.
// This is a best-effort check to prevent accidental token leakage.
func ContainsRawToken(s string) bool {
	if len(s) < 32 {
		return false
	}
	// Check for hex token patterns (64 hex chars)
	hexCount := 0
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
			hexCount++
		} else {
			hexCount = 0
		}
		if hexCount >= 64 {
			return true
		}
	}
	return false
}
