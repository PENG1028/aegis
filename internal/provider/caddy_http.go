package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"aegis/internal/config"
	"aegis/internal/proxy"
	"aegis/internal/proxy/caddy"
)

// CaddyHTTPProvider wraps the Caddy adapter as an HTTP/HTTPS provider.
type CaddyHTTPProvider struct {
	cfg     *config.Config
	adapter *caddy.Adapter
}

// NewCaddyHTTPProvider creates a Caddy HTTP provider.
func NewCaddyHTTPProvider(cfg *config.Config) *CaddyHTTPProvider {
	return &CaddyHTTPProvider{
		cfg:     cfg,
		adapter: caddy.NewAdapter(cfg),
	}
}

func (p *CaddyHTTPProvider) Info() Info {
	status := "ready"
	msg := ""
	if _, err := exec.LookPath(p.cfg.Proxy.CaddyBinary); err != nil {
		status = "degraded"
		msg = fmt.Sprintf("caddy binary not found: %v", err)
	}
	return Info{
		Name:       "caddy_http",
		Protocol:   "http",
		Status:     status,
		Message:    msg,
		ConfigPath: p.cfg.Proxy.CaddyfilePath,
	}
}

func (p *CaddyHTTPProvider) Render(routes []proxy.RouteConfig) ([]byte, error) {
	cfg := proxy.GatewayConfig{
		Routes: routes,
		Email:  p.cfg.Proxy.Email,
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
	if err := os.MkdirAll(p.cfg.Proxy.BackupDir, 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
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
	return os.WriteFile(p.cfg.Proxy.CaddyfilePath, data, 0644)
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
	if err := os.WriteFile(tmpFile, rendered, 0644); err != nil {
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
	if err := os.WriteFile(p.cfg.Proxy.CaddyfilePath, data, 0644); err != nil {
		return err
	}
	os.Remove(tempPath)
	return nil
}

// renderCaddyfile is the shared Caddy renderer (from proxy/caddy package).
func renderCaddyfile(gwCfg proxy.GatewayConfig, email string) string {
	var buf bytes.Buffer
	if email != "" {
		buf.WriteString("{\n    email " + email + "\n}\n\n")
	}
	for _, r := range gwCfg.Routes {
		if r.MaintenanceEnabled {
			msg := r.MaintenanceMessage
			if msg == "" {
				msg = "Service temporarily unavailable"
			}
			msg = strings.ReplaceAll(msg, `"`, `\"`)
			buf.WriteString(fmt.Sprintf("%s {\n    respond \"%s\" 503\n}\n", r.Domain, msg))
		} else {
			buf.WriteString(fmt.Sprintf("%s {\n    encode gzip\n    reverse_proxy %s\n}\n", r.Domain, r.UpstreamURL))
		}
	}
	return buf.String()
}
