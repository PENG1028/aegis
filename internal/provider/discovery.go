package provider

import (
	"fmt"
	"os/exec"
	"strings"
)

// ============================================================================
// Provider Discovery — detects installed gateway programs on a node.
//
// Discovery answers "what gateway types DOES this node support RIGHT NOW?"
// by checking which Provider binaries are installed and running.
//
// How discovery works:
//   1. Iterate all registered providers in the Registry.
//   2. For each external provider (Caddy, HAProxy), check if the binary exists.
//   3. For each detected binary, run Diagnose() to get version, config validity,
//      service running status.
//   4. Built-in providers (registered via RegisterBuiltin) are always "detected".
//
// The result is used by:
//   - Node capability matrix UI: shows which providers are available
//   - Agent heartbeat: gateway status is derived from discovery results
//   - Port policy computation: installed providers determine port allocation
// ============================================================================

// DiscoveredProvider is the result of detecting a single provider type on a node.
type DiscoveredProvider struct {
	// Identity
	ProviderID   string      `json:"provider_id"`
	ProviderName string      `json:"provider_name"`
	GatewayType  GatewayType `json:"gateway_type"`

	// Detection result
	Detected     bool   `json:"detected"`
	BinaryPath   string `json:"binary_path,omitempty"`
	Version      string `json:"version,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	ConfigValid  *bool  `json:"config_valid,omitempty"`

	// Runtime status
	ServiceRunning *bool  `json:"service_running,omitempty"`
	Running        bool   `json:"running"`
	StatusMessage  string `json:"status_message,omitempty"`

	// Port usage
	ListeningPorts []int `json:"listening_ports,omitempty"`

	// Capabilities (from provider state)
	Capabilities []Capability `json:"capabilities"`

	// Full diagnostic result (nil if provider not detected)
	Diagnostic *ProviderDiagnostic `json:"diagnostic,omitempty"`
}

// DiscoverProviders runs provider detection against all registered providers.
// External providers are checked for binary presence; built-in providers are
// always detected.
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

// isBuiltinProvider returns true for providers that run inside the Aegis process.
func isBuiltinProvider(id string) bool {
	switch id {
	case "aegis_tcp", "aegis_udp", "transparent":
		return true
	}
	return false
}

// discoverOne detects a single provider type on the local machine.
func discoverOne(p Provider) DiscoveredProvider {
	state := p.State()
	result := DiscoveredProvider{
		ProviderID:   state.ID,
		ProviderName: state.Name,
		GatewayType:  state.GatewayType,
		Capabilities: state.Capabilities,
	}

	// Built-in providers are always "detected" since they run inside Aegis
	if isBuiltinProvider(state.ID) {
		result.Detected = true
		result.Running = true
		result.StatusMessage = "内置于 Aegis，无需额外安装"
		return result
	}

	// External provider: run full diagnostic
	diag := p.Diagnose()
	result.Diagnostic = &diag

	if !diag.Installed {
		result.Detected = false
		result.Running = false
		result.StatusMessage = fmt.Sprintf("未检测到 %s 程序", state.Name)
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

	return result
}

// ============================================================================
// Port policy mode detection
// ============================================================================

// CurrentPortPolicyMode returns the active port policy mode ("legacy" or
// "edge_mux") by checking whether HAProxy is installed and running.
//
// Rules:
//   - HAProxy binary found AND haproxy service running → "edge_mux"
//   - Otherwise → "legacy"
func CurrentPortPolicyMode() string {
	if _, err := exec.LookPath("haproxy"); err != nil {
		return "legacy"
	}
	if err := exec.Command("systemctl", "is-active", "--quiet", "haproxy").Run(); err != nil {
		return "legacy"
	}
	return "edge_mux"
}

// ============================================================================
// System service detection
// ============================================================================

// SystemServices describes detected system-level services on a node.
type SystemServices struct {
	Systemd  bool `json:"systemd"`
	IPTables bool `json:"iptables"`
	SS       bool `json:"ss"`
	Netstat  bool `json:"netstat"`
}

// DetectSystemServices checks for the presence of system-level services
// relevant to gateway operation.
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

// CanTransparentProxy returns true if all requirements for transparent proxy are met.
func (s SystemServices) CanTransparentProxy() bool {
	return s.IPTables && s.Systemd
}

// ============================================================================
// HAProxy version parser
// ============================================================================

// ParseHAProxyVersion extracts major and minor version from HAProxy version output.
// Input format: "HAProxy version 2.4.22-1ubuntu1 ..."
// Returns (0, 0) if parsing fails.
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
