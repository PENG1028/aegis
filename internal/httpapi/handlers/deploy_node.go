package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"aegis/internal/config"
	"aegis/internal/core"
	"aegis/internal/deploy"
	"aegis/internal/distnode"
	"aegis/internal/distnode/onboarding"
	"aegis/internal/node"

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
	TargetIP        string `json:"target_ip"`         // e.g. "192.168.10.11"
	SSHUser         string `json:"ssh_user"`          // e.g. "ubuntu", defaults to "root"
	SSHPort         int    `json:"ssh_port"`          // SSH port, defaults to 22
	AuthMethod      string `json:"auth_method"`       // "key" | "password"
	SSHKey          string `json:"ssh_key"`           // PEM private key content (for auth=key)
	SSHPassword     string `json:"ssh_password"`      // SSH password (for auth=password)
	NodeName        string `json:"node_name"`         // optional, defaults to hostname
	ControllerMode  string `json:"controller_mode"`   // "current" | "push_only"
	ControlNodeID   string `json:"control_node_id"`   // required for push_only
	ControlEdgeAddr string `json:"control_edge_addr"` // required for push_only, e.g. "43.159.34.11:80"
	ControlSecret   string `json:"control_secret"`    // required for push_only
}

// DeployNodeResponse is returned after a deploy attempt.
// @ui: The frontend renders the result based on Success:
//
//	Success=true  → green banner + node_id + "出现在节点列表中"
//	Success=false → red error + raw LogOutput for debugging
//	SSH not available → manual_command shown in a code block
type DeployNodeResponse struct {
	Success       bool                    `json:"success"`
	Action        string                  `json:"action,omitempty"`
	NodeID        string                  `json:"node_id,omitempty"`
	PeerAddr      string                  `json:"peer_addr,omitempty"`
	Message       string                  `json:"message"`
	NextStep      string                  `json:"next_step,omitempty"`
	Steps         []onboarding.StepReport `json:"steps,omitempty"`
	Capabilities  []onboarding.Capability `json:"capabilities,omitempty"`
	LogOutput     string                  `json:"log_output,omitempty"`
	ManualCommand string                  `json:"manual_command,omitempty"` // fallback when SSH unavailable
}

type DeployPlanResponse struct {
	Success bool                    `json:"success"`
	Error   string                  `json:"error,omitempty"`
	Plan    *DeployPlan             `json:"plan,omitempty"`
	Report  *deploy.PreflightReport `json:"report,omitempty"`
}

type DeployPlan struct {
	Action        string              `json:"action"`
	CanProceed    bool                `json:"can_proceed"`
	Target        DeployPlanTarget    `json:"target"`
	Control       DeployPlanControl   `json:"control"`
	Artifact      DeployPlanArtifact  `json:"artifact"`
	Provider      DeployPlanProvider  `json:"provider"`
	Files         []DeployPlanFile    `json:"files"`
	Services      []DeployPlanService `json:"services"`
	Checks        []DeployPlanCheck   `json:"checks"`
	Warnings      []string            `json:"warnings,omitempty"`
	ManualActions []string            `json:"manual_actions,omitempty"`
}

type DeployPlanTarget struct {
	Host         string `json:"host"`
	OS           string `json:"os,omitempty"`
	Arch         string `json:"arch,omitempty"`
	AegisFound   bool   `json:"aegis_found"`
	AegisRunning bool   `json:"aegis_running"`
	ConfigFound  bool   `json:"config_found"`
	ConfigPath   string `json:"config_path,omitempty"`
}

type DeployPlanControl struct {
	Mode     string `json:"mode"`
	NodeID   string `json:"node_id"`
	EdgeAddr string `json:"edge_addr"`
	PushOnly bool   `json:"push_only"`
}

