package provider

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// ============================================================================
// HAProxyProvider — unified Provider for HAProxy (SNI passthrough + TCP forwarding)
// ============================================================================

// HAProxyProvider implements the Provider interface for HAProxy.
// It manages both haproxy.cfg (TLS SNI passthrough on :443) and
// haproxy_tcp.cfg (raw TCP forwarding on dedicated ports) — but exposes
// a SINGLE Provider interface. Internal config file management is an
// implementation detail.
type HAProxyProvider struct {
	configPath   string // primary config: /etc/haproxy/haproxy.cfg
	tcpConfigPath string // TCP forwarding config: /etc/haproxy/haproxy_tcp.cfg
	backupDir    string
	binaryPath   string // resolved absolute path to haproxy binary
	inspectDelay string
}

// NewHAProxyProvider creates a unified HAProxy Provider.
func NewHAProxyProvider(configPath, tcpConfigPath, backupDir string) *HAProxyProvider {
	if configPath == "" {
		configPath = "/etc/haproxy/haproxy.cfg"
	}
	if tcpConfigPath == "" {
		tcpConfigPath = "/etc/haproxy/haproxy_tcp.cfg"
	}
	if backupDir == "" {
		backupDir = "/var/lib/aegis/haproxy-backups"
	}
	bp := "haproxy"
	if resolved, err := exec.LookPath("haproxy"); err == nil {
		bp = resolved
	}
	return &HAProxyProvider{
		configPath:    configPath,
		tcpConfigPath: tcpConfigPath,
		backupDir:     backupDir,
		binaryPath:    bp,
		inspectDelay:  "5s",
	}
}

// ReadConfig implements the optional Reader interface.
// Parses the current HAProxy config files and returns a structured snapshot.
func (p *HAProxyProvider) ReadConfig(ctx context.Context) (*ConfigSnapshot, error) {
	reader := &HAProxyReader{configPath: p.configPath, tcpConfigPath: p.tcpConfigPath}
	return reader.ReadConfig(ctx)
}

// ============================================================================
// Provider interface implementation
// ============================================================================

// State returns the current runtime state of the HAProxy provider.
func (p *HAProxyProvider) State() ProviderState {
	state := ProviderState{
		ID:           "haproxy",
		Name:         "HAProxy",
		GatewayType:  TypeSNIPass,
		Status:       "unavailable",
		ConfigPath:   p.configPath,
		BinaryPath:   p.binaryPath,
		Capabilities: haproxyCapabilities(),
	}

	if _, err := exec.LookPath("haproxy"); err == nil {
		state.Installed = true
		state.Status = "degraded"
		if _, svcErr := exec.Command("systemctl", "is-active", "--quiet", "haproxy").CombinedOutput(); svcErr == nil {
			state.Running = true
			state.Status = "ready"
		}
		if verOut, verErr := exec.Command(p.binaryPath, "-v").Output(); verErr == nil {
			state.Version = strings.TrimSpace(string(verOut))
		}
	}

	// Port allocations now come from RuntimeMode (the single source of truth).
	// HAProxy serves :443 in EdgeMux mode; in Legacy mode it may not be running at all.
		// Ready check: can start right now?
		if !state.Running {
			state.Ready = p.canStart()
			if !state.Ready {
				for _, iss := range p.startupIssues() {
					state.Issues = append(state.Issues, Issue{
						Code: iss.code, Message: iss.message, Detail: iss.detail,
					})
				}
			}
		} else {
			state.Ready = true
		}


	return state
}

// Diagnose performs a full diagnostic check of the HAProxy installation.
// Delegates to the shared quickDiagnoseHAProxy with configured paths.
func (p *HAProxyProvider) Diagnose() ProviderDiagnostic {
	return quickDiagnoseHAProxy(p.binaryPath, p.configPath)
}

// Render generates HAProxy configuration files from a Plan.
// Returns up to two config files: haproxy.cfg (SNI routing) and
// haproxy_tcp.cfg (TCP forwarding on dedicated ports).
func (p *HAProxyProvider) Render(plan Plan) ([]ConfigFile, error) {
	var configs []ConfigFile

	// Split routes: SNI passthrough (tls_passthrough) vs raw TCP forwarding
	var sniRoutes []RouteSpec
	var tcpRoutes []RouteSpec

	for _, r := range plan.Routes {
		if r.TLSMode == "passthrough" {
			sniRoutes = append(sniRoutes, r)
		} else if r.Transport == "tcp" && r.AppProtocol == "raw" {
			tcpRoutes = append(tcpRoutes, r)
		}
	}

	// Always generate the main haproxy.cfg (even if empty — it defines the :443 frontend)
	mainCfg := p.renderMainConfig(plan.Listeners, sniRoutes)
	configs = append(configs, ConfigFile{
		Path:    p.configPath,
		Content: mainCfg,
	})

	// Optionally generate haproxy_tcp.cfg for TCP forwarding
	if len(tcpRoutes) > 0 {
		tcpCfg := p.renderTCPConfig(plan.Listeners, tcpRoutes)
		configs = append(configs, ConfigFile{
			Path:    p.tcpConfigPath,
			Content: tcpCfg,
		})
	}

	return configs, nil
}

