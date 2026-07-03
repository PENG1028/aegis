package provider

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"sort"
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

	// Determine port bindings
	mode := CurrentPortPolicyMode()
	if mode == "edge_mux" {
		state.Ports = []PortBinding{
			{Port: 80, Owner: "caddy", Protocol: "tcp", Purpose: "http", Status: "active"},
			{Port: 8443, Owner: "caddy", Protocol: "tcp", Purpose: "internal_https", Status: "active"},
		}
	} else {
		state.Ports = []PortBinding{
			{Port: 80, Owner: "caddy", Protocol: "tcp", Purpose: "http", Status: "active"},
			{Port: 443, Owner: "caddy", Protocol: "tcp", Purpose: "https", Status: "active"},
		}
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
// Caddyfile rendering (adapted from internal/proxy/caddy/render.go)
// ============================================================================

func (p *CaddyProvider) renderCaddyfile(plan Plan) []byte {
	var buf bytes.Buffer

	mode := CurrentPortPolicyMode()
	needGlobalBlock := p.email != "" || mode == "edge_mux"

	if needGlobalBlock {
		buf.WriteString("{\n")
		if p.email != "" {
			buf.WriteString("    email " + sanitizeCaddyValue(p.email) + "\n")
		}
		if mode == "edge_mux" {
			buf.WriteString("    https_port 8443\n")
		}
		buf.WriteString("}\n\n")
	}

	// Group routes by domain (Match.Host)
	domainRoutes := make(map[string][]RouteSpec)
	var domainOrder []string
	for _, r := range plan.Routes {
		if r.Match.Host == "" {
			continue
		}
		if _, ok := domainRoutes[r.Match.Host]; !ok {
			domainOrder = append(domainOrder, r.Match.Host)
		}
		domainRoutes[r.Match.Host] = append(domainRoutes[r.Match.Host], r)
	}

	for domainIdx, domain := range domainOrder {
		if domainIdx > 0 {
			buf.WriteString("\n")
		}
		siteAddr := caddySiteAddr(domain)
		routes := domainRoutes[domain]

		// Sort by path depth (longer paths first to match before shorter)
		sort.Slice(routes, func(i, j int) bool {
			di := len(strings.Split(strings.Trim(routes[i].Match.Path, "/"), "/"))
			dj := len(strings.Split(strings.Trim(routes[j].Match.Path, "/"), "/"))
			if routes[i].Match.Path == "" {
				return false
			}
			if routes[j].Match.Path == "" {
				return true
			}
			return di > dj
		})

		// Simple case: single route with no path prefix and no maintenance
		if len(routes) == 1 && routes[0].Match.Path == "" && !routes[0].MaintenanceEnabled {
			renderSingleRoute(&buf, routes[0], siteAddr)
			continue
		}

		buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(siteAddr)))
		hasDomainFallback := false

		for _, r := range routes {
			if r.MaintenanceEnabled {
				buf.WriteString(fmt.Sprintf("    handle %s {\n", pathPattern(r.Match.Path)))
				buf.WriteString(fmt.Sprintf("        respond \"%s\" 503\n", sanitizeCaddyValue(r.MaintenanceMessage)))
				buf.WriteString("    }\n")
				continue
			}
			if r.Match.Path != "" {
				pp := pathPattern(r.Match.Path)
				if r.StripPathPrefix {
					buf.WriteString(fmt.Sprintf("    handle_path %s {\n", pp))
				} else {
					buf.WriteString(fmt.Sprintf("    handle %s {\n", pp))
				}
				buf.WriteString("        encode gzip\n")
				writeReverseProxy(&buf, r.Upstream.Target, r.ExtraHeaders, "        ")
				buf.WriteString("    }\n")
			} else {
				hasDomainFallback = true
			}
		}

		if hasDomainFallback {
			for _, r := range routes {
				if r.Match.Path == "" && !r.MaintenanceEnabled {
					buf.WriteString("    handle {\n")
					buf.WriteString("        encode gzip\n")
					writeReverseProxy(&buf, r.Upstream.Target, r.ExtraHeaders, "        ")
					buf.WriteString("    }\n")
					break
				}
			}
		}
		buf.WriteString("}\n")
	}

	return buf.Bytes()
}

func renderSingleRoute(buf *bytes.Buffer, route RouteSpec, siteAddr string) {
	buf.WriteString(fmt.Sprintf("%s {\n", sanitizeCaddyValue(siteAddr)))
	buf.WriteString("    encode gzip\n")
	writeReverseProxy(buf, sanitizeCaddyValue(route.Upstream.Target), route.ExtraHeaders, "    ")
	buf.WriteString("}\n")
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
	return p.reload()
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

// ============================================================================
// Shared helper functions (inlined from proxy/caddy/render.go)
// ============================================================================

func sanitizeCaddyValue(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "{", "")
	s = strings.ReplaceAll(s, "}", "")
	return s
}

func pathPattern(p string) string {
	if p != "" && !strings.HasSuffix(p, "*") && !strings.HasSuffix(p, "/*") {
		return strings.TrimSuffix(p, "/") + "/*"
	}
	return p
}

func writeReverseProxy(buf *bytes.Buffer, upstream string, headers map[string]string, indent string) {
	safeUpstream := sanitizeCaddyValue(upstream)
	if len(headers) > 0 {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s {\n", indent, safeUpstream))
		for k, v := range headers {
			buf.WriteString(fmt.Sprintf("%s    header_up %s \"%s\"\n", indent, sanitizeCaddyValue(k), sanitizeCaddyValue(v)))
		}
		buf.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		buf.WriteString(fmt.Sprintf("%sreverse_proxy %s\n", indent, safeUpstream))
	}
}

func caddySiteAddr(domain string) string {
	if isInternalDomain(domain) {
		return "http://" + domain
	}
	return domain
}

func isInternalDomain(domain string) bool {
	return strings.HasSuffix(domain, ".internal") ||
		strings.HasSuffix(domain, ".local") ||
		strings.HasSuffix(domain, ".localhost")
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
		CapUpstreamTCP,
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
