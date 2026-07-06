package provider

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"aegis/internal/config"
)

// ============================================================================
// CaddyProvider — Provider implementation for Caddy v2+
// ============================================================================

// CaddyProvider implements the Provider interface for the Caddy web server.
// It generates Caddyfile configuration from Plan/RouteSpec and manages the
// full validate→backup→write→reload lifecycle.
type CaddyProvider struct {
	cfg        *config.Config
	binaryPath string // resolved absolute path to caddy binary
	email      string // ACME email from config
}

// NewCaddyProvider creates a Caddy Provider.
// Resolves the caddy binary path at construction time.
func NewCaddyProvider(cfg *config.Config) *CaddyProvider {
	bp := cfg.Proxy.CaddyBinary
	if resolved, err := exec.LookPath(bp); err == nil {
		bp = resolved
	}
	return &CaddyProvider{
		cfg:        cfg,
		binaryPath: bp,
		email:      cfg.Proxy.Email,
	}
}

// ============================================================================
// Provider interface implementation
// ============================================================================

// State returns the current runtime state of the Caddy provider.
func (p *CaddyProvider) State() ProviderState {
	state := ProviderState{
		ID:           "caddy",
		Name:         "Caddy HTTP",
		GatewayType:  TypeHTTPTerm,
		Status:       "unavailable",
		ConfigPath:   p.cfg.Proxy.CaddyfilePath,
		BinaryPath:   p.binaryPath,
		Capabilities: caddyCapabilities(),
	}

	// Check if installed
	if _, err := exec.LookPath(p.cfg.Proxy.CaddyBinary); err == nil {
		state.Installed = true
		state.Status = "degraded"
		if _, svcErr := exec.Command("systemctl", "is-active", "--quiet", "caddy").CombinedOutput(); svcErr == nil {
			state.Running = true
			state.Status = "ready"
		}
		// Get version
		if verOut, verErr := exec.Command(p.binaryPath, "version").CombinedOutput(); verErr == nil {
			state.Version = strings.TrimSpace(string(verOut))
		}
	}

	// Port allocations now come from RuntimeMode (the single source of truth).
	// ProviderState.Ports is display-only — the atom matrix shows mode-accurate ports.
	state.Ports = []PortBinding{
		{Port: 80, Owner: "caddy", Protocol: "tcp", Purpose: "http", Status: "active"},
	}

	return state
}

// Diagnose performs a full diagnostic check of the Caddy installation.
// Delegates to the shared quickDiagnoseCaddy with configured paths.
func (p *CaddyProvider) Diagnose() ProviderDiagnostic {
	diag := quickDiagnoseCaddy(p.cfg.Proxy.CaddyBinary, p.cfg.Proxy.CaddyfilePath)

	// Augment with runtime verify if the basic checks all passed
	if diag.LastErrorCode == "" {
		rtOK := p.runtimeVerify()
		diag.RuntimeVerifyOK = &rtOK
		if !rtOK {
			diag.LastErrorCode = DiagCodeRuntimeVerifyFailed
			diag.LastErrorMessage = "caddy runtime verify failed — gateway not responding on expected port"
		}
	}

	return diag
}

// Render generates a Caddyfile from a Plan and returns it as a ConfigFile.
func (p *CaddyProvider) Render(plan Plan) ([]ConfigFile, error) {
	content := p.renderCaddyfile(plan)
	return []ConfigFile{
		{
			Path:    p.cfg.Proxy.CaddyfilePath,
			Content: content,
		},
	}, nil
}

// Apply validates, backs up, writes, and reloads Caddy configuration.
func (p *CaddyProvider) Apply(configs []ConfigFile) error {
	if len(configs) == 0 {
		return fmt.Errorf("no config files to apply")
	}

	for _, cf := range configs {
		if err := p.applyOne(cf); err != nil {
			return fmt.Errorf("apply %s: %w", cf.Path, err)
		}
	}
	return nil
}

// ============================================================================
// Apply helpers
// ============================================================================

