package localgateway

import (
	"fmt"
	"net"
	"os"
)

// DiagnosticLevel represents the severity of a diagnostic check result.
type DiagnosticLevel string

const (
	DiagOK      DiagnosticLevel = "ok"
	DiagWarning DiagnosticLevel = "warning"
	DiagFailed  DiagnosticLevel = "failed"
)

// DiagnosticCheck is a single startup diagnostic check result.
type DiagnosticCheck struct {
	Name   string         `json:"name"`
	Level  DiagnosticLevel `json:"level"`
	Detail string         `json:"detail,omitempty"`
}

// StartupDiagnosticsResult holds all startup diagnostic check results.
type StartupDiagnosticsResult struct {
	NodeID        string             `json:"node_id"`
	CacheDir      string             `json:"cache_dir"`
	TokenFile     string             `json:"token_file"`
	ControlPlane  string             `json:"control_plane_url"`
	GatewayStatus string             `json:"gateway_status"`
	Checks        []DiagnosticCheck  `json:"checks"`
	AllOK         bool               `json:"all_ok"`
	HasWarnings   bool               `json:"has_warnings"`
	HasFailed     bool               `json:"has_failed"`
}

// StartupDiagnosticsParams holds the parameters for startup diagnostics.
type StartupDiagnosticsParams struct {
	NodeID        string
	TokenFile     string
	CacheDir      string
	ControlPlane  string
	BindAddr      string
	Port          int
	SecretOK      bool
	RoutingOK     bool
}

