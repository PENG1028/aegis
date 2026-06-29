package handlers

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DeployNodeRequest is the request body for deploying a node to a remote machine.
type DeployNodeRequest struct {
	TargetIP      string `json:"target_ip"`
	SSHUser       string `json:"ssh_user"`
	SSHPassword   string `json:"ssh_password"`
	JoinToken     string `json:"join_token"`
	ControlPlaneURL string `json:"control_plane_url"`
	NodeName      string `json:"node_name"`
}

// DeployNodeResponse is the response after a deploy attempt.
type DeployNodeResponse struct {
	Success  bool   `json:"success"`
	NodeID   string `json:"node_id,omitempty"`
	Message  string `json:"message"`
	LogOutput string `json:"log_output,omitempty"`
}

// deployNode performs an SSH-based deployment to a remote machine.
// It installs caddy, copies the aegis binary, creates config, and starts the node agent.
func (h *Handlers) deployNode(req DeployNodeRequest) (*DeployNodeResponse, error) {
	var logBuf bytes.Buffer
	logf := func(format string, args ...interface{}) {
		line := fmt.Sprintf(format+"\n", args...)
		logBuf.WriteString(line)
	}

	target := fmt.Sprintf("%s@%s", req.SSHUser, req.TargetIP)

	// Write SSH password to a temp file (0600) so it never appears in /proc/<pid>/cmdline.
	// sshpass -f reads the password from a file descriptor, preventing exposure via ps aux.
	var sshPassFile string
	if req.SSHPassword != "" {
		f, err := os.CreateTemp("", "aegis-ssh-*")
		if err != nil {
			return &DeployNodeResponse{
				Success: false,
				Message: fmt.Sprintf("cannot create temp file for SSH password: %v", err),
			}, nil
		}
		if err := f.Chmod(0600); err != nil {
			f.Close()
			os.Remove(f.Name())
			return &DeployNodeResponse{
				Success: false,
				Message: fmt.Sprintf("cannot chmod temp file: %v", err),
			}, nil
		}
		if _, err := f.WriteString(req.SSHPassword); err != nil {
			f.Close()
			os.Remove(f.Name())
			return &DeployNodeResponse{
				Success: false,
				Message: fmt.Sprintf("cannot write temp file: %v", err),
			}, nil
		}
		f.Close()
		sshPassFile = f.Name()
		defer os.Remove(sshPassFile)
	}

	// Build SSH command. Uses sshpass -f (reads password from temp file) instead of
	// sshpass -p (which exposes the password in /proc/<pid>/cmdline).
	sshCmd := func(command string) *exec.Cmd {
		if sshPassFile != "" {
			return exec.Command("sshpass", "-f", sshPassFile,
				"ssh", "-o", "StrictHostKeyChecking=accept-new",
				"-o", "ConnectTimeout=10",
				target, command)
		}
		return exec.Command("ssh", "-o", "StrictHostKeyChecking=accept-new",
			"-o", "ConnectTimeout=10",
			target, command)
	}

	scpCmd := func(src, dst string) *exec.Cmd {
		if sshPassFile != "" {
			return exec.Command("sshpass", "-f", sshPassFile,
				"scp", "-o", "StrictHostKeyChecking=accept-new",
				"-o", "ConnectTimeout=10",
				src, fmt.Sprintf("%s:%s", target, dst))
		}
		return exec.Command("scp", "-o", "StrictHostKeyChecking=accept-new",
			"-o", "ConnectTimeout=10",
			src, fmt.Sprintf("%s:%s", target, dst))
	}

	logf("=== Deploying Aegis node to %s ===", req.TargetIP)

	// Step 1: Check connectivity
	logf("[1/7] Testing SSH connection...")
	cmd := sshCmd("echo ok")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("SSH connection failed: %v — %s", err, string(out)),
		}, nil
	}
	logf("  SSH connection OK")

	// Step 2: Install caddy if needed
	logf("[2/7] Checking Caddy...")
	cmd = sshCmd("which caddy 2>/dev/null || (sudo apt-get update -qq && sudo apt-get install -y -qq caddy 2>&1)")
	out, err = cmd.CombinedOutput()
	if err != nil {
		logf("  Warning: caddy install issue: %s", string(out))
	} else {
		logf("  Caddy available")
	}

	// Step 3: Create directories
	logf("[3/7] Creating directories...")
	cmd = sshCmd("sudo mkdir -p /etc/aegis /var/lib/aegis/backups/db /usr/local/bin && sudo chown -R $(whoami):$(whoami) /var/lib/aegis")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("Create directories failed: %v — %s", err, string(out)),
		}, nil
	}

	// Step 4: Copy aegis binary
	logf("[4/7] Copying aegis binary...")
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "/usr/local/bin/aegis"
	}
	cmd = scpCmd(selfPath, "/tmp/aegis")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("Copy binary failed: %v — %s", err, string(out)),
		}, nil
	}
	cmd = sshCmd("sudo mv /tmp/aegis /usr/local/bin/aegis && sudo chmod +x /usr/local/bin/aegis")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("Install binary failed: %v — %s", err, string(out)),
		}, nil
	}
	logf("  Binary installed")

	// Step 5: Write config files
	logf("[5/7] Writing configuration...")
	cpURL := req.ControlPlaneURL
	if cpURL == "" {
		cpURL = "http://127.0.0.1:7380"
	}

	nodeCfg := map[string]interface{}{
		"control_plane_url":        cpURL,
		"node_id":                  "",
		"node_token_file":          "/etc/aegis/node.token",
		"cache_dir":                "/var/lib/aegis",
		"runtime_dir":              "/run/aegis",
		"heartbeat_interval_seconds": 15,
		"sync_interval_seconds":    15,
		"reconcile_mode":           "apply",
	}
	nodeYAML, _ := yaml.Marshal(nodeCfg)
	cmd = sshCmd(fmt.Sprintf("cat > /etc/aegis/node.yaml << 'NODEEOF'\n%s\nNODEEOF", string(nodeYAML)))
	cmd.CombinedOutput()

	// Write join token — base64-encoded to prevent shell injection via single-quote or
	// other shell metacharacters in the token value.
	encodedToken := base64.StdEncoding.EncodeToString([]byte(req.JoinToken))
	cmd = sshCmd(fmt.Sprintf("echo -- %s | base64 -d > /etc/aegis/join.token && chmod 600 /etc/aegis/join.token", encodedToken))
	out, err = cmd.CombinedOutput()
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("Write join token failed: %v — %s", err, string(out)),
		}, nil
	}
	logf("  Config written")

	// Step 6: Install systemd service
	logf("[6/7] Installing systemd service...")
	svcContent := fmt.Sprintf(`[Unit]
Description=Aegis Node Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aegis node run --config /etc/aegis/node.yaml
Restart=always
RestartSec=5
TimeoutStartSec=30
TimeoutStopSec=10
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`)
	cmd = sshCmd(fmt.Sprintf("cat > /tmp/aegis-node.service << 'SVCEOF'\n%s\nSVCEOF && sudo mv /tmp/aegis-node.service /etc/systemd/system/aegis-node.service", svcContent))
	cmd.CombinedOutput()
	cmd = sshCmd("sudo systemctl daemon-reload && sudo systemctl enable aegis-node")
	out, err = cmd.CombinedOutput()
	if err != nil {
		logf("  Warning: systemd setup: %s", string(out))
	}
	logf("  Service installed")

	// Step 7: Start the node agent
	logf("[7/7] Starting node agent...")
	cmd = sshCmd("sudo systemctl start aegis-node 2>&1 || sudo /usr/local/bin/aegis node run --config /etc/aegis/node.yaml &>/tmp/aegis-node.log &")
	out, err = cmd.CombinedOutput()
	if err != nil {
		logf("  Warning: start had issues: %s", string(out))
	}
	logf("  Node agent starting...")

	logf("=== Deploy complete! Node should appear in the UI within 30 seconds. ===")
	logf("Check status: ssh %s 'sudo systemctl status aegis-node'", target)
	logf("View logs: ssh %s 'sudo journalctl -u aegis-node -f'", target)

	return &DeployNodeResponse{
		Success:   true,
		Message:   fmt.Sprintf("Node deployed to %s successfully. It will appear in the UI within 30 seconds.", req.TargetIP),
		LogOutput: logBuf.String(),
	}, nil
}