// applyOne writes a single config file with the full 6-step pipeline.
func (p *CaddyProvider) applyOne(cf ConfigFile) error {
	configPath := cf.Path
	if configPath == "" {
		configPath = p.cfg.Proxy.CaddyfilePath
	}

	// 1. Write to temp file
	tmpFile := configPath + ".tmp"
	if err := writeCaddyConfig(tmpFile, cf.Content); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	defer os.Remove(tmpFile)

	// 2. Validate temp file
	if err := p.validateConfig(tmpFile); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 3. Backup existing config
	if existing, err := os.ReadFile(configPath); err == nil {
		backupPath := configPath + ".bak"
		os.WriteFile(backupPath, existing, 0600)
	}

	// 4. Atomic replace
	if err := os.Rename(tmpFile, configPath); err != nil {
		// Fallback: read+write if rename fails (cross-filesystem)
		data, _ := os.ReadFile(tmpFile)
		if err := writeCaddyConfig(configPath, data); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	// 5. Reload
	if err := p.reload(); err != nil {
		// 6. Rollback — restore known-good config and retry reload
		backupPath := configPath + ".bak"
		if backupData, backupErr := os.ReadFile(backupPath); backupErr != nil {
			log.Printf("[caddy] rollback: read backup %s: %v", backupPath, backupErr)
		} else if writeErr := writeCaddyConfig(configPath, backupData); writeErr != nil {
			log.Printf("[caddy] rollback: write config %s: %v", configPath, writeErr)
		} else if reloadErr := p.reload(); reloadErr != nil {
			log.Printf("[caddy] rollback: reload after restore: %v", reloadErr)
		}
		return fmt.Errorf("reload failed (config restored from backup): %w", err)
	}
	return nil
}

func (p *CaddyProvider) validateConfig(configPath string) error {
	cmd := exec.Command(p.binaryPath, "validate", "--config", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy validate failed: %s\n%s", err.Error(), string(output))
	}
	return nil
}

func (p *CaddyProvider) reload() error {
	reloadCmd := p.cfg.Proxy.ReloadCommand
	if reloadCmd == "" {
		return nil
	}
	parts := strings.Fields(reloadCmd)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		hint := ""
		if strings.Contains(errMsg, "permission denied") || strings.Contains(errMsg, "Permission denied") {
			hint = "\nPermission denied. Try running with sudo or configure service permissions."
		}
		return fmt.Errorf("reload failed (cmd: %s): %s\nstderr: %s%s", reloadCmd, err.Error(), errMsg, hint)
	}
	return nil
}

func (p *CaddyProvider) runtimeVerify() bool {
	if _, err := exec.LookPath("curl"); err == nil {
		cmd := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}",
			"--connect-timeout", "3", "http://127.0.0.1:80")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false
		}
		code := strings.TrimSpace(string(out))
		return code == "200" || code == "308" || code == "301" || code == "302" || code == "404"
	}
	return true
}

// writeCaddyConfig writes data to a Caddyfile with 0640 root:caddy permissions.
func writeCaddyConfig(path string, data []byte) error {
	perm := os.FileMode(0640)
	if err := os.WriteFile(path, data, perm); err != nil {
		return err
	}
	if grp, err := user.LookupGroup("caddy"); err == nil {
		if gid, err := strconv.Atoi(grp.Gid); err == nil {
			os.Chown(path, 0, gid)
		}
	}
	os.Chmod(path, perm)
	return nil
}

// ============================================================================
// Capability declaration
// ============================================================================

// caddyCapabilities returns the static capability set for Caddy v2+.
func caddyCapabilities() []Capability {
	return []Capability{
		// L4
		CapListenTCP,
		CapListenUDP, // required for HTTP/3 QUIC on :443/udp
		CapUpstreamTCP,
		CapUpstreamUDP,
		// L5
		CapTLSTerminate,
		CapTLSMasquerade,
		// L6
		CapALPNMatch,
		CapProtoDetect,
		CapOCSPStapling,
		// L7 protocols
		CapHTTP1,
		CapHTTP2,
		CapHTTP3,
		CapWebSocket,
		CapGRPC,
		CapSSE,
		// L7 routing
		CapRouteHost,
		CapRoutePath,
		// L7 operational
		CapAutoCert,
		CapLoadCert,
		CapHealthCheck,
		CapLoadBalance,
		CapRateLimit,
		CapHotReload,
		CapValidateConfig,
	}
}

// Ensure CaddyProvider implements Provider + optional interfaces
var _ Provider = (*CaddyProvider)(nil)
var _ LifecycleProvider = (*CaddyProvider)(nil)
var _ ReloadableProvider = (*CaddyProvider)(nil)
var _ ConfigReader = (*CaddyProvider)(nil)

// ─── LifecycleProvider ──────────────────────────────────────────────────────

func (p *CaddyProvider) CanInstall() bool   { return true }
func (p *CaddyProvider) Install() error     { return installPackage("caddy", "caddy") }
func (p *CaddyProvider) CanUninstall() bool { return true }
func (p *CaddyProvider) Uninstall() error   { return uninstallPackage("caddy", "caddy") }

// ─── ReloadableProvider ─────────────────────────────────────────────────────

func (p *CaddyProvider) Reload() error { return p.reload() }

// ─── ConfigReader ───────────────────────────────────────────────────────────

func (p *CaddyProvider) GetCurrentConfig() (string, error) {
	data, err := os.ReadFile(p.cfg.Proxy.CaddyfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

