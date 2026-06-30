package config

import (
	"aegis/internal/id"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the full Aegis configuration.
type Config struct {
	Proxy          ProxyConfig          `yaml:"proxy"`
	Store          StoreConfig          `yaml:"store"`
	Server         ServerConfig         `yaml:"server"`
	ManagedDomain  ManagedDomainConfig  `yaml:"managed_domain"`
	DNS            DNSConfig            `yaml:"dns"`
	Runtime        RuntimeConfig        `yaml:"runtime"`
}

// ProxyConfig holds proxy adapter settings.
type ProxyConfig struct {
	Provider        string `yaml:"provider"`
	CaddyfilePath   string `yaml:"caddyfile_path"`
	CaddyBinary     string `yaml:"caddy_binary"`
	ReloadCommand   string `yaml:"reload_command"`
	ValidateCommand string `yaml:"validate_command"`
	BackupDir       string `yaml:"backup_dir"`
	Email           string `yaml:"email"`
}

// StoreConfig holds database settings.
type StoreConfig struct {
	SQLitePath         string `yaml:"sqlite_path"`
	BackupEnabled      bool   `yaml:"backup_enabled"`
	BackupDir          string `yaml:"backup_dir"`
	BackupIntervalHrs  int    `yaml:"backup_interval_hrs"`
	BackupKeepCount    int    `yaml:"backup_keep_count"`
}

// ServerConfig holds HTTP API server settings.
type ServerConfig struct {
	Addr           string   `yaml:"addr"`
	AdminToken     string   `yaml:"admin_token"`
	SessionSecure  bool     `yaml:"session_secure"`
	AllowedOrigins []string `yaml:"allowed_origins"` // CORS origins (default: localhost only)
}

// ManagedDomainConfig holds managed domain settings.
type ManagedDomainConfig struct {
	GatewayDomain string `yaml:"gateway_domain"`
}

// DNSConfig holds local DNS resolver settings.
type DNSConfig struct {
	Enabled     bool   `yaml:"enabled"`
	ListenAddr  string `yaml:"listen_addr"`
	Upstream    string `yaml:"upstream"`
	RefreshSec  int    `yaml:"refresh_sec"`
}

// RuntimeConfig holds runtime paths.
type RuntimeConfig struct {
	ConfigDir string `yaml:"config_dir"`
	DataDir   string `yaml:"data_dir"`
}

// DefaultConfig returns a configuration with development defaults.
func DefaultConfig() *Config {
	cwd, _ := os.Getwd()
	baseDir := filepath.Join(cwd, ".aegis")

	return &Config{
		Proxy: ProxyConfig{
			Provider:        "caddy",
			CaddyfilePath:   filepath.Join(baseDir, "Caddyfile"),
			CaddyBinary:     "caddy",
			ReloadCommand:   "",
			ValidateCommand: "{{caddy_binary}} validate --adapter caddyfile --config {{config_path}}",
			BackupDir:       filepath.Join(baseDir, "backups"),
			Email:           "",
		},
		Store: StoreConfig{
			SQLitePath:        filepath.Join(baseDir, "aegis.db"),
			BackupEnabled:     true,
			BackupDir:         filepath.Join(baseDir, "backups", "db"),
			BackupIntervalHrs: 6,
			BackupKeepCount:   7,
		},
		Server: ServerConfig{
			Addr:           "127.0.0.1:7380",
			AdminToken:     generateAdminToken(),
			SessionSecure:  false, // dev: no TLS by default
			AllowedOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
		},
		DNS: DNSConfig{
			Enabled:    false,
			ListenAddr: ":5353", // non-privileged for dev
			Upstream:   "1.1.1.1:53",
			RefreshSec: 30,
		},
		ManagedDomain: ManagedDomainConfig{
			GatewayDomain: "",
		},
		Runtime: RuntimeConfig{
			ConfigDir: filepath.Join(baseDir, "config"),
			DataDir:   baseDir,
		},
	}
}

// ProductionConfig returns a configuration with system paths.
func ProductionConfig() *Config {
	return &Config{
		Proxy: ProxyConfig{
			Provider:        "caddy",
			CaddyfilePath:   "/etc/caddy/Caddyfile",
			CaddyBinary:     "caddy",
			ReloadCommand:   "systemctl reload caddy",
			ValidateCommand: "caddy validate --adapter caddyfile --config {{config_path}}",
			BackupDir:       "/var/lib/aegis/backups",
			Email:           "",
		},
		Store: StoreConfig{
			SQLitePath:        "/var/lib/aegis/aegis.db",
			BackupEnabled:     true,
			BackupDir:         "/var/lib/aegis/backups/db",
			BackupIntervalHrs: 6,
			BackupKeepCount:   7,
		},
		Server: ServerConfig{
			Addr:           "127.0.0.1:7380",
			AdminToken:     generateAdminToken(),
			SessionSecure:  true, // prod: assume TLS
			AllowedOrigins: []string{}, // empty = serve from same origin (embedded UI)
		},
		DNS: DNSConfig{
			Enabled:    false,
			ListenAddr: ":53",
			Upstream:   "1.1.1.1:53",
			RefreshSec: 30,
		},
		ManagedDomain: ManagedDomainConfig{
			GatewayDomain: "gateway.example.com",
		},
		Runtime: RuntimeConfig{
			ConfigDir: "/etc/aegis",
			DataDir:   "/var/lib/aegis",
		},
	}
}

// Load reads a YAML config file and returns a Config.
// Returns an error if the file is empty, unreadable, or missing required fields.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s: %w", path, err)
	}

	if len(data) == 0 || strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("config file %s is empty", path)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}

	// Validate required fields after load — a missing proxy provider or DB path
	// would cause obscure failures later. Catch them early.
	if cfg.Proxy.Provider == "" {
		cfg.Proxy.Provider = "caddy"
	}
	if cfg.Store.SQLitePath == "" {
		return nil, fmt.Errorf("config %s: store.sqlite_path is required", path)
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = "127.0.0.1:7380"
	}

	return cfg, nil
}