// Apply validates, backs up, writes, and reloads HAProxy configuration.
// If reload fails, all config files are restored from backup and reload is retried.
func (p *HAProxyProvider) Apply(configs []ConfigFile) error {
	if len(configs) == 0 {
		return fmt.Errorf("no config files to apply")
	}

	// Write all config files first
	for _, cf := range configs {
		if err := p.applyOne(cf); err != nil {
			return fmt.Errorf("apply %s: %w", cf.Path, err)
		}
	}

	// Reload once after all files are written
	if err := p.reload(); err != nil {
		// Rollback all configs from backup on reload failure
		for _, cf := range configs {
			backupPath := cf.Path + ".bak"
			if backupData, backupErr := os.ReadFile(backupPath); backupErr != nil {
				log.Printf("[haproxy] rollback: read backup %s: %v", backupPath, backupErr)
			} else if err := os.WriteFile(cf.Path, backupData, 0644); err != nil {
				log.Printf("[haproxy] rollback: write config %s: %v", cf.Path, err)
			}
		}
		if reloadErr := p.reload(); reloadErr != nil {
			log.Printf("[haproxy] rollback: reload after restore: %v", reloadErr)
		}
		return fmt.Errorf("reload failed (all configs restored from backup): %w", err)
	}

	return nil
}

// ============================================================================
// Apply helpers
// ============================================================================

func (p *HAProxyProvider) applyOne(cf ConfigFile) error {
	configPath := cf.Path

	// 1. Write to temp file
	tmpFile := configPath + ".tmp"
	if err := os.WriteFile(tmpFile, cf.Content, 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}
	defer os.Remove(tmpFile)

	// 2. Validate temp file
	if err := p.validateConfig(tmpFile); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	// 3. Backup existing config
	if existing, err := os.ReadFile(configPath); err == nil {
		os.WriteFile(configPath+".bak", existing, 0644)
	}

	// 4. Atomic replace
	if err := os.Rename(tmpFile, configPath); err != nil {
		data, _ := os.ReadFile(tmpFile)
		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
	}

	return nil
}

func (p *HAProxyProvider) validateConfig(configPath string) error {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil // no config to validate
	}
	cmd := exec.Command("haproxy", "-c", "-f", configPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("haproxy validate failed: %s\n%s", err.Error(), string(output))
	}
	return nil
}

func (p *HAProxyProvider) reload() error {
	cmd := exec.Command("systemctl", "reload", "haproxy")
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := string(output)
		// Fallback: haproxy -sf
		cmd2 := exec.Command("haproxy", "-f", p.configPath, "-sf", "$(pidof haproxy)")
		out2, err2 := cmd2.CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("haproxy reload failed (systemctl): %s\nstderr: %s\nhaproxy -sf also failed: %s",
				err.Error(), errMsg, string(out2))
		}
	}
	return nil
}

// ============================================================================
// Capability declaration
// ============================================================================

// haproxyCapabilities returns the static capability set for HAProxy.
func haproxyCapabilities() []Capability {
	return []Capability{
		// L4
		CapListenTCP,
		CapUpstreamTCP,
		// L5
		CapTLSPassthrough,
		CapTLSTerminate,
		CapMTLSTerminate,
		// L6
		CapSNIPreread,
		// L7 protocols
		CapHTTP1,
		CapRawTCP,
		// L7 operational
		CapLoadCert,
		CapHealthCheck,
		CapHotReload,
		CapValidateConfig,
	}
}

// Ensure HAProxyProvider implements Provider + optional interfaces
var _ Provider = (*HAProxyProvider)(nil)
var _ LifecycleProvider = (*HAProxyProvider)(nil)
var _ ReloadableProvider = (*HAProxyProvider)(nil)
var _ ConfigReader = (*HAProxyProvider)(nil)

// ─── LifecycleProvider ──────────────────────────────────────────────────────

func (p *HAProxyProvider) CanInstall() bool   { return true }
func (p *HAProxyProvider) Install() error     { return installPackage("haproxy", "haproxy") }
func (p *HAProxyProvider) CanUninstall() bool { return true }
func (p *HAProxyProvider) Uninstall() error   { return uninstallPackage("haproxy", "haproxy") }

// ─── ReloadableProvider ─────────────────────────────────────────────────────

func (p *HAProxyProvider) Reload() error { return p.reload() }

// ─── ConfigReader ───────────────────────────────────────────────────────────

func (p *HAProxyProvider) GetCurrentConfig() (string, error) {
	data, err := os.ReadFile(p.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

type haIssue struct{ code, message, detail string }

func (p *HAProxyProvider) canStart() bool { return len(p.startupIssues()) == 0 }

func (p *HAProxyProvider) startupIssues() []haIssue {
	var issues []haIssue
	if p.configPath != "" {
		if _, err := os.Stat(p.configPath); os.IsNotExist(err) {
			issues = append(issues, haIssue{
				code: "config_missing", message: "配置文件不存在",
				detail: p.configPath,
			})
		} else if out, err := exec.Command(p.binaryPath, "-c", "-f", p.configPath).CombinedOutput(); err != nil {
			issues = append(issues, haIssue{
				code: "config_invalid", message: "配置文件语法错误",
				detail: fmt.Sprintf("%v: %s", err, string(out)),
			})
		}
	}
	out, err := exec.Command("ss", "-tlnp", "sport", "=:443").CombinedOutput()
	if err == nil {
		line := string(out)
		if strings.Contains(line, ":443") && !strings.Contains(line, "haproxy") {
			issues = append(issues, haIssue{
				code: "port_conflict", message: "端口 :443 已被占用，需切换到 EdgeMux 模式",
				detail: strings.TrimSpace(line),
			})
		}
	}
	return issues
}
