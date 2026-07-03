package fake

import (
	"fmt"

	"aegis/internal/provider"
	"aegis/internal/proxy"
)

// FakeProvider implements the Provider interface for testing failure scenarios.
type FakeProvider struct {
	ProviderName string // renamed from "Name" to avoid conflict with Provider.Name() method
	Protocol     string
	ProvConfigPath string // renamed from "ConfigPath" to avoid conflict with Provider.ConfigPath() method

	// Failure injection matrix (v1.7R enhanced)
	MissingBinary          bool   // → Info() returns "unavailable", Diagnose() returns PROVIDER_MISSING
	FailValidate           bool   // → Validate returns CONFIG_VALIDATE_FAILED
	ValidateErr            string // stderr content for validate failure
	FailReload             bool   // → Reload returns SERVICE_NOT_RUNNING
	ReloadErr              string
	FailBackup             bool
	FailRestore            bool
	RuntimeVerifyFailed    bool   // → runtime verify failure
	RuntimeVerifyErr       string
	ListenerConflict       bool   // → LISTENER_CONFLICT
	ListenerConflictDetail string
	ConfigFileMissing      bool   // → CONFIG_FILE_MISSING
	VersionUnsupported     bool   // → PROVIDER_VERSION_UNSUPPORTED
	Installed              bool
	Version                string
	Running                bool
}

// NewFakeProvider creates a fake provider with defaults.
func NewFakeProvider(name, protocol string) *FakeProvider {
	return &FakeProvider{
		ProviderName:   name,
		Protocol:       protocol,
		Installed:      true,
		Version:        "1.0.0",
		Running:        true,
		ProvConfigPath: "/tmp/fake-config.conf",
	}
}

func (fp *FakeProvider) Info() provider.Info {
	status := "ready"
	msg := ""
	if !fp.Installed || fp.MissingBinary {
		status = "unavailable"
		msg = fmt.Sprintf("%s: binary not found", provider.DiagCodeProviderMissing)
	} else if fp.VersionUnsupported {
		status = "degraded"
		msg = fmt.Sprintf("%s: version %s is unsupported", provider.DiagCodeVersionUnsupported, fp.Version)
	}
	return provider.Info{
		ID:         fp.ProviderName,
		Name:       fp.ProviderName,
		Type:       provider.TypeHTTPTerm, // fake; real providers return their actual type
		Status:     status,
		Message:    msg,
		ConfigPath: fp.ProvConfigPath,
	}
}

// ID implements provider.Provider.
func (fp *FakeProvider) ID() string { return fp.ProviderName }

// Name implements provider.Provider (conflicts with field, so method returns field value).
func (fp *FakeProvider) Name() string { return fp.ProviderName }

// Type implements provider.Provider.
func (fp *FakeProvider) Type() provider.GatewayType { return provider.TypeHTTPTerm }

// Capabilities implements provider.Provider.
func (fp *FakeProvider) Capabilities() provider.ProviderCapabilities {
	return provider.CaddyCapabilities() // fake: use Caddy capabilities as default
}

// UIHints implements provider.Provider.
func (fp *FakeProvider) UIHints() provider.ProviderUIHints {
	return provider.CaddyUIHints()
}

// CanInstall implements provider.Provider.
func (fp *FakeProvider) CanInstall() bool { return true }

// Install implements provider.Provider.
func (fp *FakeProvider) Install() error { return nil }

func (fp *FakeProvider) Render(routes []proxy.RouteConfig) ([]byte, error) {
	return []byte("# fake rendered config\n"), nil
}

func (fp *FakeProvider) Validate(configPath string) error {
	if fp.ConfigFileMissing {
		return fmt.Errorf("%s: config file not found at %s", provider.DiagCodeConfigFileMissing, configPath)
	}
	if fp.FailValidate {
		errMsg := fp.ValidateErr
		if errMsg == "" {
			errMsg = "syntax error at line 1"
		}
		return fmt.Errorf("%s: %s", provider.DiagCodeConfigValidateFailed, errMsg)
	}
	if fp.ListenerConflict {
		detail := fp.ListenerConflictDetail
		if detail == "" {
			detail = "port 443 already in use"
		}
		return fmt.Errorf("%s: %s", provider.DiagCodeListenerConflict, detail)
	}
	return nil
}

