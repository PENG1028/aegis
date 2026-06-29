package caddy

import (
	"aegis/internal/config"
	"aegis/internal/proxy"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Adapter implements proxy.ProxyAdapter for Caddy.
type Adapter struct {
	cfg         *config.Config
	caddyBinary string // resolved absolute path to caddy binary
}

// NewAdapter creates a new Caddy adapter.
// Resolves the caddy binary to an absolute path at construction time to prevent
// PATH-based attacks where a malicious caddy binary is placed earlier in PATH.
func NewAdapter(cfg *config.Config) *Adapter {
	caddyPath := cfg.Proxy.CaddyBinary
	if !filepath.IsAbs(caddyPath) {
		if resolved, err := exec.LookPath(caddyPath); err == nil {
			caddyPath = resolved
		}
		// If LookPath fails, keep the original value so the error surfaces
		// naturally when we try to execute it.
	}
	return &Adapter{cfg: cfg, caddyBinary: caddyPath}
}

// Name returns the adapter name.
func (a *Adapter) Name() string {
	return "caddy"
}

// Render generates Caddyfile content from a GatewayConfig.
func (a *Adapter) Render(gwCfg proxy.GatewayConfig) ([]byte, error) {
	output := renderCaddyfile(gwCfg, a.cfg.Proxy.Email)
	return []byte(output), nil
}

// Validate runs caddy validate on the given config file.
func (a *Adapter) Validate(configPath string) error {
	validateCmd := a.cfg.ResolveValidateCommand(configPath)
	if validateCmd == "" {
		// No validation command configured; skip
		return nil
	}

	// If it's a template, do simple substitution (use resolved binary path)
	validateCmd = strings.ReplaceAll(validateCmd, "{{config_path}}", configPath)
	validateCmd = strings.ReplaceAll(validateCmd, "{{caddy_binary}}", a.caddyBinary)

	parts := strings.Fields(validateCmd)
	if len(parts) == 0 {
		return nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy validate failed: %s\n%s", err.Error(), string(output))
	}
	return nil
}

// Reload executes the reload command to apply new configuration.
func (a *Adapter) Reload(command string) error {
	reloadCmd := command
	if reloadCmd == "" {
		reloadCmd = a.cfg.Proxy.ReloadCommand
	}

	if reloadCmd == "" {
		// No reload command; just log and succeed (development mode)
		fmt.Fprintf(os.Stderr, "warning: no reload command configured, skipping reload\n")
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
			hint = "\nPermission denied. Try running 'aegis apply' with sudo or configure service permissions."
		}
		return fmt.Errorf("reload failed (cmd: %s): %s\nstderr: %s%s", reloadCmd, err.Error(), errMsg, hint)
	}
	return nil
}
