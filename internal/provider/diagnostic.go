package provider

// Provider diagnostic error codes.
const (
	DiagCodeProviderMissing         = "PROVIDER_MISSING"
	DiagCodeVersionUnsupported      = "PROVIDER_VERSION_UNSUPPORTED"
	DiagCodeConfigFileMissing       = "CONFIG_FILE_MISSING"
	DiagCodeConfigValidateFailed    = "CONFIG_VALIDATE_FAILED"
	DiagCodeServiceNotRunning       = "SERVICE_NOT_RUNNING"
	DiagCodeListenerConflict        = "LISTENER_CONFLICT"
	DiagCodeRuntimeVerifyFailed     = "RUNTIME_VERIFY_FAILED"
)

// ProviderDiagnostic is a unified diagnostic result for a provider.
// Each field represents a specific diagnostic check that can be independently queried.
type ProviderDiagnostic struct {
	Provider         string `json:"provider"`
	Installed        bool   `json:"installed"`
	BinaryPath       string `json:"binary_path"`
	Version          string `json:"version"`
	VersionSupported bool   `json:"version_supported"`
	ConfigPath       string `json:"config_path"`
	ConfigExists     bool   `json:"config_exists"`
	ConfigValid      *bool  `json:"config_valid,omitempty"`
	ServiceRunning   *bool  `json:"service_running,omitempty"`
	ListenerOK       bool   `json:"listener_ok"`
	RuntimeVerifyOK  *bool  `json:"runtime_verify_ok,omitempty"`
	LastErrorCode    string `json:"last_error_code"`
	LastErrorMessage string `json:"last_error_message"`
	Stderr           string `json:"stderr"`
	CheckedAt        string `json:"checked_at"`
}

// Diagnoser is an optional interface for providers that support diagnostics.
type Diagnoser interface {
	Diagnose() ProviderDiagnostic
}
