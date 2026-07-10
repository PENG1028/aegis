// Package deploy provides a reusable SSH deployment toolkit.
//
// It defines interfaces for remote execution, file transfer, and service
// management — abstracted so the same flow works with SSH, local exec,
// or future transport layers (Docker, WinRM, etc.).
//
// Usage (see examples/ for real Aegis integration):
//
//	conn, _ := deploy.Connect(deploy.SSHConfig{
//	    Host: "192.168.10.11",
//	    User: "ubuntu",
//	    Key:  "-----BEGIN OPENSSH PRIVATE KEY-----...",
//	})
//	defer conn.Close()
//
//	// Run a command remotely
//	result := conn.Run(ctx, "which caddy")
//
//	// Copy a file
//	conn.Copy(ctx, "/usr/local/bin/aegis", "/tmp/aegis")
//
// Design principles (read this if you're building a similar system):
//
//  1. Interface first, implementation second.
//     Executor hides "is it SSH / local / Docker?"
//     ServiceManager hides "is it systemd / init.d / supervisor?"
//     Callers only depend on interfaces, not transport details.
//
//  2. Every error is structured.
//     RunResult has ExitCode, Stdout, Stderr — not just error.
//     UI can render structured logs, not just "failed".
//
//  3. Authentication is pluggable.
//     Key mode:    ssh -i key.pem user@host
//     Password:    sshpass -p xxx ssh user@host
//     Future:      SSO, certificate-based, vault
//
//  4. No Aegis business logic.
//     This package doesn't know about Caddy, systemd unit files,
//     /etc/aegis/, or any Aegis-specific paths. Those belong in
//     the caller (internal/httpapi/handlers/deploy_node.go).
//
//  5. UI annotations (@ui) are placed on key types and methods.
//     Search "@ui" to see which parts the frontend renders.
package deploy

import (
	"context"
	"io"
)

// ─── Transport ───────────────────────────────────────────────────────────────
// @ui: The frontend offers two authentication modes:
//   - "SSH Key" — paste a private key, used for automated deployment
//   - "SSH Password" — enter password (uses sshpass), simpler but less secure
//   - "Join Token" — invite-code style, for pull-based registration
//   The three can be combined: SSH to install, JoinToken to register.

// AuthMethod indicates which authentication to use for the SSH connection.
type AuthMethod string

const (
	AuthKey      AuthMethod = "key"       // SSH private key
	AuthPassword AuthMethod = "password"  // SSH password (via sshpass)
	AuthToken    AuthMethod = "token"     // Join token (pull mode, no SSH)
)

// SSHConfig holds all parameters for connecting to a remote machine.
// @ui: This struct maps directly to the DeployNode form fields:
//
//	ssh_user    → user@host input (e.g. "ubuntu@192.168.10.11")
//	auth_method → radio group: Key / Password / Token
//	ssh_key     → <textarea> or file picker (when auth=key)
//	ssh_password→ <input type=password> (when auth=password)
//	join_token  → <input> (when auth=token, or alongside SSH)
type SSHConfig struct {
	Host        string     `json:"host" yaml:"host"`
	User        string     `json:"user" yaml:"user"`
	Port        int        `json:"port" yaml:"port"`
	AuthMethod  AuthMethod `json:"auth_method" yaml:"auth_method"`
	SSHKey      string     `json:"ssh_key,omitempty" yaml:"ssh_key,omitempty"`           // PEM private key content
	SSHPassword string     `json:"ssh_password,omitempty" yaml:"ssh_password,omitempty"` // used with sshpass
	JoinToken   string     `json:"join_token,omitempty" yaml:"join_token,omitempty"`     // invite-code registration
}

// ─── Executor ─────────────────────────────────────────────────────────────────
// @ui: After deployment starts, the UI polls the API for log output.
// Each RunResult is rendered as a log line with status icon:
//
//	✓ SSH connection OK          (exit 0)
//	✗ Caddy install failed: ...  (exit non-zero)
//	→ Copying binary...           (in progress)

// RunResult captures the outcome of a remote command.
// @ui: Frontend renders status based on ExitCode:
//
//	exit 0      → green checkmark + message
//	exit non-0  → red X + Stderr
type RunResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Error    error // transport-level error (connection refused, etc.)
}

// Executor runs commands on a remote (or local) machine.
// @ui: Each deployment step calls executor.Run() and streams the result
// to a log panel. The frontend polls GET /api/admin/v1/nodes/deploy/{id}/logs
// to show real-time progress.
type Executor interface {
	// Run executes a command remotely and returns structured output.
	Run(ctx context.Context, command string) *RunResult

	// Close terminates the underlying connection (no-op for stateless executors).
	Close() error
}

// ─── File Transfer ───────────────────────────────────────────────────────────

// FileTransfer copies files to/from a remote machine.
type FileTransfer interface {
	// CopyTo uploads a local file to the remote path.
	// @ui: Shows a progress bar or "Copying binary (16MB)..."
	CopyTo(ctx context.Context, localPath, remotePath string) *RunResult

	// Close cleans up the transfer session.
	Close() error
}

// ─── Service Manager ─────────────────────────────────────────────────────────
// @ui: Service management is abstracted so the frontend doesn't need to
// know whether the target uses systemd, init.d, or supervisor.

// ServiceManager controls a service on the target machine.
type ServiceManager interface {
	// Install copies the service definition file and enables the service.
	Install(ctx context.Context, name, unitContent string) *RunResult

	// Start begins the service immediately.
	Start(ctx context.Context, name string) *RunResult

	// Stop stops the service.
	Stop(ctx context.Context, name string) *RunResult

	// Restart restarts the service (start if stopped, reload if running).
	Restart(ctx context.Context, name string) *RunResult

	// Status returns whether the service is active.
	Status(ctx context.Context, name string) *RunResult
}

// ─── Connector ───────────────────────────────────────────────────────────────

// Connection bundles all remote access interfaces into one session.
// @ui: At deployment time, the UI shows the connection tree:
//
//	Connecting → Executor ready | FileTransfer ready | ServiceMgr ready
type Connection struct {
	Executor  Executor
	Files     FileTransfer
	Services  ServiceManager
}

// Connect establishes a remote session based on the config.
// Returns an error if the transport cannot be initialised (bad key, unreachable host, etc.).
//
// @ui: If connect fails, the UI shows the error immediately under the SSH config form.
// @ui: If connect succeeds, the UI transitions to the deployment log view.
func Connect(ctx context.Context, cfg SSHConfig) (*Connection, error) {
	return connectSSH(ctx, cfg)
}

// ─── Empty types for forward reference ───────────────────────────────────────

// nopCloser is a helper for implementations that don't need cleanup.
type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// io.Discard reference to ensure import
var _ = io.Discard