func (fp *FakeProvider) Reload() error {
	if fp.FailReload {
		errMsg := fp.ReloadErr
		if errMsg == "" {
			errMsg = "service not running"
		}
		return fmt.Errorf("%s: %s", provider.DiagCodeServiceNotRunning, errMsg)
	}
	if fp.RuntimeVerifyFailed {
		errMsg := fp.RuntimeVerifyErr
		if errMsg == "" {
			errMsg = "health check returned 502"
		}
		return fmt.Errorf("%s: %s", provider.DiagCodeRuntimeVerifyFailed, errMsg)
	}
	return nil
}

func (fp *FakeProvider) Backup() (string, error) {
	if fp.FailBackup {
		return "", fmt.Errorf("backup failed")
	}
	return "/tmp/fake-backup.bak", nil
}

func (fp *FakeProvider) Restore(backupPath string) error {
	if fp.FailRestore {
		return fmt.Errorf("restore failed")
	}
	return nil
}

func (fp *FakeProvider) GetCurrentConfig() (string, error) {
	if fp.ConfigFileMissing {
		return "", fmt.Errorf("%s: %s", provider.DiagCodeConfigFileMissing, fp.ProvConfigPath)
	}
	return "# fake current config\n", nil
}

// Diagnose implements the provider.Diagnoser interface.
func (fp *FakeProvider) Diagnose() provider.ProviderDiagnostic {
	diag := provider.ProviderDiagnostic{
		Provider:         fp.ProviderName,
		Installed:        fp.Installed && !fp.MissingBinary,
		BinaryPath:       "/usr/bin/" + fp.ProviderName,
		Version:          fp.Version,
		VersionSupported: !fp.VersionUnsupported,
		ConfigPath:       fp.ProvConfigPath,
		ConfigExists:     !fp.ConfigFileMissing,
		ListenerOK:       !fp.ListenerConflict,
	}

	if !fp.Installed || fp.MissingBinary {
		diag.LastErrorCode = provider.DiagCodeProviderMissing
		diag.LastErrorMessage = fmt.Sprintf("%s binary not found in PATH", fp.ProviderName)
		diag.Stderr = ""
		return diag
	}

	if fp.VersionUnsupported {
		diag.LastErrorCode = provider.DiagCodeVersionUnsupported
		diag.LastErrorMessage = fmt.Sprintf("version %s is below minimum required", fp.Version)
		return diag
	}

	if fp.ConfigFileMissing {
		diag.LastErrorCode = provider.DiagCodeConfigFileMissing
		diag.LastErrorMessage = fmt.Sprintf("config file not found: %s", fp.ProvConfigPath)
		return diag
	}

	if fp.FailValidate {
		valid := false
		diag.ConfigValid = &valid
		diag.LastErrorCode = provider.DiagCodeConfigValidateFailed
		diag.LastErrorMessage = fp.ValidateErr
		diag.Stderr = fmt.Sprintf("Error: %s", fp.ValidateErr)
		return diag
	}

	valid := true
	diag.ConfigValid = &valid

	if fp.ListenerConflict {
		diag.LastErrorCode = provider.DiagCodeListenerConflict
		diag.LastErrorMessage = fp.ListenerConflictDetail
		diag.ListenerOK = false
		return diag
	}

	if !fp.Running {
		running := false
		diag.ServiceRunning = &running
		diag.LastErrorCode = provider.DiagCodeServiceNotRunning
		diag.LastErrorMessage = "service is not running"
		return diag
	}

	running := true
	diag.ServiceRunning = &running

	if fp.RuntimeVerifyFailed {
		diag.LastErrorCode = provider.DiagCodeRuntimeVerifyFailed
		diag.LastErrorMessage = fp.RuntimeVerifyErr
		diag.Stderr = fp.RuntimeVerifyErr
		return diag
	}

	diag.ListenerOK = true
	return diag
}

// ResetErrors clears all failure injection flags.
func (fp *FakeProvider) ResetErrors() {
	fp.MissingBinary = false
	fp.FailValidate = false
	fp.ValidateErr = ""
	fp.FailReload = false
	fp.ReloadErr = ""
	fp.FailBackup = false
	fp.FailRestore = false
	fp.RuntimeVerifyFailed = false
	fp.RuntimeVerifyErr = ""
	fp.ListenerConflict = false
	fp.ListenerConflictDetail = ""
	fp.ConfigFileMissing = false
	fp.VersionUnsupported = false
	fp.Installed = true
	fp.Running = true
}

// ─── Layer 2: LOCATION ────────────────────────────────────────────────────

