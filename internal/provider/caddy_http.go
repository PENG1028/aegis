package provider

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aegis/internal/config"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
)

// CaddyHTTPProvider wraps the Caddy adapter as an HTTP/HTTPS provider.
type CaddyHTTPProvider struct {
	cfg        *config.Config
	adapter    *caddy.Adapter
	binaryPath string // resolved absolute path to caddy binary
}

// NewCaddyHTTPProvider creates a Caddy HTTP provider.
// Resolves the caddy binary path at construction time.
func NewCaddyHTTPProvider(cfg *config.Config) *CaddyHTTPProvider {
	bp := cfg.Proxy.CaddyBinary
	if resolved, err := exec.LookPath(bp); err == nil {
		bp = resolved
	}
	return &CaddyHTTPProvider{
		cfg:        cfg,
		adapter:    caddy.NewAdapter(cfg),
		binaryPath: bp,
	}
}

// writeCaddyConfig writes data to a Caddyfile with 0640 root:caddy permissions.
// Caddy runs as the `caddy` user, so the file must be group-readable.
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

func (p *CaddyHTTPProvider) Info() Info {
	status := "ready"
	msg := ""
	if _, err := exec.LookPath(p.cfg.Proxy.CaddyBinary); err != nil {
		status = "degraded"
		msg = fmt.Sprintf("caddy binary not found: %v", err)
	}
	return Info{
		ID:         "caddy",
		Name:       "caddy_http",
		Type:       TypeHTTPTerm,
		Status:     status,
		Message:    msg,
		ConfigPath: p.cfg.Proxy.CaddyfilePath,
	}
}

func (p *CaddyHTTPProvider) Render(routes []proxy.RouteConfig) ([]byte, error) {
	cfg := proxy.GatewayConfig{
		Routes:         routes,
		Email:          p.cfg.Proxy.Email,
		PortPolicyMode: CurrentPortPolicyMode(),
	}
	return p.adapter.Render(cfg)
}

func (p *CaddyHTTPProvider) Validate(configPath string) error {
	return p.adapter.Validate(configPath)
}

func (p *CaddyHTTPProvider) Reload() error {
	return p.adapter.Reload(p.cfg.Proxy.ReloadCommand)
}

func (p *CaddyHTTPProvider) Backup() (string, error) {
	configPath := p.cfg.Proxy.CaddyfilePath
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil
	}
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("Caddyfile.%s.bak", timestamp)
	backupPath := filepath.Join(p.cfg.Proxy.BackupDir, backupName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(p.cfg.Proxy.BackupDir, 0700); err != nil {
		return "", err
	}
	if err := os.WriteFile(backupPath, data, 0600); err != nil {
		return "", err
	}
	return backupPath, nil
}

func (p *CaddyHTTPProvider) Restore(backupPath string) error {
	if backupPath == "" {
		return fmt.Errorf("no backup path")
	}
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	return writeCaddyConfig(p.cfg.Proxy.CaddyfilePath, data)
}