// Save writes the config to a YAML file.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file %s: %w", path, err)
	}
	return nil
}

// ResolveValidateCommand replaces template variables in the validate command.
func (c *Config) ResolveValidateCommand(configPath string) string {
	cmd := c.Proxy.ValidateCommand
	cmd = strings.ReplaceAll(cmd, "{{caddy_binary}}", c.Proxy.CaddyBinary)
	cmd = strings.ReplaceAll(cmd, "{{config_path}}", configPath)
	return cmd
}

// PanelCaddyfile returns the initial Caddyfile for the Aegis panel.
//
// Two modes depending on whether a domain is configured:
//   - With domain (ManagedDomain.GatewayDomain): automatic HTTPS via Let's Encrypt
//   - Without domain: HTTP only, with a clear warning to configure a domain
//
// Generated by `aegis bootstrap --production`.
func (c *Config) PanelCaddyfile() string {
	domain := c.ManagedDomain.GatewayDomain

	if domain != "" {
		// ─── Domain mode: auto-HTTPS ───
		return `# Aegis Control Panel — auto-generated by aegis bootstrap
# TLS: automatic via Caddy + Let's Encrypt (domain configured)
#
# Panel:  https://` + domain + `
# API:    https://` + domain + `/api/*
#
# User-defined routes are appended below by Aegis Apply.

` + domain + ` {
	# /api/* → Aegis API (authenticated)
	handle /api/* {
		reverse_proxy ` + c.Server.Addr + ` {
			header_up Host {host}
			header_up X-Forwarded-For {remote_host}
			header_up X-Forwarded-Proto {scheme}
		}
	}
	# /* → Aegis embedded UI (SPA)
	handle {
		reverse_proxy ` + c.Server.Addr + ` {
			header_up Host {host}
			header_up X-Forwarded-For {remote_host}
			header_up X-Forwarded-Proto {scheme}
		}
	}
}

# Redirect HTTP → HTTPS
:80 {
	redir https://{host}{uri} permanent
}
`
	}

	// ─── IP-only mode: HTTP, no TLS ───
	return `# Aegis Control Panel — auto-generated by aegis bootstrap
#
# ═══════════════════════════════════════════════════════════════
# WARNING: No domain configured — running WITHOUT TLS.
# Admin credentials will be transmitted in plaintext over HTTP.
#
# To enable automatic HTTPS (Let's Encrypt):
#   1. Point a domain to this server's public IP
#   2. Edit /etc/aegis/config.yaml:
#        managed_domain:
#          gateway_domain: "panel.yourdomain.com"
#   3. Run: aegis bootstrap --production
#   4. Reload Caddy: systemctl reload caddy
# ═══════════════════════════════════════════════════════════════
#
# Panel:  http://<server-ip>
# API:    http://<server-ip>/api/*

:80 {
	# /api/* → Aegis API (authenticated)
	handle /api/* {
		reverse_proxy ` + c.Server.Addr + ` {
			header_up Host {host}
			header_up X-Forwarded-For {remote_host}
			header_up X-Forwarded-Proto {scheme}
		}
	}
	# /* → Aegis embedded UI (SPA)
	handle {
		reverse_proxy ` + c.Server.Addr + ` {
			header_up Host {host}
			header_up X-Forwarded-For {remote_host}
			header_up X-Forwarded-Proto {scheme}
		}
	}
}
`
}

// generateAdminToken creates a cryptographically random 32-byte hex token.
// Delegates to id.GenerateRandomHex — the project's canonical random-hex generator.
func generateAdminToken() string {
	return id.GenerateRandomHex(32)
}