// RunStartupDiagnostics runs startup diagnostic checks and returns results.
// Never leaks tokens in error messages.
func RunStartupDiagnostics(p StartupDiagnosticsParams) StartupDiagnosticsResult {
	result := StartupDiagnosticsResult{
		NodeID:       p.NodeID,
		CacheDir:     p.CacheDir,
		TokenFile:    p.TokenFile,
		ControlPlane: p.ControlPlane,
	}
	var checks []DiagnosticCheck
	allOK := true
	hasWarnings := false
	hasFailed := false

	// 1. node_id check
	if p.NodeID == "" {
		checks = append(checks, DiagnosticCheck{
			Name: "node_id", Level: DiagFailed,
			Detail: "node_id is not configured",
		})
		hasFailed = true
		allOK = false
	} else {
		checks = append(checks, DiagnosticCheck{
			Name: "node_id", Level: DiagOK,
			Detail: fmt.Sprintf("configured as %s", p.NodeID),
		})
	}

	// 2. token_file check
	if p.TokenFile == "" {
		checks = append(checks, DiagnosticCheck{
			Name: "token_file", Level: DiagWarning,
			Detail: "token_file not configured; node may not connect to control plane",
		})
		hasWarnings = true
		allOK = false
	} else {
		info, err := os.Stat(p.TokenFile)
		if err != nil {
			detail := err.Error()
			// Sanitize: don't leak path contents
			if os.IsNotExist(err) {
				detail = fmt.Sprintf("token_file not found: %s", p.TokenFile)
			} else if os.IsPermission(err) {
				detail = fmt.Sprintf("token_file not readable: %s", p.TokenFile)
			} else {
				detail = fmt.Sprintf("token_file access error: %s", p.TokenFile)
			}
			checks = append(checks, DiagnosticCheck{
				Name: "token_file", Level: DiagFailed,
				Detail: detail,
			})
			hasFailed = true
			allOK = false
		} else if info.IsDir() {
			checks = append(checks, DiagnosticCheck{
				Name: "token_file", Level: DiagFailed,
				Detail: fmt.Sprintf("token_file is a directory: %s", p.TokenFile),
			})
			hasFailed = true
			allOK = false
		} else if info.Mode().Perm()&0077 != 0 {
			checks = append(checks, DiagnosticCheck{
				Name: "token_file", Level: DiagWarning,
				Detail: fmt.Sprintf("token_file permissions too open: %o (recommend 0600)", info.Mode().Perm()),
			})
			hasWarnings = true
		} else {
			checks = append(checks, DiagnosticCheck{
				Name: "token_file", Level: DiagOK,
				Detail: fmt.Sprintf("exists at %s", p.TokenFile),
			})
		}
	}

	// 3. cache_dir check
	if p.CacheDir == "" {
		checks = append(checks, DiagnosticCheck{
			Name: "cache_dir", Level: DiagWarning,
			Detail: "cache_dir not configured; caching disabled",
		})
		hasWarnings = true
	} else {
		// Check if directory exists, if not check if parent is writable
		info, err := os.Stat(p.CacheDir)
		if err != nil {
			if os.IsNotExist(err) {
				// Check parent
				parent := parentDir(p.CacheDir)
				pInfo, pErr := os.Stat(parent)
				if pErr != nil || !pInfo.IsDir() {
					checks = append(checks, DiagnosticCheck{
						Name: "cache_dir", Level: DiagFailed,
						Detail: fmt.Sprintf("cache_dir parent does not exist: %s", parent),
					})
					hasFailed = true
					allOK = false
				} else {
					checks = append(checks, DiagnosticCheck{
						Name: "cache_dir", Level: DiagOK,
						Detail: fmt.Sprintf("will be created at %s", p.CacheDir),
					})
				}
			} else {
				checks = append(checks, DiagnosticCheck{
					Name: "cache_dir", Level: DiagFailed,
					Detail: fmt.Sprintf("cache_dir stat error: %s", p.CacheDir),
				})
				hasFailed = true
				allOK = false
			}
		} else if !info.IsDir() {
			checks = append(checks, DiagnosticCheck{
				Name: "cache_dir", Level: DiagFailed,
				Detail: fmt.Sprintf("cache_dir is not a directory: %s", p.CacheDir),
			})
			hasFailed = true
			allOK = false
		} else {
			// Test write permission
			testFile := p.CacheDir + "/.aegis_diag_test"
			if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
				checks = append(checks, DiagnosticCheck{
					Name: "cache_dir", Level: DiagFailed,
					Detail: fmt.Sprintf("cache_dir not writable: %s", p.CacheDir),
				})
				hasFailed = true
				allOK = false
			} else {
				os.Remove(testFile)
				checks = append(checks, DiagnosticCheck{
					Name: "cache_dir", Level: DiagOK,
					Detail: fmt.Sprintf("writable at %s", p.CacheDir),
				})
			}
		}
	}

	// 4. routing table cache check
	if p.RoutingOK {
		checks = append(checks, DiagnosticCheck{
			Name: "routing_table", Level: DiagOK,
			Detail: "routing table cache loaded and resolvable",
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name: "routing_table", Level: DiagWarning,
			Detail: "routing table cache not loaded; domains may not resolve",
		})
		hasWarnings = true
	}

	// 5. bind port check
	addr := fmt.Sprintf("%s:%d", p.BindAddr, p.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		checks = append(checks, DiagnosticCheck{
			Name: "bind_port", Level: DiagFailed,
			Detail: fmt.Sprintf("port %d bind failed: %v", p.Port, err),
		})
		hasFailed = true
		allOK = false
	} else {
		listener.Close()
		checks = append(checks, DiagnosticCheck{
			Name: "bind_port", Level: DiagOK,
			Detail: fmt.Sprintf("port %d available", p.Port),
		})
	}

	// 6. secret provider check
	if p.SecretOK {
		checks = append(checks, DiagnosticCheck{
			Name: "secret_provider", Level: DiagOK,
			Detail: "secret provider available",
		})
	} else {
		checks = append(checks, DiagnosticCheck{
			Name: "secret_provider", Level: DiagWarning,
			Detail: "secret provider not configured; relay authentication may fail",
		})
		hasWarnings = true
	}

	// 7. control_plane check
	if p.ControlPlane == "" {
		checks = append(checks, DiagnosticCheck{
			Name: "control_plane", Level: DiagWarning,
			Detail: "control_plane_url not configured; node runtime sync disabled",
		})
		hasWarnings = true
	} else {
		checks = append(checks, DiagnosticCheck{
			Name: "control_plane", Level: DiagOK,
			Detail: fmt.Sprintf("configured at %s", p.ControlPlane),
		})
	}

	result.Checks = checks
	result.AllOK = allOK && !hasWarnings && !hasFailed
	result.HasWarnings = hasWarnings
	result.HasFailed = hasFailed

	// Compute gateway status string
	switch {
	case hasFailed:
		result.GatewayStatus = "failed"
	case hasWarnings:
		result.GatewayStatus = "degraded"
	default:
		result.GatewayStatus = "ready"
	}

	return result
}

// parentDir returns the parent directory of a path.
func parentDir(path string) string {
	// Find last separator
	for i := len(path) - 2; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "/"
}

// SafeString returns a safe representation of diagnostics for logging.
// Never leaks raw tokens.
func (r StartupDiagnosticsResult) SafeString() string {
	s := fmt.Sprintf("StartupDiagnostics{node_id=%s, gateway_status=%s, all_ok=%v, has_warnings=%v, has_failed=%v}",
		r.NodeID, r.GatewayStatus, r.AllOK, r.HasWarnings, r.HasFailed)
	return s
}
