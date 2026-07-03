package provider

import (
	"fmt"
	"os/exec"
	"strings"
)

// ============================================================================
// Provider Discovery — detects installed gateway programs on a node.
//
// Discovery is the bridge between "what gateway types CAN this node support?"
// (static capabilities) and "what gateway types DOES this node support RIGHT NOW?"
// (runtime detection).
//
// How discovery works:
//   1. Iterate all registered providers in the Registry.
//   2. For each provider that CanInstall(), check if the binary exists in PATH.
//   3. For each detected binary, run Diagnose() to get version, config validity,
//      service running status, and listening ports.
//   4. Return a list of DiscoveredProvider — one per registered provider type,
//      with the detection result.
//
// The result is used by:
//   - Node capability matrix UI: shows which providers are available
//   - Agent heartbeat: gateway status is derived from discovery results
//   - Port policy computation: installed providers determine port allocation
//
// Usage:
//   results := DiscoverProviders(registry)
//   for _, r := range results {
//       if r.Detected {
//           fmt.Printf("%s %s is running\n", r.ProviderName, r.Version)
//       }
//   }
// ============================================================================

// DiscoveredProvider is the result of detecting a single provider type on a node.
type DiscoveredProvider struct {
	// Identity
	ProviderID   string      `json:"provider_id"`
	ProviderName string      `json:"provider_name"`
	GatewayType  GatewayType `json:"gateway_type"`

	// Detection result
	Detected     bool   `json:"detected"`      // binary found in PATH?
	BinaryPath   string `json:"binary_path,omitempty"`
	Version      string `json:"version,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	ConfigValid  *bool  `json:"config_valid,omitempty"`

	// Runtime status
	ServiceRunning *bool  `json:"service_running,omitempty"` // systemd service active?
	Running        bool   `json:"running"`                    // convenience: detected && service is up
	StatusMessage  string `json:"status_message,omitempty"`   // human-readable status

	// Port usage — what ports this provider is currently binding
	ListeningPorts []int `json:"listening_ports,omitempty"`

	// Can we install this provider if not detected?
	CanInstall bool `json:"can_install"`

	// If already registered as a gateway, this is the gateway ID
	GatewayID string `json:"gateway_id,omitempty"`

	// Capabilities (from the provider type, not instance)
	Capabilities ProviderCapabilities `json:"capabilities"`

	// Full diagnostic result (nil if provider not detected)
	Diagnostic *ProviderDiagnostic `json:"diagnostic,omitempty"`
}

// DiscoverProviders runs provider detection against all registered providers.
// Only providers with CanInstall() == true or built-in providers are checked.
// For each detected binary, a full diagnostic is run.
func DiscoverProviders(registry *Registry) []DiscoveredProvider {
	if registry == nil {
		return nil
	}

	all := registry.ListAll()
	results := make([]DiscoveredProvider, 0, len(all))

	for _, p := range all {
		result := discoverOne(p)
		results = append(results, result)
	}

	return results
}

// discoverOne detects a single provider type on the local machine.
func discoverOne(p Provider) DiscoveredProvider {
	result := DiscoveredProvider{
		ProviderID:   p.ID(),
		ProviderName: p.Name(),
		GatewayType:  p.Type(),
		Capabilities: p.Capabilities(),
		CanInstall:   p.CanInstall(),
	}

	// If the provider can't be installed and isn't built-in, skip binary detection.
	// Built-in providers (Aegis TCP/UDP) are always "detected" since they run
	// inside the Aegis process.
	if !p.CanInstall() {
		// Built-in: always available, mark as running if Aegis is running
		result.Detected = true
		result.Running = true
		result.StatusMessage = "内置于 Aegis，无需额外安装"
		result.ListeningPorts = getListeningPorts(p.ID())
		return result
	}

	// External provider: check if binary exists
	diag := p.Diagnose()
	result.Diagnostic = &diag

	if !diag.Installed {
		result.Detected = false
		result.Running = false
		result.StatusMessage = fmt.Sprintf("未检测到 %s 程序", p.Name())
		return result
	}

	// Binary found — populate detection details
	result.Detected = true
	result.BinaryPath = diag.BinaryPath
	result.Version = diag.Version
	result.ConfigPath = diag.ConfigPath
	result.ConfigValid = diag.ConfigValid
	result.ServiceRunning = diag.ServiceRunning

	running := diag.ServiceRunning != nil && *diag.ServiceRunning
	result.Running = running

	if running {
		result.StatusMessage = "运行中"
	} else {
		result.StatusMessage = "已安装但未运行"
	}

	result.ListeningPorts = getListeningPorts(p.ID())

	return result
}

// getListeningPorts returns the ports a given provider is expected to bind,
// based on the current port policy. This is a heuristic — actual port usage
// requires ss/netstat which may need root.
//
// Ports per provider:
//
//	caddy (legacy):      80, 443
//	caddy (edge_mux):    80, 8443
//	haproxy_edge_mux:   443
//	aegis_tcp:          varies (whatever exposures are active)
//	aegis_udp:          varies
func getListeningPorts(providerID string) []int {
	switch providerID {
	case "caddy":
		// Check if HAProxy is installed to determine mode
		if _, err := exec.LookPath("haproxy"); err == nil {
			// EdgeMux mode: Caddy only has 80 + 8443
			return []int{80, 8443}
		}
		// Legacy mode: Caddy has 80 + 443
		return []int{80, 443}
	case "haproxy_edge_mux":
		return []int{443}
	case "haproxy_tcp":
		// HAProxy TCP ports are configured per-exposure, can't predict statically
		return nil
	case "aegis_tcp":
		// Aegis TCP Manager ports are dynamic
		return nil
	case "aegis_udp":
		return nil
	case "aegis_transparent":
		// Transparent proxy uses local ports in 18100-18199 range
		return nil
	default:
		return nil
	}
}

// ============================================================================
// Port policy computation — determines port allocation based on installed providers
// ============================================================================

// ComputePortPolicy determines the active port allocation strategy based on
// which providers are detected on the node.
//
// Rules:
//   - If HAProxy EdgeMux is installed AND running → EdgeMux mode
//   - Otherwise → Legacy mode (Caddy owns both :80 and :443)
//
// In EdgeMux mode:
//   - HAProxy takes :443 (TLS SNI passthrough)
//   - Caddy moves from :443 to :8443 (internal HTTPS for TLS termination)
//   - Caddy keeps :80 (HTTP)
func ComputePortPolicy(discovered []DiscoveredProvider) PortPolicy {
	hasHAProxy := false
	haproxyRunning := false

	for _, d := range discovered {
		if d.ProviderID == "haproxy_edge_mux" {
			hasHAProxy = d.Detected
			haproxyRunning = d.Running
		}
	}

	if hasHAProxy && haproxyRunning {
		return DefaultEdgeMuxPortPolicy()
	}
	return DefaultLegacyPortPolicy()
}

// CurrentPortPolicyMode returns the active port policy mode string ("legacy" or
// "edge_mux") by checking whether HAProxy is installed and running on this node.
//
// Unlike ComputePortPolicy(), this function does NOT require a provider registry
// or a full discovery scan. It only checks for the HAProxy binary and service,
// which is sufficient to determine the mode.
//
// This is the lightweight entry point used by config rendering to inject the
// correct https_port into the generated Caddyfile.
//
// Rules:
//   - HAProxy binary found AND haproxy service running → "edge_mux"
//   - Otherwise → "legacy"
func CurrentPortPolicyMode() string {
	// Check if HAProxy binary exists
	if _, err := exec.LookPath("haproxy"); err != nil {
		return "legacy"
	}

	// HAProxy exists — check if it's actually running.
	// The EdgeMux mode only activates when HAProxy is actively managing :443.
	// If the binary is installed but the service is stopped, we stay in legacy
	// mode so Caddy continues to handle :443 (no outage from partial install).
	if err := exec.Command("systemctl", "is-active", "--quiet", "haproxy").Run(); err != nil {
		return "legacy"
	}

	return "edge_mux"
}

// DetectSystemServices checks for the presence of system-level services that
// are relevant to gateway operation but aren't themselves providers.
// Currently checks: systemd, iptables, ss/netstat.
func DetectSystemServices() SystemServices {
	svc := SystemServices{}

	if _, err := exec.LookPath("systemctl"); err == nil {
		svc.Systemd = true
	}
	if _, err := exec.LookPath("iptables"); err == nil {
		svc.IPTables = true
	}
	if _, err := exec.LookPath("ss"); err == nil {
		svc.SS = true
	} else if _, err := exec.LookPath("netstat"); err == nil {
		svc.Netstat = true
	}

	return svc
}

// SystemServices describes detected system-level services on a node.
type SystemServices struct {
	Systemd bool `json:"systemd"`  // systemctl available
	IPTables bool `json:"iptables"` // iptables available (needed for transparent proxy)
	SS      bool `json:"ss"`       // ss command available (port scanning)
	Netstat bool `json:"netstat"`  // netstat fallback if ss unavailable
}

// CanTransparentProxy returns true if all requirements for transparent proxy
// are met: Linux + iptables + root/sudo.
func (s SystemServices) CanTransparentProxy() bool {
	return s.IPTables && s.Systemd
}

// ============================================================================
// HAProxy version parser — canonical implementation
// ============================================================================

// ParseHAProxyVersion extracts major and minor version from HAProxy version output.
// Input format: "HAProxy version 2.4.22-1ubuntu1 ..."
// Returns (0, 0) if parsing fails.
//
// This is the SINGLE canonical version parser for HAProxy. All other HAProxy
// version parsing code (diagnostics.go, haproxy_tcp.go) must use this function.
// If you find another HAProxy version parser in the codebase, it is a bug.
func ParseHAProxyVersion(version string) (major, minor int) {
	fields := strings.Fields(version)
	for i, f := range fields {
		if f == "version" && i+1 < len(fields) {
			ver := strings.TrimRight(fields[i+1], ",")
			parts := strings.Split(ver, ".")
			if len(parts) >= 2 {
				fmt.Sscanf(parts[0], "%d", &major)
				fmt.Sscanf(parts[1], "%d", &minor)
			}
			return
		}
	}
	// Fallback: try parsing directly as "X.Y.Z..."
	parts := strings.Split(version, ".")
	if len(parts) >= 2 {
		fmt.Sscanf(parts[0], "%d", &major)
		fmt.Sscanf(parts[1], "%d", &minor)
	}
	return
}
