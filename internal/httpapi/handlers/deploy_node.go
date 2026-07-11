package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"aegis/internal/deploy"
)

// ─── Request / Response ──────────────────────────────────────────────────────
// @ui: These types directly map to the DeployNode.tsx form fields.
// The frontend validation mirrors the backend validation (both check required fields).

// DeployNodeRequest is the request body for deploying Aegis to a remote machine.
//
// Authentication options (auth_method):
//
//	"key"      — SSH private key (recommended for automation)
//	"password" — SSH password via sshpass (simpler, for ad-hoc)
//	"token"    — Join-token only (pull mode, no SSH; the node registers itself)
//
// @ui: Frontend form layout (see ui/src/pages/runtime/DeployNode.tsx):
//
//	┌─ 部署目标 ───────────────────────────────────┐
//	│  SSH 地址: [user@host              ]  端口: [] │
//	│                                                │
//	│  认证方式:  ● SSH Key  ○ SSH Password  ○ Token │
//	│  [SSH Key]  [-----BEGIN OPENSSH PRIVATE...]    │
//	│  [或选择文件]                                   │
//	│                                                │
//	│  Join Token（可选，SSH模式传则自动注册）         │
//	│  [jt_abc123...]  [新建]                        │
//	│                                                │
//	│  [测试连接]  [开始部署]                         │
//	└────────────────────────────────────────────────┘
type DeployNodeRequest struct {
	TargetIP    string `json:"target_ip"`    // e.g. "192.168.10.11"
	SSHUser     string `json:"ssh_user"`     // e.g. "ubuntu", defaults to "root"
	SSHPort     int    `json:"ssh_port"`     // SSH port, defaults to 22
	AuthMethod  string `json:"auth_method"`  // "key" | "password" | "token"
	SSHKey      string `json:"ssh_key"`      // PEM private key content (for auth=key)
	SSHPassword string `json:"ssh_password"` // SSH password (for auth=password)
	JoinToken   string `json:"join_token"`   // invite-code (optional for SSH, required for token mode)
	NodeName    string `json:"node_name"`    // optional, defaults to hostname
}

// DeployNodeResponse is returned after a deploy attempt.
// @ui: The frontend renders the result based on Success:
//
//	Success=true  → green banner + node_id + "出现在节点列表中"
//	Success=false → red error + raw LogOutput for debugging
//	SSH not available → manual_command shown in a code block
type DeployNodeResponse struct {
	Success       bool   `json:"success"`
	NodeID        string `json:"node_id,omitempty"`
	Message       string `json:"message"`
	LogOutput     string `json:"log_output,omitempty"`
	ManualCommand string `json:"manual_command,omitempty"` // fallback when SSH unavailable
}

// ─── Handler ─────────────────────────────────────────────────────────────────
// @ui: The handler does 3 things the frontend needs to know:
//  1. Validates input → show errors inline on the form
//  2. If SSH is available → deploys via SSH, returns log_output
//  3. If SSH is NOT available → returns a manual_command (one-liner)
//     Frontend renders this as a copyable code block

