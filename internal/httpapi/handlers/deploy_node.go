package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"aegis/internal/config"
	"aegis/internal/deploy"
	"aegis/internal/distnode"
	"aegis/internal/hostdep/provider"
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
	NodeName    string   `json:"node_name"`    // optional, defaults to hostname
	AutoInstall []string `json:"auto_install"`  // provider IDs to install on deploy (empty = detect only)
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
	default:
		writeError(w, http.StatusBadRequest, "auth_method must be 'key' or 'password'")
		return
	}

	// If SSH tools aren't available locally, fall back to manual command.
	// @ui: When the API returns manual_command, the frontend should display it
	// as a highlighted code block with a "复制命令" button.
	if !isSSHAvailable() {
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
	//   [1/5] Testing SSH connection...     ✓
	//   [2/5] Detecting [2/7] Installing Caddy
	// optionally installing middleware...           ✓
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
	logf("[1/5] Connecting via SSH (%s auth)...", req.AuthMethod)

	authMethod := deploy.AuthMethod(req.AuthMethod)
	conn, err := deploy.Connect(ctx, deploy.SSHConfig{
		Host:        req.TargetIP,
		User:        req.SSHUser,
		Port:        req.SSHPort,
		AuthMethod:  authMethod,
		SSHKey:      req.SSHKey,
		SSHPassword: req.SSHPassword,
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

	// ── Step 2: Detect + optionally install middleware providers ──
	// @ui: Shows a per-provider status line with version or install progress.
	// When AutoInstall is empty, only detection runs (no install).
	logf("[2/5] Detecting middleware...")

	// Determine which providers are expected on this node from the active
	// runtime mode — never hardcoded to a specific provider name.
	type provInfo struct {
		ID        string
		Installed bool
		Version   string
		Action    string // "detected" | "installing" | "installed" | "failed" | "skipped"
		Detail    string
	}
	var detected []provInfo

	for _, p := range h.ProvReg.ListAll() {
		diag := p.Diagnose()
		info := provInfo{ID: p.State().ID}
		if diag.Installed {
			info.Installed = true
			info.Version = diag.Version
			info.Action = "detected"
			info.Detail = fmt.Sprintf("version=%s", diag.Version)
		} else {
			info.Action = "not_installed"
			info.Detail = "binary not found"
		}
		detected = append(detected, info)
	}

	// Auto-install if the operator opted in.
	autoSet := make(map[string]bool, len(req.AutoInstall))
	for _, id := range req.AutoInstall {
		autoSet[id] = true
	}

	if len(autoSet) > 0 {
		for i := range detected {
			if !autoSet[detected[i].ID] || detected[i].Installed {
				continue
			}
			detected[i].Action = "installing"
			detected[i].Detail = "installing via local endpoint..."

			p := h.ProvReg.Get(detected[i].ID)
			if p == nil {
				detected[i].Action = "failed"
				detected[i].Detail = "provider not found in registry"
				continue
			}
			lc, ok := p.(provider.LifecycleProvider)
			if !ok || !lc.CanInstall() {
				detected[i].Action = "skipped"
				detected[i].Detail = "this provider does not support automatic installation"
				continue
			}
			// Trigger install on the NEW node via its own HTTP endpoint — the
			// node runs the install locally through LifecycleProvider.Install(),
			// not remote apt-get from the control plane.
			installCmd := fmt.Sprintf("curl -s -X POST http://127.0.0.1:7380/api/admin/v1/providers/%s/install", detected[i].ID)
			instResult := conn.Executor.Run(ctx, installCmd)
			if instResult.ExitCode != 0 {
				detected[i].Action = "failed"
				detected[i].Detail = fmt.Sprintf("install failed: %s", strings.TrimSpace(instResult.Stderr))
			} else {
				detected[i].Action = "installed"
				detected[i].Installed = true
				detected[i].Detail = "installed successfully"
			}
		}
	}

	for _, info := range detected {
		switch info.Action {
		case "detected":
			logf("  ✓ %s %s", info.ID, info.Detail)
		case "installing":
			logf("  ⏳ Installing %s...", info.ID)
		case "installed":
			logf("  ✓ %s installed", info.ID)
		case "failed":
			logf("  ✗ %s: %s", info.ID, info.Detail)
		case "skipped":
			logf("  ⚠ %s: %s", info.ID, info.Detail)
		default:
			logf("  ⚠ %s 未安装 — 可在节点页面手动安装", info.ID)
		}
	}

	// ── Step 3: Create directories ──
	logf("[3/5] Creating directories...")
	result := conn.Executor.Run(ctx, "sudo mkdir -p /etc/aegis /var/lib/aegis/backups/db /var/lib/aegis/keys /run/aegis /usr/local/bin && sudo chown -R $(whoami):$(whoami) /var/lib/aegis")
	if result.Error != nil {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Create dirs failed: %v", result.Error)}, nil
	}
	if result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Create dirs failed: %s", result.Stderr)}, nil
	}
	logf("  Directories created")

	// ── Step 4: Copy binary ──
	// @ui: Shows "Copying aegis binary (16MB)..."
	logf("[2/5] Copying aegis binary...")
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
	logf("[3/5] Writing configuration...")

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

	// ── Step 6: Install systemd service ──
	// @ui: "Installing systemd service... ✓"
	logf("[4/5] Installing systemd service...")
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
	logf("[5/5] Starting node agent...")
	result = conn.Services.Start(ctx, "aegis-node")
	if result.Error != nil || result.ExitCode != 0 {
		// The node agent failing to start is a hard failure — don't report a
		// green deploy over a node that never came up.
		return &DeployNodeResponse{Success: false,
			Message: fmt.Sprintf("Node agent failed to start (exit=%d): %s", result.ExitCode, result.Stderr)}, nil
	}
	logf("  Node agent started")

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
	return fmt.Sprintf(`ssh %s "sudo mkdir -p /etc/aegis /var/lib/aegis && echo 'control_plane_url: http://%s:7380
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


func (h *Handlers) AdminDeployPreflight(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil { writeError(w, http.StatusBadRequest, "invalid JSON"); return }
	if req.TargetIP == "" { writeError(w, http.StatusBadRequest, "target_ip required"); return }
	if req.SSHUser == "" { req.SSHUser = "root" }
	if req.SSHPort == 0 { req.SSHPort = 22 }
	if req.AuthMethod == "" { req.AuthMethod = "key" }
	req.TargetIP = strings.TrimSpace(req.TargetIP)

	report, err := deploy.Preflight(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey: req.SSHKey, SSHPassword: req.SSHPassword,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "report": report})
}

// ─── Node Join ────────────────────────────────────────────────────────────────

// AdminJoinNode handles POST /api/admin/v1/nodes/join
// Connects an existing Aegis instance as a node to this control plane via distnode.
func (h *Handlers) AdminJoinNode(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.TargetIP == "" { writeError(w, http.StatusBadRequest, "target_ip required"); return }
	if req.SSHUser == "" { req.SSHUser = "root" }
	if req.SSHPort == 0 { req.SSHPort = 22 }
	if req.AuthMethod == "" { req.AuthMethod = "key" }
	req.TargetIP = strings.TrimSpace(req.TargetIP)

	// Step 1: Preflight — is Aegis running?
	report, err := deploy.Preflight(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey: req.SSHKey, SSHPassword: req.SSHPassword,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "error": "SSH failed: " + err.Error(),
		})
		return
	}
	if report == nil || !report.Aegis.Found {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "error": "目标未安装 Aegis，请使用「开始部署」先进行全量部署",
			"action":  "deploy_first",
		})
		return
	}
	if !report.Aegis.Running {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "error": "目标 Aegis 未运行，请先启动服务: systemctl start aegis",
			"action":  "start_first",
		})
		return
	}

		// Step 2: Connect to target
	conn, err := deploy.Connect(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey: req.SSHKey, SSHPassword: req.SSHPassword,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "error": "SSH连接失败: " + err.Error(),
		})
		return
	}
	defer conn.Executor.Close()

	// Step 3: Get hostname as node_id
	hostResult := conn.Executor.Run(r.Context(), "hostname")
	targetHostname := strings.TrimSpace(hostResult.Stdout)
	if targetHostname == "" {
		targetHostname = req.TargetIP
	}

	// ── Control plane (A) identity + edge address ──
	// distnode reaches peers over the 80/443 edge (7380 is localhost-only and the
	// cloud firewall only opens 80/443). cpHost is A's externally reachable host.
	cpHost := r.Host
	if colon := strings.LastIndex(cpHost, ":"); colon > strings.LastIndex(cpHost, "]") {
		cpHost = cpHost[:colon]
	}
	if cpHost == "" || strings.HasPrefix(cpHost, "127.") || cpHost == "localhost" {
		cpHost = req.TargetIP
	}
	aEdge := net.JoinHostPort(cpHost, "80")
	bEdge := net.JoinHostPort(req.TargetIP, "80")

	// ── Step 4a: register B as a peer on THIS control plane ──
	// Persist to config (survives restart) and hot-add to the running Membership
	// (no A restart). Once A's health check reaches B:80/api/healthz, distnode's
	// OnEvent registers B in the nodes table and it appears in the UI.
	if h.DistNode != nil {
		h.DistNode.Membership.AddPeer(distnode.PeerConfig{ID: targetHostname, Addr: bEdge})
	}
	if !slices.ContainsFunc(h.Config.DistNode.Peers, func(p config.DistNodePeer) bool { return p.ID == targetHostname }) {
		h.Config.DistNode.Peers = append(h.Config.DistNode.Peers, config.DistNodePeer{ID: targetHostname, Addr: bEdge})
		cpCfgPath := filepath.Join(h.Config.Runtime.ConfigDir, "config.yaml")
		if err := h.Config.Save(cpCfgPath); err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": "写入控制平面配置失败: " + err.Error()})
			return
		}
	}

	// ── Step 4b: sync cluster secret + register A as a peer on the target ──
	// distnode default-on has already written B a distnode block with its own
	// random secret; rewrite it so B shares OUR cluster secret (cross-node RPC
	// HMAC) and knows us as a peer. Drop the existing block first (avoid a
	// duplicate YAML key) then append. `addr` is left for B's config loader to
	// fill from its own Server.Addr — no hardcoded port here.
	newBlock := fmt.Sprintf("distnode:\n  enabled: true\n  id: %q\n  name: %q\n  secret: %q\n  peers:\n    - id: %q\n      addr: %q\n",
		targetHostname, targetHostname, h.Config.DistNode.Secret, h.Config.DistNode.ID, aEdge)
	rewrite := "sudo cp /etc/aegis/config.yaml /etc/aegis/config.yaml.join-bak && " +
		"sudo awk 'BEGIN{s=0} /^distnode:/{s=1;next} s==1 && /^[^[:space:]]/{s=0} s==0{print}' /etc/aegis/config.yaml.join-bak | sudo tee /etc/aegis/config.yaml.new >/dev/null && " +
		fmt.Sprintf("cat <<'YAMLEOF' | sudo tee -a /etc/aegis/config.yaml.new >/dev/null\n%sYAMLEOF\n", newBlock) +
		"sudo mv /etc/aegis/config.yaml.new /etc/aegis/config.yaml"
	if res := conn.Executor.Run(r.Context(), rewrite); res.Error != nil || res.ExitCode != 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": false,
			"error": fmt.Sprintf("写入目标 distnode 配置失败: exit=%d err=%v stderr=%s", res.ExitCode, res.Error, res.Stderr)})
		return
	}

	// ── Step 5: restart target, then apply so its edge renders the control route ──
	restart := conn.Executor.Run(r.Context(), "sudo systemctl restart aegis && sleep 3 && curl -s http://127.0.0.1:7380/api/healthz")
	if restart.Error != nil || restart.ExitCode != 0 || !strings.Contains(restart.Stdout, "alive") {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": false,
			"error":    fmt.Sprintf("目标重启后健康检查失败: exit=%d stderr=%s", restart.ExitCode, restart.Stderr),
			"rollback": "SSH 到目标: sudo cp /etc/aegis/config.yaml.join-bak /etc/aegis/config.yaml && sudo systemctl restart aegis",
		})
		return
	}
	// Trigger an apply on the target so its Caddyfile gains the /api/* control
	// route (the ingress edge for its control plane). Uses the target's own admin
	// token from its config.
	applyCmd := "TOK=$(sudo awk '/admin_token:/{print $2; exit}' /etc/aegis/config.yaml | tr -d '\"'); " +
		"curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:7380/api/apply -H \"Authorization: Bearer $TOK\" -H 'Content-Type: application/json'"
	applyRes := conn.Executor.Run(r.Context(), applyCmd)
	if applyCode := strings.TrimSpace(applyRes.Stdout); applyCode != "200" {
		writeJSON(w, http.StatusOK, map[string]interface{}{"success": false,
			"error": "目标已加入但 apply 失败（控制路由未渲染）: http=" + applyCode + " — 请在目标执行 sudo aegis apply 或在 UI 重试",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "节点加入成功 — " + targetHostname,
		"node_id":   targetHostname,
		"target":    req.TargetIP,
		"peer_addr": bEdge,
		"next_step": "目标已重启并 apply。刷新节点列表，≤30s 内应看到该节点上线。",
	})

}
