package handlers

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"aegis/internal/provider"
)

// TransparentProxyStatus returns the availability diagnosis for transparent proxy.
// GET /api/admin/v1/transparent/status
func (h *Handlers) TransparentProxyStatus(w http.ResponseWriter, r *http.Request) {
	type check struct {
		Name    string `json:"name"`
		Passed  bool   `json:"passed"`
		Detail  string `json:"detail"`
	}

	checks := make([]check, 0, 5)

	// 1. Platform
	isLinux := runtime.GOOS == "linux"
	checks = append(checks, check{
		Name: "Linux 系统", Passed: isLinux,
		Detail: map[bool]string{true: runtime.GOOS, false: "透明代理需要 Linux iptables"}[isLinux],
	})

	// 2. iptables binary
	_, iptErr := exec.LookPath("iptables")
	checks = append(checks, check{
		Name: "iptables 可用", Passed: iptErr == nil,
		Detail: map[bool]string{true: "iptables 已安装", false: "未找到 iptables 命令"}[iptErr == nil],
	})

	// 3. Root
	isRoot := os.Geteuid() == 0
	checks = append(checks, check{
		Name: "Root 权限", Passed: isRoot,
		Detail: map[bool]string{true: "具有 root 权限", false: "iptables DNAT 需要 root"}[isRoot],
	})

	// 4. Forward target — derived from RuntimeMode
	states := h.ProvReg.List()
	mode := provider.DetectRuntimeMode(states)
	forwardHost := "127.0.0.1"
	forwardPort := 80
	listeners := mode.ListenerSpecsFor("caddy")
	for _, l := range listeners {
		if l.Purpose == "http" || l.Purpose == "internal_https" {
			forwardPort = l.Port
			break
		}
	}
	checks = append(checks, check{
		Name:   "Caddy 转发口",
		Passed: true,
		Detail: fmt.Sprintf("%s → %s:%d", mode.Label, forwardHost, forwardPort),
	})

	// 5. Provider online
	caddyHealthy := false
	for _, s := range states {
		if s.ID == "caddy" && s.Healthy() {
			caddyHealthy = true
			break
		}
	}
	checks = append(checks, check{
		Name: "Caddy 运行中", Passed: caddyHealthy,
		Detail: map[bool]string{true: "Caddy 已安装并运行", false: "Caddy 未安装或未运行"}[caddyHealthy],
	})

	allPassed := true
	for _, c := range checks {
		if !c.Passed {
			allPassed = false
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available":     allPassed,
		"checks":        checks,
		"forward_host":  forwardHost,
		"forward_port":  forwardPort,
		"mode":          mode.Label,
	})
}

// AdminListTransparentRules handles GET /api/admin/v1/transparent/rules
func (h *Handlers) AdminListTransparentRules(w http.ResponseWriter, r *http.Request) {
	if h.TransparentMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"rules":  []interface{}{},
			"count":  0,
			"message": "transparent proxy not configured",
		})
		return
	}

	rules := h.TransparentMgr.ListStatus()

	result := make([]map[string]interface{}, len(rules))
	for i, rs := range rules {
		result[i] = map[string]interface{}{
			"id":               rs.Rule.ID,
			"original_ip":      rs.Rule.OriginalIP,
			"original_port":    rs.Rule.OriginalPort,
			"local_proxy_port": rs.Rule.LocalProxyPort,
			"target_service":   rs.Rule.TargetServiceID,
			"target_node":      rs.Rule.TargetNodeID,
			"description":      rs.Rule.Description,
			"active":           rs.Active,
			"bytes_in":         rs.BytesIn,
			"bytes_out":        rs.BytesOut,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": result,
		"count": len(result),
	})
}

// AdminDeleteTransparentRule handles DELETE /api/admin/v1/transparent/rules/{id}
func (h *Handlers) AdminDeleteTransparentRule(w http.ResponseWriter, r *http.Request) {
	if h.TransparentMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "transparent proxy not configured")
		return
	}

	ruleID := r.PathValue("id")
	if ruleID == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	if err := h.TransparentMgr.StopRedirect(ruleID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "removed",
		"rule_id": ruleID,
	})
}
