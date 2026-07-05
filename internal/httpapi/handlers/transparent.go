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
//
// Diagnosis checks:
//  1. Platform (Linux required for iptables)
//  2. iptables binary available
//  3. Root/sudo access
//  4. Gateway forward entry — derived from HTTP Route composition availability.
//     Transparent proxy needs ANY provider with route_host + upstream_tcp capability.
//     This is the same requirement as the "HTTP Route" binding composition.
func (h *Handlers) TransparentProxyStatus(w http.ResponseWriter, r *http.Request) {
	type check struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Detail string `json:"detail"`
	}

	type fwdTarget struct {
		Composition string `json:"composition"`
		ProviderID  string `json:"provider_id"`
		Host        string `json:"host"`
		Port        int    `json:"port"`
		ProviderOK  bool   `json:"provider_ok"`
	}

	checks := make([]check, 0, 4)

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

	// 4. Gateway forward entry — search by capability, not hardcoded provider name.
	//    Derived from HTTP Route composition = [listen_tcp, upstream_tcp, route_host].
	//    Any provider with those capabilities can serve as the transparent proxy target.
	states := h.ProvReg.List()
	mode := provider.DetectRuntimeMode(states)

	var forwardTargets []fwdTarget
	for _, p := range states {
		if !p.HasCapability(provider.CapRouteHost) || !p.HasCapability(provider.CapUpstreamTCP) {
			continue
		}
		listeners := mode.ListenerSpecsFor(p.ID)
		for _, l := range listeners {
			if l.Purpose == "http" || l.Purpose == "internal_https" {
				forwardTargets = append(forwardTargets, fwdTarget{
					Composition: "HTTP Route",
					ProviderID:  p.ID,
					Host:        "127.0.0.1",
					Port:        l.Port,
					ProviderOK:  p.Healthy(),
				})
				break
			}
		}
	}

	gatewayReady := len(forwardTargets) > 0 && forwardTargets[0].ProviderOK
	gatewayDetail := "无 Provider 提供 route_host + upstream_tcp 能力（需要 HTTP Route 组合）"
	if len(forwardTargets) > 0 {
		ft := forwardTargets[0]
		if ft.ProviderOK {
			gatewayDetail = fmt.Sprintf("%s → %s:%d（%s 已就绪，%s 模式）", ft.Composition, ft.Host, ft.Port, ft.ProviderID, mode.Label)
		} else {
			gatewayDetail = fmt.Sprintf("%s → %s:%d（%s 未安装，%s 模式）", ft.Composition, ft.Host, ft.Port, ft.ProviderID, mode.Label)
		}
	}
	checks = append(checks, check{
		Name:   "网关转发入口",
		Passed: gatewayReady,
		Detail: gatewayDetail,
	})

	allPassed := isLinux && iptErr == nil && isRoot && gatewayReady

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available":       allPassed,
		"checks":          checks,
		"forward_targets": forwardTargets,
		"composition":     "HTTP Route",
		"mode":            mode.Label,
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
