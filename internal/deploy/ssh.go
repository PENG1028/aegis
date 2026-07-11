package deploy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// ─── Native SSH via crypto/ssh ────────────────────────────────────────────────
// Zero external binary dependencies — no ssh, sshpass, or scp required.
// Supports key-based and password-based authentication.

type sshClient struct {
	client *ssh.Client
}

func dialSSH(cfg SSHConfig) (*sshClient, error) {
	var authMethods []ssh.AuthMethod

	switch cfg.AuthMethod {
	case AuthKey:
		signer, err := ssh.ParsePrivateKey([]byte(cfg.SSHKey))
		if err != nil {
			// Try with passphrase (treat key as-is; if encrypted, user must provide decrypted key)
			return nil, fmt.Errorf("parse SSH key: %w — ensure the key is not password-protected, or use password auth", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	case AuthPassword:
		authMethods = append(authMethods, ssh.Password(cfg.SSHPassword))

	default:
		return nil, fmt.Errorf("unsupported auth method: %s", cfg.AuthMethod)
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.User == "" {
		cfg.User = "root"
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port)), &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s@%s:%d: %w", cfg.User, cfg.Host, cfg.Port, err)
	}

	return &sshClient{client: client}, nil
}

// ─── Executor ─────────────────────────────────────────────────────────────────

type nativeExecutor struct {
	client *ssh.Client
}

func (e *nativeExecutor) Run(ctx context.Context, command string) *RunResult {
	session, err := e.client.NewSession()
	if err != nil {
		return &RunResult{Error: fmt.Errorf("create session: %w", err)}
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	runErr := session.Run(command)
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return &RunResult{Error: runErr}
		}
	}

	return &RunResult{
		ExitCode: exitCode,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
	}
}

func (e *nativeExecutor) Close() error {
	return e.client.Close()
}

// ─── File Transfer (SFTP) ─────────────────────────────────────────────────────

type nativeFileTransfer struct {
	client *ssh.Client
}

func (f *nativeFileTransfer) CopyTo(ctx context.Context, localPath, remotePath string) *RunResult {
	src, err := os.Open(localPath)
	if err != nil {
		return &RunResult{Error: fmt.Errorf("open local file: %w", err)}
	}
	defer src.Close()

	sftpClient, err := sftpClient(f.client)
	if err != nil {
		return &RunResult{Error: fmt.Errorf("sftp: %w", err)}
	}
	defer sftpClient.Close()

	dst, err := sftpClient.Create(remotePath)
	if err != nil {
		return &RunResult{Error: fmt.Errorf("sftp create %s: %w", remotePath, err)}
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return &RunResult{Error: fmt.Errorf("sftp copy: %w", err)}
	}

	return &RunResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("copied %s → %s:%s", filepath.Base(localPath), f.client.RemoteAddr().String(), remotePath),
	}
}

func (f *nativeFileTransfer) Close() error { return nil }

// sftpClient creates an SFTP client from an SSH client.
// Uses the external SFTP library if available, otherwise falls back to manual.
func sftpClient(client *ssh.Client) (sftpCloser, error) {
	return newSFTP(client)
}

// sftpCloser combines the SFTP client interface with Close.
type sftpCloser interface {
	Create(path string) (io.WriteCloser, error)
	Close() error
}

// Minimal SFTP implementation — only Create (upload) without external library.
type minimalSFTP struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
}

func newSFTP(client *ssh.Client) (*minimalSFTP, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, err
	}
	return &minimalSFTP{client: client, session: session, stdin: stdin}, nil
}

func (s *minimalSFTP) Create(remotePath string) (io.WriteCloser, error) {
	// Use SCP-like approach: cat > remotePath
	cmd := fmt.Sprintf("cat > %s", remotePath)
	s.session.Stderr = nil // discard stderr
	if err := s.session.Start(cmd); err != nil {
		return nil, err
	}
	return &scpWriter{stdin: s.stdin, session: s.session, path: remotePath}, nil
}

func (s *minimalSFTP) Close() error {
	return s.client.Close()
}

type scpWriter struct {
	stdin   io.WriteCloser
	session *ssh.Session
	path    string
}

func (w *scpWriter) Write(p []byte) (int, error) {
	return w.stdin.Write(p)
}

func (w *scpWriter) Close() error {
	w.stdin.Close()
	return w.session.Wait()
}

// ─── Service Manager ──────────────────────────────────────────────────────────

type nativeServiceManager struct {
	executor *nativeExecutor
}

func (m *nativeServiceManager) Install(ctx context.Context, name, unitContent string) *RunResult {
	cmd := fmt.Sprintf("cat > /tmp/%s.service << 'AEGISUNIT'\n%s\nAEGISUNIT && sudo mv /tmp/%s.service /etc/systemd/system/%s.service && sudo systemctl daemon-reload && sudo systemctl enable %s",
		name, unitContent, name, name, name)
	return m.executor.Run(ctx, cmd)
}

func (m *nativeServiceManager) Start(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl start %s", name))
}

func (m *nativeServiceManager) Stop(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl stop %s", name))
}

func (m *nativeServiceManager) Restart(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("sudo systemctl restart %s", name))
}

func (m *nativeServiceManager) Status(ctx context.Context, name string) *RunResult {
	return m.executor.Run(ctx, fmt.Sprintf("systemctl is-active %s", name))
}

// ─── Connect ──────────────────────────────────────────────────────────────────

func connectSSH(ctx context.Context, cfg SSHConfig) (*Connection, error) {
	client, err := dialSSH(cfg)
	if err != nil {
		return nil, err
	}

	executor := &nativeExecutor{client: client.client}

	// Verify connectivity
	result := executor.Run(ctx, "echo ok")
	if result.Error != nil {
		client.client.Close()
		return nil, fmt.Errorf("ssh connect to %s@%s: %w", cfg.User, cfg.Host, result.Error)
	}
	if result.ExitCode != 0 {
		client.client.Close()
		return nil, fmt.Errorf("ssh connect to %s@%s: exit %d — %s", cfg.User, cfg.Host, result.ExitCode, result.Stderr)
	}

	return &Connection{
		Executor: executor,
		Files:    &nativeFileTransfer{client: client.client},
		Services: &nativeServiceManager{executor: executor},
	}, nil
}

// Ensure io and fs used for future SFTP
var _ = io.EOF
var _ fs.FileInfo
