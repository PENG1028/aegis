package config

import (
	"aegis/internal/core"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the full Aegis configuration.
type Config struct {
	Proxy         ProxyConfig         `yaml:"proxy"`
	Store         StoreConfig         `yaml:"store"`
	Server        ServerConfig        `yaml:"server"`
	ManagedDomain ManagedDomainConfig `yaml:"managed_domain"`
	DNS           DNSConfig           `yaml:"dns"`
	Runtime       RuntimeConfig       `yaml:"runtime"`
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
	TlsCertFile     string `yaml:"tls_cert_file"` // custom TLS certificate (PEM)
	TlsKeyFile      string `yaml:"tls_key_file"`  // custom TLS private key (PEM)
}

// StoreConfig holds database settings.
type StoreConfig struct {
	SQLitePath        string `yaml:"sqlite_path"`
	BackupEnabled     bool   `yaml:"backup_enabled"`
	BackupDir         string `yaml:"backup_dir"`
	BackupIntervalHrs int    `yaml:"backup_interval_hrs"`
	BackupKeepCount   int    `yaml:"backup_keep_count"`
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
	Enabled    bool   `yaml:"enabled"`
	ListenAddr string `yaml:"listen_addr"`
	Upstream   string `yaml:"upstream"`
	RefreshSec int    `yaml:"refresh_sec"`
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
			AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:5173", "http://127.0.0.1:5173"},
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
			GatewayDomain: "",
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
	// Dev mode default: allow localhost CORS when not explicitly configured
	if len(cfg.Server.AllowedOrigins) == 0 {
		cfg.Server.AllowedOrigins = []string{"http://localhost:3000", "http://127.0.0.1:3000", "http://localhost:5173", "http://127.0.0.1:5173"}
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

	if err := os.WriteFile(path, data, 0600); err != nil {
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
// TLS behavior (in priority order):
//  1. Custom cert (tls_cert_file + tls_key_file) → manual TLS
//  2. Domain configured (gateway_domain) → automatic Let's Encrypt
//  3. Neither → HTTP only with security warning
//
// Generated by `aegis bootstrap --production`.
func (c *Config) PanelCaddyfile() string {
	domain := c.ManagedDomain.GatewayDomain
	addr := c.Server.Addr
	hasCustomCert := c.Proxy.TlsCertFile != "" && c.Proxy.TlsKeyFile != ""

	proxyBlock := func() string {
		return "\t# /api/* → Aegis API (authenticated)\n" +
			"\thandle /api/* {\n" +
			"\t\treverse_proxy " + addr + " {\n" +
			"\t\t\theader_up Host {host}\n" +
			"\t\t\theader_up X-Forwarded-For {remote_host}\n" +
			"\t\t\theader_up X-Forwarded-Proto {scheme}\n" +
			"\t\t}\n" +
			"\t}\n" +
			"\t# /* → Aegis embedded UI (SPA)\n" +
			"\thandle {\n" +
			"\t\treverse_proxy " + addr + " {\n" +
			"\t\t\theader_up Host {host}\n" +
			"\t\t\theader_up X-Forwarded-For {remote_host}\n" +
			"\t\t\theader_up X-Forwarded-Proto {scheme}\n" +
			"\t\t}\n" +
			"\t}\n"
	}

	hasTLS := domain != ""
	tlsLabel := "none (HTTP only)"
	tlsDirective := ""

	if hasCustomCert {
		tlsLabel = "custom certificate (" + c.Proxy.TlsCertFile + ")"
		tlsDirective = "\ttls " + c.Proxy.TlsCertFile + " " + c.Proxy.TlsKeyFile + "\n"
	} else if domain != "" {
		tlsLabel = "automatic Let's Encrypt"
	}

	var b strings.Builder
	b.WriteString("# Aegis Control Panel — auto-generated by aegis bootstrap\n")
	b.WriteString("# TLS: " + tlsLabel + "\n")

	if !hasTLS {
		b.WriteString("#\n")
		b.WriteString("# ═══════════════════════════════════════════════════════════════\n")
		b.WriteString("# WARNING: No domain or custom cert configured.\n")
		b.WriteString("# Admin credentials will be transmitted in plaintext over HTTP.\n")
		b.WriteString("#\n")
		b.WriteString("# To enable HTTPS:\n")
		b.WriteString("#   Option 1 (Let's Encrypt): point a domain to this server,\n")
		b.WriteString("#     then set gateway_domain in Settings UI.\n")
		b.WriteString("#   Option 2 (custom cert): upload cert + key, set\n")
		b.WriteString("#     tls_cert_file and tls_key_file in config.yaml.\n")
		b.WriteString("# ═══════════════════════════════════════════════════════════════\n")
	}
	b.WriteString("#\n")

	if hasTLS {
		b.WriteString("# Panel:  https://" + domain + "\n")
		b.WriteString("# API:    https://" + domain + "/api/*\n")
	} else {
		b.WriteString("# Panel:  http://<server-ip>\n")
		b.WriteString("# API:    http://<server-ip>/api/*\n")
	}
	b.WriteString("#\n")
	b.WriteString("# User-defined routes are appended below by Aegis Apply.\n\n")

	if hasTLS {
		b.WriteString(domain + " {\n")
		b.WriteString(tlsDirective)
		b.WriteString(proxyBlock())
		b.WriteString("}\n\n")
		b.WriteString("# Redirect HTTP → HTTPS\n")
		b.WriteString(":80 {\n")
		b.WriteString("\tredir https://{host}{uri} permanent\n")
		b.WriteString("}\n")
	} else {
		b.WriteString(":80 {\n")
		b.WriteString(proxyBlock())
		b.WriteString("}\n")
	}

	return b.String()
}

// generateAdminToken creates a cryptographically random 32-byte hex token.
// Delegates to core.GenerateRandomHex — the project's canonical random-hex generator.
func generateAdminToken() string {
	return core.GenerateRandomHex(32)
}
