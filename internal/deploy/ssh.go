package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─── SSH Executor ────────────────────────────────────────────────────────────
// @ui: The SSH executor powers the "SSH Key" and "SSH Password" modes.
// The frontend sends one of:
//
//	{"auth_method": "key",      "ssh_key": "-----BEGIN..."}
//	{"auth_method": "password", "ssh_password": "..."}
//
// Both use the same executor underneath — only the auth flag changes.

// sshExecutor runs commands over SSH via the local `ssh` binary.
// It supports both key-based and password-based authentication.
//
// Key mode:     ssh -i <key_file> -o StrictHostKeyChecking=accept-new user@host <cmd>
// Password:     sshpass -f <pass_file> ssh -o ... user@host <cmd>
//
// The executor does NOT use golang.org/x/crypto/ssh directly because:
//  1. The deploy toolkit runs on the control plane, which already has ssh/sshpass.
//  2. Using the system SSH binary supports SSH config files, jump hosts, and agent forwarding.
//  3. This keeps the deploy package's dependency footprint minimal.
//
// @ui: Frontend implementation notes:
//   - SSH key should be accepted as both file upload and paste.
//   - Password input should have a show/hide toggle.
//   - "Test Connection" button before deploying is recommended.
type sshExecutor struct {
	host       string
	user       string
	port       int
	keyFile    string   // temp file holding the SSH key (cleaned up on Close)
	passFile   string   // temp file holding the password (cleaned up on Close)
}

func newSSHExecutor(cfg SSHConfig) (*sshExecutor, error) {
	e := &sshExecutor{
		host: cfg.Host,
		user: cfg.User,
		port: cfg.Port,
	}

	switch cfg.AuthMethod {
	case AuthKey:
		if cfg.SSHKey == "" {
			return nil, fmt.Errorf("deploy: ssh_key is required for key auth")
		}
		f, err := os.CreateTemp("", "aegis-deploy-key-*")
		if err != nil {
			return nil, fmt.Errorf("deploy: create temp key file: %w", err)
		}
		if _, err := f.WriteString(cfg.SSHKey); err != nil {
			f.Close()
			os.Remove(f.Name())
			return nil, fmt.Errorf("deploy: write key file: %w", err)
		}
		if err := f.Chmod(0600); err != nil {
			f.Close()
			os.Remove(f.Name())
			return nil, fmt.Errorf("deploy: chmod key file: %w", err)
		}
		f.Close()
		e.keyFile = f.Name()

	case AuthPassword:
		if cfg.SSHPassword == "" {
			return nil, fmt.Errorf("deploy: ssh_password is required for password auth")
		}
		// Verify sshpass is available
		if _, err := exec.LookPath("sshpass"); err != nil {
			return nil, fmt.Errorf("deploy: sshpass not found (install with: apt install sshpass)")
		}
		// Write password to temp file (never passes via -p to avoid /proc exposure)
		f, err := os.CreateTemp("", "aegis-deploy-pass-*")
		if err != nil {
			return nil, fmt.Errorf("deploy: create temp pass file: %w", err)
		}
		if _, err := f.WriteString(cfg.SSHPassword); err != nil {
			f.Close()
			os.Remove(f.Name())
			return nil, fmt.Errorf("deploy: write pass file: %w", err)
		}
		if err := f.Chmod(0600); err != nil {
			f.Close()
			os.Remove(f.Name())
			return nil, fmt.Errorf("deploy: chmod pass file: %w", err)
		}
		f.Close()
		e.passFile = f.Name()

	case AuthToken:
		// Token mode doesn't create an executor — it's for pull-based registration.
		// Return a nil executor here; the caller handles token registration separately.
		return nil, fmt.Errorf("deploy: token auth is not a transport — use key or password for SSH")

	default:
		return nil, fmt.Errorf("deploy: unknown auth method: %s", cfg.AuthMethod)
	}

	return e, nil
}