// AdminDeployNode handles POST /api/admin/v1/nodes/deploy
//
// @ui: Frontend call pattern (see ui/src/lib/real-api-client.ts):
//
//	await post('/api/admin/v1/nodes/deploy', {
//	    target_ip: "192.168.10.11",
//	    auth_method: "key",
//	    ssh_key: "-----BEGIN...",
//	    join_token: "jt_xxx",  // optional
//	})
//
// @ui: The frontend should poll the result — deployment takes 10-30 seconds.
// For now it's synchronous; future versions should return a deployment ID
// and let the frontend poll GET /api/admin/v1/nodes/deploy/{id}/logs.
func (h *Handlers) AdminDeployNode(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// ── Validation ──────────────────────────────────────────────────────────
	// @ui: Each validation error should map to a specific form field:
	//   target_ip  → "SSH 地址不能为空"
	//   auth_method→ "请选择认证方式"
	//   ssh_key    → "请粘贴 SSH 私钥或选择文件"

	if req.TargetIP == "" {
		writeError(w, http.StatusBadRequest, "target_ip is required")
		return
	}
	if req.AuthMethod == "" {
		req.AuthMethod = "key" // default to key auth
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}

	// Validate auth-specific fields
	switch req.AuthMethod {
	case "key":
		if req.SSHKey == "" {
			writeError(w, http.StatusBadRequest, "ssh_key is required for key auth — paste your private key or upload a file")
			return
		}
	case "password":
		if req.SSHPassword == "" {
			writeError(w, http.StatusBadRequest, "ssh_password is required for password auth")
			return
		}
	case "token":
		if req.JoinToken == "" {
			writeError(w, http.StatusBadRequest, "join_token is required for token auth — create one in Access → Join Tokens")
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "auth_method must be 'key', 'password', or 'token'")
		return
	}

	// If SSH tools aren't available locally, fall back to manual command.
	// @ui: When the API returns manual_command, the frontend should display it
	// as a highlighted code block with a "复制命令" button.
	if !isSSHAvailable() && req.AuthMethod != "token" {
		cmd := generateDeployCommand(req)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success":        false,
			"message":        "ssh not available on this server — run this command manually on your target machine:",
			"manual_command": cmd,
		})
		return
	}

	// ── Determine control plane URL ──
	// @ui: The frontend doesn't need to set this — it's auto-detected from
	// the request's Host header. Falls back to 127.0.0.1:7380 for dev.
	cpURL := req.TargetIP // backward compat
	if r.Host != "" {
		cpURL = r.Host
	}

	// ── Deploy ──────────────────────────────────────────────────────────────
	// @ui: Deployment steps are logged to LogOutput, which the frontend polls.
	// Each step starts with a [N/7] marker — frontend renders these as:
	//   [1/7] Testing SSH connection...     ✓
	//   [2/7] Installing Caddy...           ✓
	//   ...

	var logBuf strings.Builder
	logf := func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format+"\n", args...))
	}

	result, err := h.executeDeploy(r.Context(), req, cpURL, logf)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result.LogOutput = logBuf.String()
	writeJSON(w, http.StatusOK, result)
}

