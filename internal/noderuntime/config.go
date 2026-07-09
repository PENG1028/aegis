package noderuntime

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Default paths for node runtime.
const (
	DefaultConfigPath     = "/etc/aegis/node.yaml"
	DefaultCacheDir       = "/var/lib/aegis"
	DefaultRuntimeDir     = "/run/aegis"
	DefaultTokenFile      = "/etc/aegis/node.token"
	DefaultHeartbeatSec   = 15
	DefaultSyncSec        = 15
	DefaultReconcileMode  = "dry_run"
)

// Config holds the node runtime configuration.
type Config struct {
	ControlPlaneURL        string `yaml:"control_plane_url" json:"control_plane_url"`
	NodeID                 string `yaml:"node_id" json:"node_id"`
	NodeTokenFile          string `yaml:"node_token_file" json:"node_token_file"`
	NodeToken              string `yaml:"-" json:"-"` // loaded from file, never serialized
	CacheDir               string `yaml:"cache_dir" json:"cache_dir"`
	RuntimeDir             string `yaml:"runtime_dir" json:"runtime_dir"`
	HeartbeatIntervalSec   int    `yaml:"heartbeat_interval_seconds" json:"heartbeat_interval_seconds"`
	SyncIntervalSec        int    `yaml:"sync_interval_seconds" json:"sync_interval_seconds"`
	ReconcileMode          string `yaml:"reconcile_mode" json:"reconcile_mode"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ControlPlaneURL:      "http://127.0.0.1:8080",
		NodeID:               "",
		NodeTokenFile:        DefaultTokenFile,
		CacheDir:             DefaultCacheDir,
		RuntimeDir:           DefaultRuntimeDir,
		HeartbeatIntervalSec: DefaultHeartbeatSec,
		SyncIntervalSec:      DefaultSyncSec,
		ReconcileMode:        DefaultReconcileMode,
	}
}

// LoadConfig loads configuration from a YAML file and environment overrides.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config: %w", err)
		}
		type configFile struct {
			ControlPlaneURL      string `yaml:"control_plane_url"`
			NodeID               string `yaml:"node_id"`
			NodeTokenFile        string `yaml:"node_token_file"`
			CacheDir             string `yaml:"cache_dir"`
			RuntimeDir           string `yaml:"runtime_dir"`
			HeartbeatIntervalSec int    `yaml:"heartbeat_interval_seconds"`
			SyncIntervalSec      int    `yaml:"sync_interval_seconds"`
			ReconcileMode        string `yaml:"reconcile_mode"`
		}
		var fc configFile
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return nil, fmt.Errorf("parse YAML: %w", err)
		}
		if fc.ControlPlaneURL != "" {
			cfg.ControlPlaneURL = fc.ControlPlaneURL
		}
		if fc.NodeID != "" {
			cfg.NodeID = fc.NodeID
		}
		if fc.NodeTokenFile != "" {
			cfg.NodeTokenFile = fc.NodeTokenFile
		}
		if fc.CacheDir != "" {
			cfg.CacheDir = fc.CacheDir
		}
		if fc.RuntimeDir != "" {
			cfg.RuntimeDir = fc.RuntimeDir
		}
		if fc.HeartbeatIntervalSec > 0 {
			cfg.HeartbeatIntervalSec = fc.HeartbeatIntervalSec
		}
		if fc.SyncIntervalSec > 0 {
			cfg.SyncIntervalSec = fc.SyncIntervalSec
		}
		if fc.ReconcileMode != "" {
			cfg.ReconcileMode = fc.ReconcileMode
		}
	}

	// Environment overrides
	if v := os.Getenv("AEGIS_CONTROL_PLANE_URL"); v != "" {
		cfg.ControlPlaneURL = v
	}
	if v := os.Getenv("AEGIS_NODE_ID"); v != "" {
		cfg.NodeID = v
	}
	if v := os.Getenv("AEGIS_NODE_TOKEN_FILE"); v != "" {
		cfg.NodeTokenFile = v
	}
	if v := os.Getenv("AEGIS_NODE_TOKEN"); v != "" {
		cfg.NodeToken = v
	}
	if v := os.Getenv("AEGIS_CACHE_DIR"); v != "" {
		cfg.CacheDir = v
	}

	// Load token from file
	if cfg.NodeToken == "" && cfg.NodeTokenFile != "" {
		data, err := os.ReadFile(cfg.NodeTokenFile)
		if err == nil {
			cfg.NodeToken = strings.TrimSpace(string(data))
		}
	}

	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}
	if cfg.NodeToken == "" {
		return nil, fmt.Errorf("node_token is required (set token_file or AEGIS_NODE_TOKEN)")
	}
	if cfg.HeartbeatIntervalSec <= 0 {
		cfg.HeartbeatIntervalSec = DefaultHeartbeatSec
	}
	if cfg.SyncIntervalSec <= 0 {
		cfg.SyncIntervalSec = DefaultSyncSec
	}
	if cfg.ReconcileMode == "" {
		cfg.ReconcileMode = DefaultReconcileMode
	}

	return cfg, nil
}

// SafeString returns a log-safe representation of the config (no token).
func (c *Config) SafeString() string {
	return fmt.Sprintf("Config{control_plane=%s, node_id=%s, cache_dir=%s, heartbeat=%ds, sync=%ds, mode=%s}",
		c.ControlPlaneURL, c.NodeID, c.CacheDir,
		c.HeartbeatIntervalSec, c.SyncIntervalSec, c.ReconcileMode)
}

// Validate returns an error if the config is invalid.
func (c *Config) Validate() error {
	if c.NodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if c.NodeToken == "" {
		return fmt.Errorf("node_token is required")
	}
	if c.ControlPlaneURL == "" {
		return fmt.Errorf("control_plane_url is required")
	}
	return nil
}