func (e *sshExecutor) Run(ctx context.Context, command string) *RunResult {
	target := fmt.Sprintf("%s@%s", e.user, e.host)

	// Build base args
	var args []string
	if e.passFile != "" {
		args = append(args, "sshpass", "-f", e.passFile)
	}
	args = append(args, "ssh",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
	)
	if e.keyFile != "" {
		args = append(args, "-i", e.keyFile)
	}
	if e.port > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", e.port))
	}
	args = append(args, target, command)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &RunResult{Error: err}
		}
	}

	return &RunResult{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
	}
}

func (e *sshExecutor) Close() error {
	var errs []string
	if e.keyFile != "" {
		if err := os.Remove(e.keyFile); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if e.passFile != "" {
		if err := os.Remove(e.passFile); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("deploy: cleanup errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ─── SSH File Transfer ──────────────────────────────────────────────────────

type sshFileTransfer struct {
	host     string
	user     string
	port     int
	keyFile  string
	passFile string
}

func newSSHFileTransfer(cfg SSHConfig, e *sshExecutor) *sshFileTransfer {
	return &sshFileTransfer{
		host:     cfg.Host,
		user:     cfg.User,
		port:     cfg.Port,
		keyFile:  e.keyFile,
		passFile: e.passFile,
	}
}

func (f *sshFileTransfer) CopyTo(ctx context.Context, localPath, remotePath string) *RunResult {
	target := fmt.Sprintf("%s@%s:%s", f.user, f.host, remotePath)

	var args []string
	if f.passFile != "" {
		args = append(args, "sshpass", "-f", f.passFile)
	}
	args = append(args, "scp",
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
	)
	if f.keyFile != "" {
		args = append(args, "-i", f.keyFile)
	}
	if f.port > 0 {
		args = append(args, "-P", fmt.Sprintf("%d", f.port))
	}
	args = append(args, localPath, target)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return &RunResult{
			Error:  fmt.Errorf("scp failed: %w — %s", err, strings.TrimSpace(stderr.String())),
			Stderr: stderr.String(),
		}
	}

	return &RunResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("copied %s → %s:%s", filepath.Base(localPath), f.host, remotePath),
	}
}

func (f *sshFileTransfer) Close() error { return nil }

// ─── SSH Service Manager ────────────────────────────────────────────────────

type sshServiceManager struct {
	executor *sshExecutor
}

func newSSHServiceManager(executor *sshExecutor) *sshServiceManager {
	return &sshServiceManager{executor: executor}
}

func (m *sshServiceManager) Install(ctx context.Context, name, unitContent string) *RunResult {
	// Write unit file via tee (avoids shell escaping issues with heredoc)
	cmd := fmt.Sprintf("cat > /tmp/%s.service << 'AEGISUNIT'\n%s\nAEGISUNIT && sudo mv /tmp/%s.service /etc/systemd/system/%s.service && sudo systemctl daemon-reload && sudo systemctl enable %s",
		name, unitContent, name, name, name)
	return m.executor.Run(ctx, cmd)
}

func (m *sshServiceManager) Start(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl start %s", name))
}

func (m *sshServiceManager) Stop(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl stop %s", name))
}

func (m *sshServiceManager) Restart(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl restart %s", name))
}

func (m *sshServiceManager) Status(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("systemctl is-active %s", name))
}

// ─── Connect ────────────────────────────────────────────────────────────────

func connectSSH(ctx context.Context, cfg SSHConfig) (*Connection, error) {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.User == "" {
		cfg.User = "root"
	}

	executor, err := newSSHExecutor(cfg)
	if err != nil {
		return nil, err
	}

	// Verify connectivity
	result := executor.Run(ctx, "echo ok")
	if result.Error != nil {
		executor.Close()
		return nil, fmt.Errorf("deploy: ssh connect to %s@%s: %w", cfg.User, cfg.Host, result.Error)
	}
	if result.ExitCode != 0 {
		executor.Close()
		return nil, fmt.Errorf("deploy: ssh connect to %s@%s: exit %d — %s", cfg.User, cfg.Host, result.ExitCode, result.Stderr)
	}

	return &Connection{
		Executor:  executor,
		Files:     newSSHFileTransfer(cfg, executor),
		Services:  newSSHServiceManager(executor),
	}, nil
}
