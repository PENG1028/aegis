package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aegis/internal/config"
	"aegis/internal/core"
	"aegis/internal/deploy"
	"aegis/internal/distnode"

	"gopkg.in/yaml.v3"
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
//
// @ui: Frontend form layout (see ui/src/pages/runtime/DeployNode.tsx):
//
//	┌─ 部署目标 ───────────────────────────────────┐
//	│  SSH 地址: [user@host              ]  端口: [] │
//	│                                                │
//	│  认证方式:  ● SSH Key  ○ SSH Password          │
//	│  [SSH Key]  [-----BEGIN OPENSSH PRIVATE...]    │
//	│  [或选择文件]                                   │
//	│                                                │
//	│  [测试连接]  [开始部署]                         │
//	└────────────────────────────────────────────────┘
type DeployNodeRequest struct {
	TargetIP    string `json:"target_ip"`    // e.g. "192.168.10.11"
	SSHUser     string `json:"ssh_user"`     // e.g. "ubuntu", defaults to "root"
	SSHPort     int    `json:"ssh_port"`     // SSH port, defaults to 22
	AuthMethod  string `json:"auth_method"`  // "key" | "password"
	SSHKey      string `json:"ssh_key"`      // PEM private key content (for auth=key)
	SSHPassword string `json:"ssh_password"` // SSH password (for auth=password)
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

	// If SSH tools aren't available locally, SSH deployment cannot run from this
	// control plane host.
	if !isSSHAvailable() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "ssh/scp is not available on this control plane host; install OpenSSH client or run deployment from a Linux control plane",
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
	return h.executeDeployServe(ctx, req, cpURL, logf)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// isSSHAvailable checks if the local system has SSH tools.
// @ui: If this returns false, the frontend switches to "manual command" mode.
func (h *Handlers) executeDeployServe(ctx context.Context, req DeployNodeRequest, cpURL string, logf func(string, ...interface{})) (*DeployNodeResponse, error) {
	if h.Config == nil || h.Config.DistNode.Secret == "" {
		return &DeployNodeResponse{Success: false, Message: "distnode secret is not configured on this control plane"}, nil
	}

	logf("=== Deploying Aegis serve node to %s ===", req.TargetIP)
	logf("[1/8] Connecting via SSH (%s auth)...", req.AuthMethod)
	conn, err := deploy.Connect(ctx, deploy.SSHConfig{
		Host:        req.TargetIP,
		User:        req.SSHUser,
		Port:        req.SSHPort,
		AuthMethod:  deploy.AuthMethod(req.AuthMethod),
		SSHKey:      req.SSHKey,
		SSHPassword: req.SSHPassword,
	})
	if err != nil {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("SSH connection failed: %v", err)}, nil
	}
	defer conn.Executor.Close()
	defer conn.Files.Close()
	logf("  SSH connection OK")

	logf("[2/8] Reading target identity...")
	hostResult := conn.Executor.Run(ctx, "hostname")
	targetName := strings.TrimSpace(hostResult.Stdout)
	if req.NodeName != "" {
		targetName = strings.TrimSpace(req.NodeName)
	}
	if targetName == "" {
		targetName = req.TargetIP
	}
	targetNodeID := "node_" + targetName
	cpHost := edgeHost(cpURL, req.TargetIP)
	controlEdge := net.JoinHostPort(cpHost, "80")
	targetEdge := net.JoinHostPort(req.TargetIP, "80")
	logf("  Target name: %s", targetName)
	logf("  Control edge: %s", controlEdge)
	logf("  Target edge: %s", targetEdge)

	logf("[3/8] Creating directories...")
	result := conn.Executor.Run(ctx, "sudo mkdir -p /etc/aegis /var/lib/aegis/backups/db /var/lib/aegis/keys /run/aegis /usr/local/bin && sudo chown -R $(whoami):$(whoami) /var/lib/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Create dirs failed: %v %s", result.Error, result.Stderr)}, nil
	}
	logf("  Directories ready")

	logf("[4/8] Copying aegis binary...")
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "/usr/local/bin/aegis"
	}
	result = conn.Files.CopyTo(ctx, selfPath, "/tmp/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Copy binary failed: %v %s", result.Error, result.Stderr)}, nil
	}
	result = conn.Executor.Run(ctx, "sudo install -m 0755 /tmp/aegis /usr/local/bin/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Install binary failed: %v %s", result.Error, result.Stderr)}, nil
	}
	logf("  Binary installed")

	logf("[5/8] Writing /etc/aegis/config.yaml...")
	targetAdminToken := core.NewID("adm")
	cfgYAML, err := renderNodeServeConfig(h.Config.Proxy, targetName, targetAdminToken, h.Config.DistNode.Secret, h.Config.DistNode.ID, controlEdge)
	if err != nil {
		return &DeployNodeResponse{Success: false, Message: "Render node config failed: " + err.Error()}, nil
	}
	result = conn.Executor.Run(ctx, fmt.Sprintf("cat > /tmp/aegis-config.yaml << 'CFG'\n%s\nCFG\nsudo mv /tmp/aegis-config.yaml /etc/aegis/config.yaml && sudo chmod 600 /etc/aegis/config.yaml", cfgYAML))
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Write config failed: %v %s", result.Error, result.Stderr)}, nil
	}
	logf("  config.yaml written")

	logf("[6/8] Installing aegis.service...")
	unitContent := `[Unit]
Description=Aegis Gateway Control Plane
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aegis serve --config /etc/aegis/config.yaml
Restart=always
RestartSec=5
TimeoutStartSec=30
TimeoutStopSec=10

[Install]
WantedBy=multi-user.target
`
	result = conn.Services.Install(ctx, "aegis", unitContent)
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Install service failed: %v %s", result.Error, result.Stderr)}, nil
	}
	logf("  aegis.service installed")

	logf("[7/8] Starting aegis serve...")
	result = conn.Services.Restart(ctx, "aegis")
	if result.Error != nil || result.ExitCode != 0 {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Aegis service failed to start: %v %s", result.Error, result.Stderr)}, nil
	}
	health := conn.Executor.Run(ctx, "sleep 3; curl -s http://127.0.0.1:7380/api/healthz")
	if health.ExitCode != 0 || !strings.Contains(health.Stdout, "alive") {
		return &DeployNodeResponse{Success: false, Message: fmt.Sprintf("Local health check failed: %s %s", health.Stdout, health.Stderr)}, nil
	}
	logf("  Local control plane is healthy")

	logf("[8/8] Registering distnode peer and validating edge...")
	if h.DistNode != nil {
		h.DistNode.Membership.AddPeer(distnode.PeerConfig{ID: targetName, Addr: targetEdge})
	}
	if !slices.ContainsFunc(h.Config.DistNode.Peers, func(p config.DistNodePeer) bool { return p.ID == targetName }) {
		h.Config.DistNode.Peers = append(h.Config.DistNode.Peers, config.DistNodePeer{ID: targetName, Addr: targetEdge})
		cpCfgPath := filepath.Join(h.Config.Runtime.ConfigDir, "config.yaml")
		if err := h.Config.Save(cpCfgPath); err != nil {
			return &DeployNodeResponse{Success: false, Message: "write control-plane config failed: " + err.Error()}, nil
		}
	}
	if code := applyTarget(ctx, conn, targetAdminToken); code == "200" {
		logf("  Target apply OK")
	} else {
		logf("  Warning: target apply returned HTTP %s; provider edge may need manual repair", code)
	}
	if err := waitHTTPAlive(ctx, "http://"+targetEdge+"/api/healthz", 12*time.Second); err != nil {
		return &DeployNodeResponse{Success: false, NodeID: targetNodeID, Message: "Aegis installed, but target 80 /api/healthz is not reachable: " + err.Error()}, nil
	}
	logf("  Target edge /api/healthz reachable")
	logf("=== Deploy complete: %s joined as %s ===", req.TargetIP, targetNodeID)

	return &DeployNodeResponse{
		Success: true,
		NodeID:  targetNodeID,
		Message: fmt.Sprintf("Node deployed to %s and configured for distnode.", req.TargetIP),
	}, nil
}