// executeDeploy runs the 7-step deployment workflow.
//
// @ui: Each step maps to a visual phase in the deployment log.
// The frontend can animate the log as steps complete:
//
//	Phase 1: Connect     → steps 1
//	Phase 2: Prereqs     → steps 2-3
//	Phase 3: Install     → steps 4-5
//	Phase 4: Service     → steps 6-7
func (h *Handlers) executeDeploy(ctx context.Context, req DeployNodeRequest, cpURL string, logf func(string, ...interface{})) (*DeployNodeResponse, error) {
	// ── Connect ──
	// @ui: If this step fails, the form stays visible with the error.
	// If it succeeds, the form transitions to the log view.
	logf("=== Deploying Aegis node to %s ===", req.TargetIP)
	logf("[1/7] Connecting via SSH (%s auth)...", req.AuthMethod)

	authMethod := deploy.AuthMethod(req.AuthMethod)
	conn, err := deploy.Connect(ctx, deploy.SSHConfig{
		Host:        req.TargetIP,
		User:        req.SSHUser,
		Port:        req.SSHPort,
		AuthMethod:  authMethod,
		SSHKey:      req.SSHKey,
		SSHPassword: req.SSHPassword,
		JoinToken:   req.JoinToken,
	})
	if err != nil {
		return &DeployNodeResponse{
			Success: false,
			Message: fmt.Sprintf("SSH connection failed: %v", err),
		}, nil
	}
	defer conn.Executor.Close()
	defer conn.Files.Close()
	logf("  SSH connection OK")

	// ── Step 2: Check/Install Caddy ──
	// @ui: Shows "✔ Caddy already installed" or "⏳ Installing Caddy..."
	logf("[2/7] Checking Caddy...")
	result := conn.Executor.Run(ctx, "which caddy 2>/dev/null || (sudo apt-get update -qq && sudo apt-get install -y -qq caddy 2>&1)")
	if result.ExitCode == 0 && strings.Contains(result.Stdout, "caddy") {
		logf("  Caddy already installed")
	} else if result.ExitCode == 0 {
		logf("  Caddy installed")
	} else {
		logf("  Warning: caddy install: %s", result.Stderr)
	}

	// ── Step 3: Create directories ──
	logf("[3/7] Creating directories...")
	result = conn.Executor.Run(ctx, "sudo mkdir -p /etc/aegis /var/lib/aegis/backups/db /var/lib/aegis/keys /run/aegis /usr/local/bin && sudo chown -R $(whoami):$(whoami) /var/lib/aegis")
	if result.Error != nil {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Create dirs failed: %v", result.Error)}, nil
	}
	if result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Create dirs failed: %s", result.Stderr)}, nil
	}
	logf("  Directories created")

	// ── Step 4: Copy binary ──
	// @ui: Shows "Copying aegis binary (16MB)..."
	logf("[4/7] Copying aegis binary...")
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "/usr/local/bin/aegis"
	}
	result = conn.Files.CopyTo(ctx, selfPath, "/tmp/aegis")
	if result.Error != nil {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Copy binary failed: %v", result.Error)}, nil
	}
	logf("  Binary copied (%s)", result.Stdout)

	result = conn.Executor.Run(ctx, "sudo mv /tmp/aegis /usr/local/bin/aegis && sudo chmod +x /usr/local/bin/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Install binary failed: %v", result.Error)}, nil
	}
	logf("  Binary installed at /usr/local/bin/aegis")

	// ── Step 5: Write config ──
	// @ui: Config files are generated server-side. The frontend doesn't
	// need to know the details — it just sees "Writing configuration... ✓"
	logf("[5/7] Writing configuration...")

	// Write node.yaml
	cfgYAML := fmt.Sprintf(`control_plane_url: http://%s
node_token_file: /etc/aegis/node.token
cache_dir: /var/lib/aegis
runtime_dir: /run/aegis
heartbeat_interval_seconds: 15
sync_interval_seconds: 15
reconcile_mode: apply
`, cpURL)
	result = conn.Executor.Run(ctx, fmt.Sprintf("cat > /etc/aegis/node.yaml << 'CFG'\n%s\nCFG", cfgYAML))
	if result.Error != nil {
		logf("  Warning: write config: %s", result.Error)
	} else {
		logf("  node.yaml written")
	}

	// Write join token if provided
	// @ui: If the user provided a JoinToken in the form, it's used for registration.
	// If not, the node will be registered via SSH (the handler registers it directly).
	if req.JoinToken != "" {
		// base64-encode to prevent shell injection through the token value
		encoded := fmt.Sprintf("echo %x | xxd -r -p > /etc/aegis/join.token && chmod 600 /etc/aegis/join.token", req.JoinToken)
		conn.Executor.Run(ctx, encoded)
		logf("  join.token written")
	}

	// ── Step 6: Install systemd service ──
	// @ui: "Installing systemd service... ✓"
	logf("[6/7] Installing systemd service...")
	unitContent := `[Unit]
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

[Install]
WantedBy=multi-user.target
`
	result = conn.Services.Install(ctx, "aegis-node", unitContent)
	if result.Error != nil {
		logf("  Warning: service install: %s", result.Error)
	} else {
		logf("  Service installed and enabled")
	}

	// ── Step 7: Start ──
	// @ui: "Starting node agent... ✓" — the final step.
	logf("[7/7] Starting node agent...")
	result = conn.Services.Start(ctx, "aegis-node")
	if result.Error != nil || result.ExitCode != 0 {
		logf("  Warning: start output: %s", result.Stderr)
	}
	logf("  Node agent starting...")

	logf("=== Deploy complete! Node should appear in the UI within 30 seconds. ===")

	return &DeployNodeResponse{
		Success: true,
		Message: fmt.Sprintf("Node deployed to %s successfully. It will appear in the UI within 30 seconds.", req.TargetIP),
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// isSSHAvailable checks if the local system has SSH tools.
// @ui: If this returns false, the frontend switches to "manual command" mode.
func isSSHAvailable() bool {
	_, err := os.Stat("/usr/bin/ssh")
	if err != nil {
		_, err = os.Stat("/usr/local/bin/ssh")
	}
	return err == nil
}

// generateDeployCommand returns a one-liner for manual deployment.
// @ui: The frontend renders this as a code block with a "复制命令" button.
// Example layout:
//
//	┌─────────────────────────────────────────────────────────┐
//	│ 面板检测到当前服务器没有 SSH 工具。请在目标机器上运行：  │
//	│                                                         │
//	│  ssh ubuntu@192.168.10.11 'sudo curl -sL ... | bash'     │
//	│                                                         │
//	│  [复制命令]                                              │
//	└─────────────────────────────────────────────────────────┘
func generateDeployCommand(req DeployNodeRequest) string {
	target := fmt.Sprintf("%s@%s", req.SSHUser, req.TargetIP)
	return fmt.Sprintf(`ssh %s "sudo apt-get update -qq && sudo apt-get install -y -qq caddy && sudo mkdir -p /etc/aegis /var/lib/aegis && echo 'control_plane_url: http://%s:7380
node_token_file: /etc/aegis/node.token
cache_dir: /var/lib/aegis
heartbeat_interval_seconds: 15
sync_interval_seconds: 15
reconcile_mode: apply' | sudo tee /etc/aegis/node.yaml > /dev/null && echo '[Unit]
Description=Aegis Node Agent
After=network-online.target
[Service]
Type=simple
ExecStart=/usr/local/bin/aegis node run --config /etc/aegis/node.yaml
Restart=always
[Install]
WantedBy=multi-user.target' | sudo tee /etc/systemd/system/aegis-node.service > /dev/null && sudo systemctl daemon-reload && sudo systemctl enable aegis-node && sudo systemctl start aegis-node"`,
		target, req.TargetIP)
}

// ─── Preflight ────────────────────────────────────────────────────────────────

type PreflightResult struct {
	Success      bool     `json:"success"`
	Error        string   `json:"error,omitempty"`
	AegisFound   bool     `json:"aegis_found"`
	AegisVersion string   `json:"aegis_version,omitempty"`
	CaddyFound   bool     `json:"caddy_found"`
	ConfigFound  bool     `json:"config_found"`
	HasWarnings  bool     `json:"has_warnings"`
	Warnings     []string `json:"warnings,omitempty"`
}

func (h *Handlers) AdminDeployPreflight(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.TargetIP == "" { writeError(w, http.StatusBadRequest, "target_ip is required"); return }
	if req.SSHUser == "" { req.SSHUser = "root" }
	if req.SSHPort == 0 { req.SSHPort = 22 }
	if req.AuthMethod == "" { req.AuthMethod = "key" }

	conn, err := deploy.Connect(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey: req.SSHKey, SSHPassword: req.SSHPassword,
	})
	if err != nil { writeJSON(w, http.StatusOK, PreflightResult{Success: false, Error: fmt.Sprintf("SSH failed: %v", err)}); return }
	defer conn.Executor.Close()

	result := PreflightResult{Success: true}
	var warnings []string

	r1 := conn.Executor.Run(r.Context(), "test -f /usr/local/bin/aegis && /usr/local/bin/aegis version 2>/dev/null | head -1 || echo ''")
	if v := strings.TrimSpace(r1.Stdout); v != "" && strings.HasPrefix(v, "v") {
		result.AegisFound, result.AegisVersion = true, v
		warnings = append(warnings, "检测到 Aegis "+v+" — 建议使用节点加入而非全量部署")
	}

	r2 := conn.Executor.Run(r.Context(), "which caddy 2>/dev/null && caddy version 2>/dev/null | head -1 || echo ''")
	if v := strings.TrimSpace(r2.Stdout); v != "" {
		result.CaddyFound = true
		warnings = append(warnings, "已安装 Caddy — 部署时可跳过")
	}

	r3 := conn.Executor.Run(r.Context(), "test -f /etc/aegis/config.yaml && echo 'y' || echo ''")
	result.ConfigFound = strings.TrimSpace(r3.Stdout) == "y"
	if result.ConfigFound { warnings = append(warnings, "已有配置文件 — 部署将覆盖") }

	result.HasWarnings = len(warnings) > 0
	result.Warnings = warnings
	writeJSON(w, http.StatusOK, result)
}