func (fp *FakeProvider) ConfigPath() string  { return fp.ProvConfigPath }
func (fp *FakeProvider) BinaryPath() string  { return "/usr/bin/" + fp.ProviderName }
func (fp *FakeProvider) ServiceName() string { return fp.ProviderName }

// ─── Layer 3: INSTALL / UNINSTALL ─────────────────────────────────────────

func (fp *FakeProvider) CanUninstall() bool { return true }
func (fp *FakeProvider) Uninstall() error   { return nil }

// ─── Layer 4: CONFIG ──────────────────────────────────────────────────────

func (fp *FakeProvider) WriteConfig(content []byte) error {
	if fp.FailBackup {
		return fmt.Errorf("backup failed")
	}
	if fp.FailReload {
		return fmt.Errorf("%s: %s", provider.DiagCodeServiceNotRunning, fp.ReloadErr)
	}
	return nil
}

// Ensure FakeProvider implements Provider
var _ provider.Provider = (*FakeProvider)(nil)

// FakeCluster simulates multi-node cluster scenarios for testing.
type FakeCluster struct {
	Nodes           []FakeNode
	ACKTimeout      bool
	SplitBrain      bool
	NoLeader        bool
	VersionMismatch bool // v1.7R: version mismatch across nodes
}

// FakeNode represents a simulated node in a cluster.
type FakeNode struct {
	NodeID       string
	IsLeader     bool
	IsCurrent    bool
	StateVersion uint64
	IsStale      bool
	LastSeen     string
	LocalIP      string
}

// NewFakeCluster creates a fake cluster with default healthy state.
func NewFakeCluster(nodeCount int) *FakeCluster {
	fc := &FakeCluster{}
	for i := 0; i < nodeCount; i++ {
		fc.Nodes = append(fc.Nodes, FakeNode{
			NodeID:       fmt.Sprintf("node-%d", i+1),
			IsCurrent:    i == 0,
			IsLeader:     i == 0,
			StateVersion: 100,
			LastSeen:     "2026-01-01T00:00:00Z",
			LocalIP:      "10.0.0.1",
		})
	}
	return fc
}

// GetLeader returns the leader node or nil.
func (fc *FakeCluster) GetLeader() *FakeNode {
	for i := range fc.Nodes {
		if fc.Nodes[i].IsLeader {
			return &fc.Nodes[i]
		}
	}
	return nil
}

// GetCurrent returns the current (local) node.
func (fc *FakeCluster) GetCurrent() *FakeNode {
	for i := range fc.Nodes {
		if fc.Nodes[i].IsCurrent {
			return &fc.Nodes[i]
		}
	}
	return nil
}

// InjectDrift creates a version gap on a non-leader node.
func (fc *FakeCluster) InjectDrift(nodeIndex int, versionDiff uint64) {
	if nodeIndex < len(fc.Nodes) {
		leader := fc.GetLeader()
		if leader != nil {
			fc.Nodes[nodeIndex].StateVersion = leader.StateVersion - versionDiff
		}
	}
}

// InjectSplitBrain sets a second node as leader.
func (fc *FakeCluster) InjectSplitBrain() {
	fc.SplitBrain = true
	if len(fc.Nodes) >= 2 {
		fc.Nodes[1].IsLeader = true
	}
}

// InjectStaleNode marks a node as stale.
func (fc *FakeCluster) InjectStaleNode(nodeIndex int) {
	if nodeIndex < len(fc.Nodes) {
		fc.Nodes[nodeIndex].IsStale = true
		fc.Nodes[nodeIndex].LastSeen = "2025-01-01T00:00:00Z"
	}
}

// InjectVersionMismatch creates version differences across all nodes (v1.7R).
func (fc *FakeCluster) InjectVersionMismatch() {
	fc.VersionMismatch = true
	for i := range fc.Nodes {
		fc.Nodes[i].StateVersion = uint64(100 - i*10)
	}
}

// CheckDrift reports a drift severity between two nodes.
func (fc *FakeCluster) CheckDrift(nodeA, nodeB int) string {
	vA := fc.Nodes[nodeA].StateVersion
	vB := fc.Nodes[nodeB].StateVersion
	diff := vA - vB
	if diff > vB {
		diff = vB - vA
	}
	if diff > 5 {
		return "HIGH"
	} else if diff > 1 {
		return "MEDIUM"
	} else if diff > 0 {
		return "LOW"
	}
	return "NONE"
}