func edgeHost(host, fallback string) string {
	host = strings.TrimSpace(host)
	if host == "" || strings.HasPrefix(host, "127.") || host == "localhost" {
		return fallback
	}
	if h, _, err := net.SplitHostPort(host); err == nil && h != "" {
		return h
	}
	if i := strings.LastIndex(host, ":"); i > -1 && !strings.Contains(host[i+1:], "]") {
		return host[:i]
	}
	return host
}

func renderNodeServeConfig(controlProxy config.ProxyConfig, nodeName, adminToken, distSecret, controlPeerID, controlEdge string) (string, error) {
	cfg := config.ProductionConfig()
	cfg.Proxy = nodeProxyConfig(controlProxy)
	cfg.Store = config.StoreConfig{
		SQLitePath:    "/var/lib/aegis/aegis.db",
		BackupEnabled: false,
		BackupDir:     "/var/lib/aegis/backups/db",
	}
	cfg.Server = config.ServerConfig{
		Addr:           "127.0.0.1:7380",
		AdminToken:     adminToken,
		SessionSecure:  false,
		AllowedOrigins: []string{},
	}
	cfg.ManagedDomain = config.ManagedDomainConfig{}
	cfg.DNS = config.DNSConfig{
		Enabled:    true,
		ListenAddr: ":5353",
		Upstream:   "1.1.1.1:53",
		RefreshSec: 10,
	}
	cfg.Egress = config.EgressConfig{Enabled: false}
	cfg.DistNode = config.DistNodeConfig{
		Enabled: true,
		ID:      nodeName,
		Name:    nodeName,
		Addr:    "127.0.0.1:7380",
		Secret:  distSecret,
		Peers: []config.DistNodePeer{
			{ID: controlPeerID, Addr: controlEdge},
		},
	}
	cfg.Runtime = config.RuntimeConfig{
		ConfigDir: "/etc/aegis",
		DataDir:   "/var/lib/aegis",
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func nodeProxyConfig(control config.ProxyConfig) config.ProxyConfig {
	proxy := config.ProductionConfig().Proxy
	provider := strings.TrimSpace(control.Provider)
	if provider == "" {
		provider = proxy.Provider
	}
	proxy.Provider = provider

	switch provider {
	case "haproxy":
		proxy.CaddyfilePath = "/etc/haproxy/haproxy.cfg"
		proxy.CaddyBinary = "haproxy"
		proxy.ReloadCommand = "systemctl reload haproxy"
		proxy.ValidateCommand = "haproxy -c -f {{config_path}}"
		proxy.BackupDir = "/var/lib/aegis/haproxy-backups"
	case "caddy":
		proxy.CaddyfilePath = "/etc/caddy/Caddyfile"
		proxy.CaddyBinary = "caddy"
		proxy.ReloadCommand = "systemctl reload caddy"
		proxy.ValidateCommand = "caddy validate --adapter caddyfile --config {{config_path}}"
		proxy.BackupDir = "/var/lib/aegis/backups"
	}

	if isLinuxSystemPath(control.CaddyfilePath) {
		proxy.CaddyfilePath = control.CaddyfilePath
	}
	if strings.TrimSpace(control.CaddyBinary) != "" {
		proxy.CaddyBinary = control.CaddyBinary
	}
	if strings.TrimSpace(control.ReloadCommand) != "" {
		proxy.ReloadCommand = control.ReloadCommand
	}
	if strings.TrimSpace(control.ValidateCommand) != "" {
		proxy.ValidateCommand = control.ValidateCommand
	}
	if isLinuxSystemPath(control.BackupDir) {
		proxy.BackupDir = control.BackupDir
	}
	proxy.Email = control.Email
	proxy.TlsCertFile = control.TlsCertFile
	proxy.TlsKeyFile = control.TlsKeyFile
	return proxy
}

func isLinuxSystemPath(path string) bool {
	path = strings.TrimSpace(path)
	return strings.HasPrefix(path, "/etc/") || strings.HasPrefix(path, "/var/")
}

func applyTarget(ctx context.Context, conn *deploy.Connection, adminToken string) string {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' -X POST http://127.0.0.1:7380/api/apply -H 'Authorization: Bearer %s' -H 'Content-Type: application/json'", adminToken)
	res := conn.Executor.Run(ctx, cmd)
	if res.Error != nil || res.ExitCode != 0 {
		return "000"
	}
	return strings.TrimSpace(res.Stdout)
}

func waitHTTPAlive(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	var lastErr error
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return lastErr
}

func isSSHAvailable() bool {
	_, err := os.Stat("/usr/bin/ssh")
	if err != nil {
		_, err = os.Stat("/usr/local/bin/ssh")
	}
	return err == nil
}

func (h *Handlers) AdminDeployPreflight(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.TargetIP == "" {
		writeError(w, http.StatusBadRequest, "target_ip required")
		return
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.AuthMethod == "" {
		req.AuthMethod = "key"
	}
	req.TargetIP = strings.TrimSpace(req.TargetIP)

	report, err := deploy.Preflight(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey:     req.SSHKey, SSHPassword: req.SSHPassword,
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
	if req.TargetIP == "" {
		writeError(w, http.StatusBadRequest, "target_ip required")
		return
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.AuthMethod == "" {
		req.AuthMethod = "key"
	}
	req.TargetIP = strings.TrimSpace(req.TargetIP)

	// Step 1: Preflight — is Aegis running?
	report, err := deploy.Preflight(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey:     req.SSHKey, SSHPassword: req.SSHPassword,
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
			"action": "deploy_first",
		})
		return
	}
	if !report.Aegis.Running {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false, "error": "目标 Aegis 未运行，请先启动服务: systemctl start aegis",
			"action": "start_first",
		})
		return
	}

	// Step 2: Connect to target
	conn, err := deploy.Connect(r.Context(), deploy.SSHConfig{
		Host: req.TargetIP, User: req.SSHUser, Port: req.SSHPort,
		AuthMethod: deploy.AuthMethod(req.AuthMethod),
		SSHKey:     req.SSHKey, SSHPassword: req.SSHPassword,
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
