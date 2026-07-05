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
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Detail string `json:"detail"`
	}

	type fwdEntry struct {
		Composition   string `json:"composition"`
		Status        string `json:"status"` // "available" | "provider_missing" | "unsupported"
		ProviderID    string `json:"provider_id,omitempty"`
		Host          string `json:"host,omitempty"`
		Port          int    `json:"port,omitempty"`
		ProviderOK    bool   `json:"provider_ok"`
		Detail        string `json:"detail"`
	}

	checks := make([]check, 0, 4)

	// 1. Platform
	isLinux := runtime.GOOS == "linux"
	platformDetail := runtime.GOOS
	if !isLinux {
		platformDetail = "透明代理需要 Linux iptables（当前: " + runtime.GOOS + "）"
	}
	checks = append(checks, check{
		Name: "Linux 系统", Passed: isLinux,
		Detail: platformDetail,
	})

	// 2. iptables
	_, iptErr := exec.LookPath("iptables")
	iptDetail := "iptables 已安装"
	if iptErr != nil {
		iptDetail = "未找到 iptables 命令（需要安装 iptables）"
	}
	checks = append(checks, check{
		Name: "iptables 可用", Passed: iptErr == nil,
		Detail: iptDetail,
	})

	// 3. Root / sudo
	isRoot := os.Geteuid() == 0
	_, sudoErr := exec.LookPath("sudo")
	rootDetail := "具有 root 权限"
	if !isRoot {
		if sudoErr == nil {
			rootDetail = "非 root，但 sudo 可用（iptables 可通过 sudo 执行）"
		} else {
			rootDetail = "非 root 且 sudo 不可用（iptables DNAT 需要 root 或 sudo）"
		}
	}
	checks = append(checks, check{
		Name: "Root / Sudo", Passed: isRoot || sudoErr == nil,
		Detail: rootDetail,
	})

	// 4. Gateway forward entries — iterate ALL compositions.
	//    Each composition declares whether it qualifies as a transparent proxy target.
	//    When new compositions are added to the registry, they auto-appear here.
	states := h.ProvReg.List()
	mode := provider.DetectRuntimeMode(states)
	mode.EvalAllCompositions(states) // same status path as diagnostic table

	var allForwardTargets []fwdEntry
	for _, c := range mode.Compositions {
		def := provider.LookupCompByName(c.Name)
		if def == nil {
			continue
		}
		entry := fwdEntry{Composition: c.Name, Status: c.Status}
		switch c.Status {
		case provider.CompUnsupported:
			entry.Detail = fmt.Sprintf("%s 模式不支持此组合能力", mode.Label)
		case provider.CompAvailable, provider.CompMissingProvider:
			for _, p := range states {
				hasAll := true
				for _, cap := range def.Requirements() {
					if !p.HasCapability(cap) {
						hasAll = false
						break
					}
				}
				if !hasAll {
					continue
				}
				listeners := mode.ListenerSpecsFor(p.ID)
				// Prefer listener matching composition's transport, fall back to first
				for _, l := range listeners {
					if l.Protocol == def.Transport {
						entry.ProviderID, entry.Host, entry.Port = p.ID, "127.0.0.1", l.Port
						break
					}
				}
				if entry.Port == 0 && len(listeners) > 0 {
					entry.ProviderID, entry.Host, entry.Port = p.ID, "127.0.0.1", listeners[0].Port
				}
				entry.ProviderOK = p.Healthy()
				if p.Healthy() {
					entry.Detail = fmt.Sprintf("%s → %s:%d（%s 已就绪）", c.Name, entry.Host, entry.Port, p.ID)
				} else {
					entry.Detail = fmt.Sprintf("需要 %s 提供 %s 能力（%s 未安装或未运行）", p.ID, c.Name, p.ID)
				}
			}
			if entry.ProviderID == "" {
				entry.Detail = fmt.Sprintf("需要 %s 组合能力，但无 Provider 声明所需能力", c.Name)
			}
		}
		allForwardTargets = append(allForwardTargets, entry)
	}

	// Gateway check passes if at least one forward target is available
	gatewayReady := false
	for _, ft := range allForwardTargets {
		if ft.Status == "available" {
			gatewayReady = true
			break
		}
	}

	gatewayDetail := "无可用转发入口"
	if gatewayReady {
		gatewayDetail = "至少一个转发入口已就绪"
	} else if len(allForwardTargets) > 0 {
		unavailable := 0
		for _, ft := range allForwardTargets {
			if ft.Status != "available" {
				unavailable++
			}
		}
		gatewayDetail = fmt.Sprintf("%d 个转发入口均不可用", unavailable)
	}
	checks = append(checks, check{
		Name:   "网关转发入口",
		Passed: gatewayReady,
		Detail: gatewayDetail,
	})

	allPassed := isLinux && iptErr == nil && (isRoot || sudoErr == nil) && gatewayReady

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"available":          allPassed,
		"checks":             checks,
		"forward_targets":    allForwardTargets,
		"mode":               mode.Label,
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