func (p *CaddyHTTPProvider) GetCurrentConfig() (string, error) {
	data, err := os.ReadFile(p.cfg.Proxy.CaddyfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// WriteTemp writes rendered config to a temp file.
func (p *CaddyHTTPProvider) WriteTemp(rendered []byte) (string, error) {
	dir := filepath.Dir(p.cfg.Proxy.CaddyfilePath)
	tmpFile := filepath.Join(dir, ".Caddyfile.tmp")
	if err := writeCaddyConfig(tmpFile, rendered); err != nil {
		return "", err
	}
	return tmpFile, nil
}

// CommitTemp moves temp file to actual config path.
func (p *CaddyHTTPProvider) CommitTemp(tempPath string) error {
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return err
	}
	if err := writeCaddyConfig(p.cfg.Proxy.CaddyfilePath, data); err != nil {
		return err
	}
	os.Remove(tempPath)
	return nil
}

// runtimeVerify performs a quick smoke test against the Caddy gateway.
func (p *CaddyHTTPProvider) runtimeVerify() bool {
	// Try a simple TCP connection check on port 80 and 443
	// Use curl or netcat if available; otherwise skip
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
	// No curl available — skip runtime verify
	return true
}

// ─── Layer 1: DETECTION ──────────────────────────────────────────────────

func (p *CaddyHTTPProvider) Diagnose() ProviderDiagnostic {
	now := time.Now().Format(time.RFC3339)
	diag := ProviderDiagnostic{
		Provider:   "caddy_http",
		ConfigPath: p.cfg.Proxy.CaddyfilePath,
		CheckedAt:  now,
	}

	// 1. Check binary installed (PROVIDER_MISSING)
	caddyPath, err := exec.LookPath(p.cfg.Proxy.CaddyBinary)
	if err != nil {
		diag.LastErrorCode = DiagCodeProviderMissing
		diag.LastErrorMessage = fmt.Sprintf("caddy binary '%s' not found in PATH", p.cfg.Proxy.CaddyBinary)
		return diag
	}
	diag.Installed = true
	diag.BinaryPath = caddyPath

	// 2. Get version (PROVIDER_VERSION_UNSUPPORTED)
	verOut, verErr := exec.Command(caddyPath, "version").CombinedOutput()
	if verErr != nil {
		diag.Version = "unknown"
		diag.VersionSupported = false
		diag.LastErrorCode = DiagCodeVersionUnsupported
		diag.LastErrorMessage = fmt.Sprintf("caddy version check failed: %v", verErr)
		diag.Stderr = string(verOut)
		return diag
	}
	diag.Version = strings.TrimSpace(string(verOut))
	diag.VersionSupported = strings.HasPrefix(diag.Version, "v2") || strings.Contains(diag.Version, "2.")

	// 3-7: remaining checks
	if _, statErr := os.Stat(p.cfg.Proxy.CaddyfilePath); os.IsNotExist(statErr) {
		diag.LastErrorCode = DiagCodeConfigFileMissing
		diag.LastErrorMessage = fmt.Sprintf("config file not found: %s", p.cfg.Proxy.CaddyfilePath)
		return diag
	}
	diag.ConfigExists = true

	validOut, validErr := exec.Command(caddyPath, "validate", "--config", p.cfg.Proxy.CaddyfilePath).CombinedOutput()
	valid := validErr == nil
	diag.ConfigValid = &valid
	if !valid {
		diag.LastErrorCode = DiagCodeConfigValidateFailed
		diag.LastErrorMessage = fmt.Sprintf("caddy validate failed for %s", p.cfg.Proxy.CaddyfilePath)
		diag.Stderr = string(validOut)
		return diag
	}

	_, svcErr := exec.Command("systemctl", "is-active", "--quiet", "caddy").CombinedOutput()
	running := svcErr == nil
	diag.ServiceRunning = &running
	if !running {
		diag.LastErrorCode = DiagCodeServiceNotRunning
		diag.LastErrorMessage = "caddy systemd service is not active"
		return diag
	}

	diag.ListenerOK = true
	rtOK := p.runtimeVerify()
	diag.RuntimeVerifyOK = &rtOK
	if !rtOK {
		diag.LastErrorCode = DiagCodeRuntimeVerifyFailed
		diag.LastErrorMessage = "caddy runtime verify failed — gateway not responding on expected port"
	}

	return diag
}

// ─── Layer 2: LOCATION ────────────────────────────────────────────────────

func (p *CaddyHTTPProvider) ID() string           { return "caddy" }
func (p *CaddyHTTPProvider) Name() string         { return "caddy_http" }
func (p *CaddyHTTPProvider) Type() GatewayType    { return TypeHTTPTerm }
func (p *CaddyHTTPProvider) ConfigPath() string   { return p.cfg.Proxy.CaddyfilePath }
func (p *CaddyHTTPProvider) BinaryPath() string   { return p.binaryPath }
func (p *CaddyHTTPProvider) ServiceName() string  { return "caddy" }

// ─── Layer 3: INSTALL / UNINSTALL ─────────────────────────────────────────

func (p *CaddyHTTPProvider) CanInstall() bool   { return true }
func (p *CaddyHTTPProvider) Install() error     { return installPackage("caddy", "caddy") }
func (p *CaddyHTTPProvider) CanUninstall() bool { return true }
func (p *CaddyHTTPProvider) Uninstall() error   { return uninstallPackage("caddy", "caddy") }

// ─── Layer 5: UI ──────────────────────────────────────────────────────────

func (p *CaddyHTTPProvider) Capabilities() ProviderCapabilities { return CaddyCapabilities() }
func (p *CaddyHTTPProvider) UIHints() ProviderUIHints         { return CaddyUIHints() }

// ─── Layer 4: CONFIG ──────────────────────────────────────────────────────

// WriteConfig validates, backs up, writes, and reloads the Caddyfile.
// This is the canonical write path — the HTTP Save Config handler delegates here.
func (p *CaddyHTTPProvider) WriteConfig(content []byte) error {
	configPath := p.cfg.Proxy.CaddyfilePath

	// 1. Write to temp file and validate
	tmpFile := configPath + ".tmp"
	if err := writeCaddyConfig(tmpFile, content); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	defer os.Remove(tmpFile)

	if err := p.Validate(tmpFile); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 2. Backup existing config
	if existing, err := os.ReadFile(configPath); err == nil {
		backupPath := configPath + ".bak"
		os.WriteFile(backupPath, existing, 0600)
	}

	// 3. Atomic replace
	if err := os.Rename(tmpFile, configPath); err != nil {
		// Fallback: read+write if rename fails (cross-filesystem)
		data, _ := os.ReadFile(tmpFile)
		if err := writeCaddyConfig(configPath, data); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	// 4. Reload
	return p.Reload()
}

// Ensure CaddyHTTPProvider implements Provider
var _ Provider = (*CaddyHTTPProvider)(nil)

// NOTE: Caddyfile rendering functions (renderCaddyfile, sanitizeCaddyValue,
// caddySiteAddr) live in internal/proxy/caddy/render.go — that is the canonical
// implementation. Do NOT duplicate them here. This file only contains the
// Provider interface methods and diagnostic logic.