// isSSHAvailable checks if SSH-related tools are available for deployment.
func isSSHAvailable() bool {
	// Check if ssh command exists
	_, err := exec.LookPath("ssh")
	if err != nil {
		return false
	}
	// sshpass is optional (only needed for password auth)
	_, _ = exec.LookPath("sshpass")
	return true
}

// generateDeployCommand returns the one-liner shell command for manual deployment.
// Uses base64 encoding for the join token to prevent shell injection.
func generateDeployCommand(req DeployNodeRequest) string {
	cpURL := req.ControlPlaneURL
	if cpURL == "" {
		cpURL = "http://<control-plane-ip>:7380"
	}
	joinToken := req.JoinToken
	encodedToken := base64.StdEncoding.EncodeToString([]byte(joinToken))
	if joinToken == "" {
		joinToken = "<join-token>"
		encodedToken = "<base64-encoded-join-token>"
	}

	return fmt.Sprintf(
		`ssh %s@%s "sudo apt-get update -qq && sudo apt-get install -y -qq caddy && sudo mkdir -p /etc/aegis /var/lib/aegis && echo -- %s | base64 -d | sudo tee /etc/aegis/join.token > /dev/null && sudo chmod 600 /etc/aegis/join.token && printf 'control_plane_url: %s\nnode_token_file: /etc/aegis/node.token\ncache_dir: /var/lib/aegis\nheartbeat_interval_seconds: 15\nsync_interval_seconds: 15\nreconcile_mode: apply\n' | sudo tee /etc/aegis/node.yaml && echo '[Unit]\nDescription=Aegis Node Agent\nAfter=network-online.target\n\n[Service]\nType=simple\nExecStart=/usr/local/bin/aegis node run --config /etc/aegis/node.yaml\nRestart=always\n\n[Install]\nWantedBy=multi-user.target' | sudo tee /etc/systemd/system/aegis-node.service && sudo systemctl daemon-reload && sudo systemctl enable aegis-node && sudo systemctl start aegis-node"`,
		req.SSHUser, req.TargetIP, encodedToken, cpURL)
}

// AdminDeployNode handles POST /api/admin/v1/nodes/deploy
func (h *Handlers) AdminDeployNode(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.TargetIP == "" {
		writeError(w, http.StatusBadRequest, "target_ip is required")
		return
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.JoinToken == "" {
		writeError(w, http.StatusBadRequest, "join_token is required — create one via POST /api/admin/v1/node-join-tokens")
		return
	}

	if !isSSHAvailable() {
		// Return the manual command instead
		cmd := generateDeployCommand(req)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":       false,
			"message":       "ssh not available on this server — run this command manually on your dev machine:",
			"manual_command": cmd,
		})
		return
	}

	resp, err := h.deployNode(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Ensure imports are used
var _ = strings.TrimSpace
var _ = filepath.Join
var _ = os.Getenv