type DeployPlanArtifact struct {
	Source        string `json:"source"`
	URL           string `json:"url,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	TargetPath    string `json:"target_path"`
	NeedsDownload bool   `json:"needs_download"`
	Platform      string `json:"platform,omitempty"`
}

type DeployPlanProvider struct {
	ExpectedID   string                        `json:"expected_id"`
	Installed    bool                          `json:"installed"`
	Running      bool                          `json:"running"`
	Matched      bool                          `json:"matched"`
	ConfigPath   string                        `json:"config_path,omitempty"`
	Status       string                        `json:"status"`
	Reason       string                        `json:"reason,omitempty"`
	Candidates   map[string]*deploy.BinaryInfo `json:"candidates,omitempty"`
	PortBindings []DeployPlanPortBinding       `json:"port_bindings,omitempty"`
}

type DeployPlanPortBinding struct {
	Port     int    `json:"port"`
	Process  string `json:"process"`
	Listen   string `json:"listen"`
	Expected bool   `json:"expected"`
}

type DeployPlanFile struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Backup bool   `json:"backup"`
	Reason string `json:"reason,omitempty"`
}

type DeployPlanService struct {
	Name   string `json:"name"`
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type DeployPlanCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ensureNodeMode = onboarding.Mode

const (
	ensureNodeJoinOnly ensureNodeMode = onboarding.ModeJoinOnly
	ensureNodeDeploy   ensureNodeMode = onboarding.ModeDeploy
)

const (
	controllerModeCurrent  = "current"
	controllerModePushOnly = "push_only"
)

type controlPeer struct {
	NodeID    string
	EdgeAddr  string
	Secret    string
	PushOnly  bool
	HostLabel string
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
	if req.ControllerMode == "" {
		req.ControllerMode = controllerModeCurrent
	}
	req.TargetIP = strings.TrimSpace(req.TargetIP)

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

	result, err := h.ensureNode(ctx, req, cpURL, ensureNodeDeploy, logf)
	if err != nil {
		return nil, err
	}
	return deployResponseFromEnsure(result), nil
}

func deployResponseFromEnsure(result *onboarding.EnsureResult) *DeployNodeResponse {
	if result == nil {
		return &DeployNodeResponse{Success: false, Message: "node ensure returned no result"}
	}
	return &DeployNodeResponse{
		Success:      result.Success,
		Action:       result.Action,
		NodeID:       result.NodeID,
		PeerAddr:     result.PeerAddr,
		Message:      result.Message,
		NextStep:     result.NextStep,
		Steps:        result.Steps,
		Capabilities: result.Capabilities,
	}
}

func (h *Handlers) AdminDeployPlan(w http.ResponseWriter, r *http.Request) {
	var req DeployNodeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	normalizeDeployRequest(&req)
	if req.TargetIP == "" {
		writeError(w, http.StatusBadRequest, "target_ip required")
		return
	}
	if h.Config == nil {
		writeJSON(w, http.StatusOK, DeployPlanResponse{Success: false, Error: "control plane config is not available"})
		return
	}
	control, err := h.resolveControlPeer(req, r.Host)
	if err != nil {
		writeJSON(w, http.StatusOK, DeployPlanResponse{Success: false, Error: err.Error()})
		return
	}
	report, err := deploy.Preflight(r.Context(), deploy.SSHConfig{
		Host:        req.TargetIP,
		User:        req.SSHUser,
		Port:        req.SSHPort,
		AuthMethod:  deploy.AuthMethod(req.AuthMethod),
		SSHKey:      req.SSHKey,
		SSHPassword: req.SSHPassword,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, DeployPlanResponse{Success: false, Error: err.Error()})
		return
	}
	plan := h.buildDeployPlan(req, control, report)
	writeJSON(w, http.StatusOK, DeployPlanResponse{Success: true, Plan: plan, Report: report})
}

func normalizeDeployRequest(req *DeployNodeRequest) {
	req.TargetIP = strings.TrimSpace(req.TargetIP)
	if req.AuthMethod == "" {
		req.AuthMethod = "key"
	}
	if req.SSHUser == "" {
		req.SSHUser = "root"
	}
	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.ControllerMode == "" {
		req.ControllerMode = controllerModeCurrent
	}
}

func (h *Handlers) resolveControlPeer(req DeployNodeRequest, cpURL string) (controlPeer, error) {
	mode := strings.TrimSpace(req.ControllerMode)
	if mode == "" {
		mode = controllerModeCurrent
	}
	switch mode {
	case controllerModeCurrent:
	case controllerModePushOnly:
	default:
		return controlPeer{}, fmt.Errorf("controller_mode must be %q or %q", controllerModeCurrent, controllerModePushOnly)
	}

	if mode == controllerModePushOnly {
		controlID := node.StableNodeID(strings.TrimSpace(req.ControlNodeID))
		if controlID == "" {
			return controlPeer{}, fmt.Errorf("control_node_id is required for push_only mode")
		}
		controlSecret := strings.TrimSpace(req.ControlSecret)
		if controlSecret == "" {
			return controlPeer{}, fmt.Errorf("control_secret is required for push_only mode")
		}
		controlAddr, err := normalizeEdgeAddr(req.ControlEdgeAddr)
		if err != nil {
			return controlPeer{}, err
		}
		return controlPeer{NodeID: controlID, EdgeAddr: controlAddr, Secret: controlSecret, PushOnly: true, HostLabel: controlAddr}, nil
	}

	controlID := strings.TrimSpace(h.Config.DistNode.ID)
	if controlID == "" {
		controlID = node.StableNodeID(h.Config.DistNode.Name)
	}
	if controlID == "" || controlID == "node_" {
		return controlPeer{}, fmt.Errorf("current control plane distnode id is not configured")
	}
	cpHost := edgeHost(cpURL, "")
	if isLocalControlHost(cpHost) {
		return controlPeer{}, fmt.Errorf("current UI/control plane is %q, which is not reachable from the target; use push_only mode with a public control endpoint", cpURL)
	}
	if cpHost == "" {
		cpHost = strings.TrimSpace(req.TargetIP)
	}
	controlAddr := net.JoinHostPort(cpHost, "80")
	return controlPeer{NodeID: controlID, EdgeAddr: controlAddr, Secret: h.Config.DistNode.Secret, HostLabel: cpHost}, nil
}

func (h *Handlers) buildDeployPlan(req DeployNodeRequest, control controlPeer, report *deploy.PreflightReport) *DeployPlan {
	action := "deploy"
	if report != nil && report.Aegis != nil && report.Aegis.Found {
		if report.Aegis.Running {
			action = "join"
		} else {
			action = "repair"
		}
	}
	targetOS, targetArch := targetPlatform(report)
	artifact := h.planArtifact(targetOS, targetArch)
	providerPlan := h.planProvider(report)
	files := h.planFiles(action, report)
	services := h.planServices(action, report)
	checks := h.planChecks(report, providerPlan)
	warnings := h.planWarnings(action, providerPlan, checks, control)
	manual := h.planManualActions(providerPlan, checks, control)

	target := DeployPlanTarget{Host: req.TargetIP, OS: targetOS, Arch: targetArch}
	if report != nil {
		if report.Aegis != nil {
			target.AegisFound = report.Aegis.Found
			target.AegisRunning = report.Aegis.Running
		}
		if report.Config != nil {
			target.ConfigFound = report.Config.Found
			target.ConfigPath = report.Config.Path
		}
	}

	canProceed := artifact.Source != "" && providerPlan.Status != "port_conflict" && providerPlan.Status != "unsupported_provider"
	return &DeployPlan{
		Action:     action,
		CanProceed: canProceed,
		Target:     target,
		Control: DeployPlanControl{
			Mode:     req.ControllerMode,
			NodeID:   control.NodeID,
			EdgeAddr: control.EdgeAddr,
			PushOnly: control.PushOnly,
		},
		Artifact:      artifact,
		Provider:      providerPlan,
		Files:         files,
		Services:      services,
		Checks:        checks,
		Warnings:      warnings,
		ManualActions: manual,
	}
}

func (h *Handlers) planArtifact(targetOS, targetArch string) DeployPlanArtifact {
	provider := newLocalAegisArtifactProvider()
	url := provider.artifactURL(targetOS, targetArch)
	sha := provider.expectedSHA256(targetOS, targetArch)
	platform := ""
	if targetOS != "" || targetArch != "" {
		platform = targetOS + "/" + targetArch
	}
	if url != "" {
		return DeployPlanArtifact{
			Source:        url,
			URL:           url,
			SHA256:        sha,
			TargetPath:    "/usr/local/bin/aegis",
			NeedsDownload: true,
			Platform:      platform,
		}
	}
	if targetOS == "" || targetArch == "" {
		return DeployPlanArtifact{Source: "current_binary", TargetPath: "/usr/local/bin/aegis", Platform: platform}
	}
	return DeployPlanArtifact{TargetPath: "/usr/local/bin/aegis", Platform: platform}
}

func (h *Handlers) planProvider(report *deploy.PreflightReport) DeployPlanProvider {
	expected := strings.TrimSpace(h.Config.Proxy.Provider)
	if expected == "" {
		expected = config.ProductionConfig().Proxy.Provider
	}
	out := DeployPlanProvider{
		ExpectedID: expected,
		Status:     "unknown",
		Candidates: map[string]*deploy.BinaryInfo{},
	}
	if report != nil {
		out.Candidates = report.Providers
	}
	if expected != "caddy" && expected != "haproxy" {
		out.Status = "unsupported_provider"
		out.Reason = "deployment planning currently understands caddy and haproxy only"
		return out
	}
	var info *deploy.BinaryInfo
	if report != nil && report.Providers != nil {
		info = report.Providers[expected]
	}
	if info == nil || !info.Found {
		out.Status = "provider_missing"
		out.Reason = "target does not have the expected provider installed"
		out.PortBindings = providerPortBindings(report, expected)
		return out
	}
	out.Installed = true
	out.Running = info.Running
	out.ConfigPath = info.ConfigPath
	out.Matched = true
	out.PortBindings = providerPortBindings(report, expected)
	if !info.Running {
		out.Status = "provider_stopped"
		out.Reason = "expected provider is installed but not running"
		return out
	}
	if hasUnexpectedPortOwner(out.PortBindings) {
		out.Status = "port_conflict"
		out.Reason = "80/443 is owned by a different process"
		return out
	}
	out.Status = "ready"
	out.Reason = "expected provider is installed and running"
	return out
}

func providerPortBindings(report *deploy.PreflightReport, expected string) []DeployPlanPortBinding {
	if report == nil {
		return nil
	}
	var out []DeployPlanPortBinding
	for _, p := range report.Ports {
		if p.Port != 80 && p.Port != 443 {
			continue
		}
		out = append(out, DeployPlanPortBinding{
			Port:     p.Port,
			Process:  p.Process,
			Listen:   p.Listen,
			Expected: strings.EqualFold(p.Process, expected) || strings.EqualFold(p.Process, "aegis"),
		})
	}
	return out
}

func hasUnexpectedPortOwner(bindings []DeployPlanPortBinding) bool {
	for _, b := range bindings {
		if !b.Expected {
			return true
		}
	}
	return false
}

func (h *Handlers) planFiles(action string, report *deploy.PreflightReport) []DeployPlanFile {
	files := []DeployPlanFile{
		{Path: "/usr/local/bin/aegis", Action: "install_or_replace", Backup: false, Reason: "node executable"},
		{Path: "/etc/aegis/config.yaml", Action: "write", Backup: true, Reason: "node runtime and distnode config"},
		{Path: "/etc/systemd/system/aegis.service", Action: "write", Backup: true, Reason: "service unit"},
		{Path: "/var/lib/aegis", Action: "ensure_dir", Backup: false, Reason: "data directory"},
		{Path: "/run/aegis", Action: "ensure_dir", Backup: false, Reason: "runtime directory"},
	}
	if action == "join" {
		files = []DeployPlanFile{{Path: "/etc/aegis/config.yaml", Action: "update_distnode_block", Backup: true, Reason: "join existing node to control plane"}}
	}
	if report != nil && report.Config != nil && report.Config.Found && report.Config.Path != "" && action != "join" {
		files[1].Path = report.Config.Path
		files[1].Action = "replace"
	}
	return files
}

func (h *Handlers) planServices(action string, report *deploy.PreflightReport) []DeployPlanService {
	if action == "join" {
		return []DeployPlanService{{Name: "aegis", Action: "restart", Reason: "reload distnode config"}}
	}
	return []DeployPlanService{{Name: "aegis", Action: "install_enable_restart", Reason: "run node after deployment"}}
}

func (h *Handlers) planChecks(report *deploy.PreflightReport, provider DeployPlanProvider) []DeployPlanCheck {
	checks := []DeployPlanCheck{
		{Name: "ssh", Status: "ok", Detail: "SSH preflight completed"},
		{Name: "artifact", Status: "ok", Detail: "artifact source resolved"},
		{Name: "provider", Status: provider.Status, Detail: provider.Reason},
		{Name: "local_health", Status: "planned", Detail: "verify http://127.0.0.1:7380/api/healthz after start"},
		{Name: "edge_health", Status: "planned", Detail: "verify target edge http://target:80/api/healthz"},
	}
	if report == nil || report.Host == nil || normalizeOS(report.Host.OS) != "linux" {
		checks = append(checks, DeployPlanCheck{Name: "platform", Status: "unsupported_os", Detail: "automatic service deployment currently targets Linux"})
	}
	return checks
}

func (h *Handlers) planWarnings(action string, provider DeployPlanProvider, checks []DeployPlanCheck, control controlPeer) []string {
	var warnings []string
	if action == "repair" {
		warnings = append(warnings, "Aegis is installed but not running; deployment may need service repair instead of a clean install.")
	}
	switch provider.Status {
	case "provider_missing":
		warnings = append(warnings, "Expected provider is missing; this version reports the issue but does not auto-install middleware.")
	case "provider_stopped":
		warnings = append(warnings, "Expected provider is installed but stopped; deployment can continue, but edge verification may fail until it is started.")
	case "port_conflict":
		warnings = append(warnings, "80/443 appears to be owned by another process; Aegis will not automatically take over it in this plan.")
	case "unsupported_provider":
		warnings = append(warnings, "The configured provider is not supported by deployment planning yet.")
	}
	if control.PushOnly {
		warnings = append(warnings, "Push-only mode writes the target to point at the public control node, but does not yet register the target on that public control plane.")
	}
	for _, c := range checks {
		if c.Status == "unsupported_os" {
			warnings = append(warnings, c.Detail)
		}
	}
	return warnings
}

func (h *Handlers) planManualActions(provider DeployPlanProvider, checks []DeployPlanCheck, control controlPeer) []string {
	var actions []string
	if provider.Status == "provider_missing" {
		actions = append(actions, "Install the expected provider or switch the cluster provider before deploying.")
	}
	if provider.Status == "port_conflict" {
		actions = append(actions, "Stop or migrate the process currently owning 80/443 before applying gateway config.")
	}
	if control.PushOnly {
		actions = append(actions, "Register the target node on the public control plane after SSH deployment succeeds.")
	}
	for _, c := range checks {
		if c.Status == "unsupported_os" {
			actions = append(actions, "Use a Linux target or provide a platform-specific deployment adapter.")
		}
	}
	return actions
}

func (h *Handlers) ensureNode(ctx context.Context, req DeployNodeRequest, cpURL string, mode ensureNodeMode, logf func(string, ...interface{})) (*onboarding.EnsureResult, error) {
	out := &onboarding.EnsureResult{Action: string(mode)}
	if logf == nil {
		logf = func(string, ...interface{}) {}
	}
	if h.Config == nil {
		out.Action = "not_configured"
		out.Message = "control plane config is not available"
		out.AddStep("config", onboarding.StepFailed, out.Message)
		return out, nil
	}
	control, err := h.resolveControlPeer(req, cpURL)
	if err != nil {
		out.Action = "control_plane_invalid"
		out.Message = err.Error()
		out.AddStep("control_plane", onboarding.StepFailed, err.Error())
		return out, nil
	}
	if !control.PushOnly && h.Config.DistNode.Secret == "" {
		out.Action = "not_configured"
		out.Message = "distnode secret is not configured on this control plane"
		out.AddStep("config", onboarding.StepFailed, out.Message)
		return out, nil
	}
	out.AddStep("control_plane", onboarding.StepOK, fmt.Sprintf("%s via %s", control.NodeID, control.EdgeAddr))
	logf("=== Ensuring Aegis node %s (%s) ===", req.TargetIP, mode)
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
		out.AddStep("connect", onboarding.StepFailed, err.Error())
		out.Message = fmt.Sprintf("SSH connection failed: %v", err)
		return out, nil
	}
	defer conn.Executor.Close()
	defer conn.Files.Close()
	logf("  SSH connection OK")
	out.AddStep("connect", onboarding.StepOK, "SSH connection OK")

	if mode == ensureNodeJoinOnly {
		logf("[2/8] Detecting existing Aegis installation...")
		report, err := deploy.PreflightConnection(ctx, conn)
		if err != nil {
			out.AddStep("preflight", onboarding.StepFailed, err.Error())
			out.Message = "SSH failed: " + err.Error()
			return out, nil
		}
		out.AddStep("preflight", onboarding.StepOK, "target inspected")
		if report == nil || report.Aegis == nil || !report.Aegis.Found {
			out.Action = "deploy_first"
			out.Message = "target does not have Aegis installed; deploy first"
			out.AddStep("detect_aegis", onboarding.StepFailed, "Aegis not installed")
			return out, nil
		}
		if !report.Aegis.Running {
			out.Action = "start_first"
			out.Message = "target Aegis is not running; start aegis first"
			out.AddStep("detect_aegis", onboarding.StepFailed, "Aegis not running")
			return out, nil
		}
		out.AddStep("detect_aegis", onboarding.StepOK, "Aegis is running")
		return h.joinExistingConnected(ctx, req, control, conn, out, logf), nil
	}

	return h.installAegisNodeConnected(ctx, req, control, conn, out, logf), nil
}

func (h *Handlers) joinExistingConnected(ctx context.Context, req DeployNodeRequest, control controlPeer, conn *deploy.Connection, out *onboarding.EnsureResult, logf func(string, ...interface{})) *onboarding.EnsureResult {
	logf("[3/8] Reading target identity...")
	hostResult := conn.Executor.Run(ctx, "hostname")
	targetHostname := strings.TrimSpace(hostResult.Stdout)
	if req.NodeName != "" {
		targetHostname = strings.TrimSpace(req.NodeName)
	}
	if targetHostname == "" {
		targetHostname = req.TargetIP
	}
	targetNodeID := node.StableNodeID(targetHostname)
	out.NodeID = targetNodeID

	bEdge := net.JoinHostPort(req.TargetIP, "80")
	out.PeerAddr = bEdge
	logf("  Target name: %s", targetHostname)
	logf("  Target node_id: %s", targetNodeID)
	logf("  Control edge: %s", control.EdgeAddr)
	logf("  Target edge: %s", bEdge)

	logf("[4/8] Registering target as distnode peer...")
	if control.PushOnly {
		out.AddStep("register_peer", onboarding.StepWarning, "push-only controller cannot update the public control plane peer list")
	} else if h.DistNode != nil {
		h.DistNode.Membership.AddPeer(distnode.PeerConfig{ID: targetNodeID, Addr: bEdge})
	}
	if !control.PushOnly && !slices.ContainsFunc(h.Config.DistNode.Peers, func(p config.DistNodePeer) bool { return p.ID == targetNodeID }) {
		h.Config.DistNode.Peers = append(h.Config.DistNode.Peers, config.DistNodePeer{ID: targetNodeID, Addr: bEdge})
		cpCfgPath := filepath.Join(h.Config.Runtime.ConfigDir, "config.yaml")
		if err := h.Config.Save(cpCfgPath); err != nil {
			out.Action = "join_failed"
			out.Message = "write control-plane config failed: " + err.Error()
			out.AddStep("register_peer", onboarding.StepFailed, err.Error())
			return out
		}
	}
	if !control.PushOnly {
		out.AddStep("register_peer", onboarding.StepOK, "target registered on control plane")
	}

	logf("[5/8] Writing target distnode peer config...")
	newBlock := fmt.Sprintf("distnode:\n  enabled: true\n  id: %q\n  name: %q\n  secret: %q\n  peers:\n    - id: %q\n      addr: %q\n",
		targetNodeID, targetHostname, control.Secret, control.NodeID, control.EdgeAddr)
	rewrite := "sudo cp /etc/aegis/config.yaml /etc/aegis/config.yaml.join-bak && " +
		"sudo awk 'BEGIN{s=0} /^distnode:/{s=1;next} s==1 && /^[^[:space:]]/{s=0} s==0{print}' /etc/aegis/config.yaml.join-bak | sudo tee /etc/aegis/config.yaml.new >/dev/null && " +
		fmt.Sprintf("cat <<'YAMLEOF' | sudo tee -a /etc/aegis/config.yaml.new >/dev/null\n%sYAMLEOF\n", newBlock) +
		"sudo mv /etc/aegis/config.yaml.new /etc/aegis/config.yaml"
	if res := conn.Executor.Run(ctx, rewrite); res.Error != nil || res.ExitCode != 0 {
		out.Action = "join_failed"
		out.Message = fmt.Sprintf("write target distnode config failed: exit=%d err=%v stderr=%s", res.ExitCode, res.Error, res.Stderr)
		out.AddStep("write_target_config", onboarding.StepFailed, res.Stderr)
		return out
	}
	out.AddStep("write_target_config", onboarding.StepOK, "target distnode config updated")

	logf("[6/8] Restarting target Aegis...")
	restart := conn.Executor.Run(ctx, "sudo systemctl restart aegis && sleep 3 && curl -s http://127.0.0.1:7380/api/healthz")
	if restart.Error != nil || restart.ExitCode != 0 || !strings.Contains(restart.Stdout, "alive") {
		out.Action = "join_failed"
		out.Message = fmt.Sprintf("target health check failed after restart: exit=%d stderr=%s", restart.ExitCode, restart.Stderr)
		out.NextStep = "SSH to target and run: sudo cp /etc/aegis/config.yaml.join-bak /etc/aegis/config.yaml && sudo systemctl restart aegis"
		out.AddStep("restart_target", onboarding.StepFailed, restart.Stderr)
		return out
	}
	out.AddStep("restart_target", onboarding.StepOK, "target restarted")

	logf("[7/8] Applying target provider config...")
	applyCmd := "TOK=$(sudo awk '/admin_token:/{print $2; exit}' /etc/aegis/config.yaml | tr -d '\"'); " +
		"curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:7380/api/apply -H \"Authorization: Bearer $TOK\" -H 'Content-Type: application/json'"
	applyRes := conn.Executor.Run(ctx, applyCmd)
	if applyCode := strings.TrimSpace(applyRes.Stdout); applyCode != "200" {
		out.Action = "join_failed"
		out.Message = "target joined but apply failed; http=" + applyCode
		out.NextStep = "Run sudo aegis apply on the target or retry from the UI."
		out.AddStep("target_apply", onboarding.StepFailed, "HTTP "+applyCode)
		return out
	}
	out.AddStep("target_apply", onboarding.StepOK, "target apply OK")

	out.Success = true
	if control.PushOnly {
		out.Action = "push_only_joined"
		out.NextStep = "target now points at the public control endpoint; register this target on that public control plane to make the cluster fully bidirectional."
	} else {
		out.Action = "joined"
		out.NextStep = "target restarted and applied; refresh the node list, it should show online soon."
	}
	out.Message = "node joined successfully - " + targetNodeID
	logf("=== Join complete: %s joined as %s ===", req.TargetIP, targetNodeID)
	return out
}

func (h *Handlers) installAegisNodeConnected(ctx context.Context, req DeployNodeRequest, control controlPeer, conn *deploy.Connection, out *onboarding.EnsureResult, logf func(string, ...interface{})) *onboarding.EnsureResult {
	logf("[2/8] Reading target identity...")
	hostResult := conn.Executor.Run(ctx, "hostname")
	targetName := strings.TrimSpace(hostResult.Stdout)
	if req.NodeName != "" {
		targetName = strings.TrimSpace(req.NodeName)
	}
	if targetName == "" {
		targetName = req.TargetIP
	}
	targetNodeID := node.StableNodeID(targetName)
	targetEdge := net.JoinHostPort(req.TargetIP, "80")
	logf("  Target name: %s", targetName)
	logf("  Target node_id: %s", targetNodeID)
	logf("  Control edge: %s", control.EdgeAddr)
	logf("  Target edge: %s", targetEdge)

	logf("[3/8] Creating directories...")
	result := conn.Executor.Run(ctx, "sudo mkdir -p /etc/aegis /var/lib/aegis/backups/db /var/lib/aegis/keys /run/aegis /usr/local/bin && sudo chown -R $(whoami):$(whoami) /var/lib/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		out.AddStep("prepare_dirs", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Create dirs failed: %v %s", result.Error, result.Stderr)
		return out
	}
	logf("  Directories ready")
	out.AddStep("prepare_dirs", onboarding.StepOK, "directories ready")

	logf("[4/8] Resolving and copying aegis binary...")
	report, err := deploy.PreflightConnection(ctx, conn)
	if err != nil {
		out.AddStep("preflight", onboarding.StepFailed, err.Error())
		out.Message = "Target preflight failed: " + err.Error()
		return out
	}
	artifact, err := newLocalAegisArtifactProvider().Resolve(ctx, report)
	if err != nil {
		out.AddStep("resolve_artifact", onboarding.StepFailed, err.Error())
		out.Message = "Resolve artifact failed: " + err.Error()
		return out
	}
	if artifact.Cleanup != nil {
		defer artifact.Cleanup()
	}
	logf("  Artifact: %s (%s)", artifact.LocalPath, artifact.Source)
	out.AddStep("resolve_artifact", onboarding.StepOK, artifact.Source)
	result = conn.Files.CopyTo(ctx, artifact.LocalPath, "/tmp/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		out.AddStep("upload_artifact", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Copy binary failed: %v %s", result.Error, result.Stderr)
		return out
	}
	result = conn.Executor.Run(ctx, "sudo install -m 0755 /tmp/aegis /usr/local/bin/aegis")
	if result.Error != nil || result.ExitCode != 0 {
		out.AddStep("install_artifact", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Install binary failed: %v %s", result.Error, result.Stderr)
		return out
	}
	logf("  Binary installed")
	out.AddStep("install_artifact", onboarding.StepOK, "binary installed")

	logf("[5/8] Writing /etc/aegis/config.yaml...")
	targetAdminToken := core.NewID("adm")
	cfgYAML, err := renderNodeServeConfig(h.Config.Proxy, targetName, targetAdminToken, control.Secret, control.NodeID, control.EdgeAddr)
	if err != nil {
		out.AddStep("render_config", onboarding.StepFailed, err.Error())
		out.Message = "Render node config failed: " + err.Error()
		return out
	}
	result = conn.Executor.Run(ctx, fmt.Sprintf("cat > /tmp/aegis-config.yaml << 'CFG'\n%s\nCFG\nsudo mv /tmp/aegis-config.yaml /etc/aegis/config.yaml && sudo chmod 600 /etc/aegis/config.yaml", cfgYAML))
	if result.Error != nil || result.ExitCode != 0 {
		out.AddStep("write_config", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Write config failed: %v %s", result.Error, result.Stderr)
		return out
	}
	logf("  config.yaml written")
	out.AddStep("write_config", onboarding.StepOK, "config.yaml written")

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
		out.AddStep("install_service", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Install service failed: %v %s", result.Error, result.Stderr)
		return out
	}
	logf("  aegis.service installed")
	out.AddStep("install_service", onboarding.StepOK, "aegis.service installed")

	logf("[7/8] Starting aegis serve...")
	result = conn.Services.Restart(ctx, "aegis")
	if result.Error != nil || result.ExitCode != 0 {
		out.AddStep("start_service", onboarding.StepFailed, result.Stderr)
		out.Message = fmt.Sprintf("Aegis service failed to start: %v %s", result.Error, result.Stderr)
		return out
	}
	health := conn.Executor.Run(ctx, "sleep 3; curl -s http://127.0.0.1:7380/api/healthz")
	if health.ExitCode != 0 || !strings.Contains(health.Stdout, "alive") {
		out.AddStep("local_health", onboarding.StepFailed, health.Stderr)
		out.Message = fmt.Sprintf("Local health check failed: %s %s", health.Stdout, health.Stderr)
		return out
	}
	logf("  Local control plane is healthy")
	out.AddStep("local_health", onboarding.StepOK, "local control plane is healthy")

	logf("[8/8] Registering distnode peer and validating edge...")
	if control.PushOnly {
		out.AddStep("register_peer", onboarding.StepWarning, "push-only controller cannot update the public control plane peer list")
	} else if h.DistNode != nil {
		h.DistNode.Membership.AddPeer(distnode.PeerConfig{ID: targetNodeID, Addr: targetEdge})
	}
	if !control.PushOnly && !slices.ContainsFunc(h.Config.DistNode.Peers, func(p config.DistNodePeer) bool { return p.ID == targetNodeID }) {
		h.Config.DistNode.Peers = append(h.Config.DistNode.Peers, config.DistNodePeer{ID: targetNodeID, Addr: targetEdge})
		cpCfgPath := filepath.Join(h.Config.Runtime.ConfigDir, "config.yaml")
		if err := h.Config.Save(cpCfgPath); err != nil {
			out.AddStep("register_peer", onboarding.StepFailed, err.Error())
			out.Message = "write control-plane config failed: " + err.Error()
			return out
		}
	}
	if !control.PushOnly {
		out.AddStep("register_peer", onboarding.StepOK, "peer registered")
	}
	if code := applyTarget(ctx, conn, targetAdminToken); code == "200" {
		logf("  Target apply OK")
		out.AddStep("target_apply", onboarding.StepOK, "target apply OK")
	} else {
		logf("  Warning: target apply returned HTTP %s; provider edge may need manual repair", code)
		out.AddStep("target_apply", onboarding.StepWarning, "HTTP "+code)
	}
	if err := waitHTTPAlive(ctx, "http://"+targetEdge+"/api/healthz", 12*time.Second); err != nil {
		out.NodeID = targetNodeID
		out.AddStep("edge_health", onboarding.StepFailed, err.Error())
		out.Message = "Aegis installed, but target 80 /api/healthz is not reachable: " + err.Error()
		return out
	}
	logf("  Target edge /api/healthz reachable")
	logf("=== Deploy complete: %s joined as %s ===", req.TargetIP, targetNodeID)

	out.Success = true
	if control.PushOnly {
		out.Action = "push_only_deployed"
		out.NextStep = "target was deployed and points at the public control endpoint; register this target on that public control plane to make the cluster fully bidirectional."
	} else {
		out.Action = string(onboarding.ModeDeploy)
	}
	out.NodeID = targetNodeID
	out.PeerAddr = targetEdge
	out.Message = fmt.Sprintf("Node deployed to %s and configured for distnode.", req.TargetIP)
	out.AddStep("edge_health", onboarding.StepOK, "target edge /api/healthz reachable")
	return out
}

func normalizeEdgeAddr(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", fmt.Errorf("control_edge_addr is required for push_only mode")
	}
	addr = strings.TrimPrefix(strings.TrimPrefix(addr, "http://"), "https://")
	addr = strings.TrimRight(addr, "/")
	host, port, err := net.SplitHostPort(addr)
	if err == nil {
		if strings.TrimSpace(host) == "" {
			return "", fmt.Errorf("control_edge_addr host is required")
		}
		if strings.TrimSpace(port) == "" {
			return "", fmt.Errorf("control_edge_addr port is required")
		}
		return net.JoinHostPort(host, port), nil
	}
	if strings.Contains(addr, ":") && strings.Count(addr, ":") > 1 {
		return "", fmt.Errorf("control_edge_addr must include a host and optional port")
	}
	return net.JoinHostPort(addr, "80"), nil
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

func isLocalControlHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return false
	}
	if host == "localhost" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func renderNodeServeConfig(controlProxy config.ProxyConfig, nodeName, adminToken, distSecret, controlPeerID, controlEdge string) (string, error) {
	nodeID := node.StableNodeID(nodeName)
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
		ID:      nodeID,
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
	return true
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

	var logBuf strings.Builder
	result, err := h.ensureNode(r.Context(), req, r.Host, ensureNodeJoinOnly, func(format string, args ...interface{}) {
		logBuf.WriteString(fmt.Sprintf(format+"\n", args...))
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := map[string]interface{}{
		"success":    result.Success,
		"action":     result.Action,
		"node_id":    result.NodeID,
		"target":     req.TargetIP,
		"peer_addr":  result.PeerAddr,
		"next_step":  result.NextStep,
		"steps":      result.Steps,
		"log_output": logBuf.String(),
	}
	if result.Success {
		resp["message"] = result.Message
	} else {
		resp["error"] = result.Message
	}
	writeJSON(w, http.StatusOK, resp)
	return
}
