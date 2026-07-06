package handlers

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"aegis/internal/egress"
)

// AdminListEgressRules handles GET /api/admin/v1/egress/rules
func (h *Handlers) AdminListEgressRules(w http.ResponseWriter, r *http.Request) {
	if h.EgressSvc == nil {
		writeError(w, http.StatusNotImplemented, "egress not available")
		return
	}
	rules, err := h.EgressSvc.ListRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rules == nil {
		rules = []egress.EgressRule{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"rules": rules, "count": len(rules)})
}

// AdminCreateEgressRule handles POST /api/admin/v1/egress/rules
func (h *Handlers) AdminCreateEgressRule(w http.ResponseWriter, r *http.Request) {
	if h.EgressSvc == nil {
		writeError(w, http.StatusNotImplemented, "egress not available")
		return
	}
	var rule egress.EgressRule
	if err := decodeJSON(r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if err := h.EgressSvc.CreateRule(r.Context(), &rule); err != nil {
		if errors.Is(err, egress.ErrInvalidRule) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.PendingState != nil {
		h.PendingState.MarkPending("egress rule created: " + rule.ID)
	}
	writeJSON(w, http.StatusCreated, rule)
}

// AdminUpdateEgressRule handles PUT /api/admin/v1/egress/rules/{id}
func (h *Handlers) AdminUpdateEgressRule(w http.ResponseWriter, r *http.Request) {
	if h.EgressSvc == nil {
		writeError(w, http.StatusNotImplemented, "egress not available")
		return
	}
	id := r.PathValue("id")
	existing, err := h.EgressSvc.GetRule(r.Context(), id)
	if err != nil || existing == nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	var rule egress.EgressRule
	if err := decodeJSON(r, &rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	if err := h.EgressSvc.UpdateRule(r.Context(), &rule); err != nil {
		if errors.Is(err, egress.ErrInvalidRule) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

// AdminDeleteEgressRule handles DELETE /api/admin/v1/egress/rules/{id}
func (h *Handlers) AdminDeleteEgressRule(w http.ResponseWriter, r *http.Request) {
	if h.EgressSvc == nil {
		writeError(w, http.StatusNotImplemented, "egress not available")
		return
	}
	id := r.PathValue("id")
	if err := h.EgressSvc.DeleteRule(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id})
}

// AdminEgressCheck handles GET /api/admin/v1/egress/check
// Runs a full egress health check and returns results.
func (h *Handlers) AdminEgressCheck(w http.ResponseWriter, r *http.Request) {
	if h.EgressSvc == nil {
		writeError(w, http.StatusNotImplemented, "egress not available")
		return
	}

	type CheckResult struct {
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Detail string `json:"detail"`
	}

	results := []CheckResult{}

	// 1. DNS check
	if h.DNSMgmt != nil {
		running := h.DNSMgmt.IsActive()
		entryCount := len(h.DNSMgmt.Resolver.Table())
		results = append(results, CheckResult{
			Name:   "DNS 解析器",
			Passed: running,
			Detail: fmt.Sprintf("%s · %d 条记录", map[bool]string{true: "运行中", false: "已停"}[running], entryCount),
		})
	}

	// 2. Egress rules
	rules, _ := h.EgressSvc.ListRules(r.Context())
	activeRules := 0
	for _, rule := range rules {
		if rule.Status == "active" {
			activeRules++
		}
	}
	results = append(results, CheckResult{
		Name:   "出口规则",
		Passed: true,
		Detail: fmt.Sprintf("%d 条活跃规则", activeRules),
	})

	// 3. ServiceAuth status
	if h.ServiceAuthSvc != nil {
		services, _ := h.ServiceAuthSvc.ListServices(r.Context())
		activeSvcs := 0
		for _, s := range services {
			if s.Status == "active" {
				activeSvcs++
			}
		}
		results = append(results, CheckResult{
			Name:   "服务认证",
			Passed: activeSvcs > 0,
			Detail: fmt.Sprintf("%d 个在线服务", activeSvcs),
		})
	}

	allPassed := true
	for _, c := range results {
		if !c.Passed {
			allPassed = false
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"checks":  results,
		"healthy": allPassed,
	})
}

// AdminEgressToggle handles POST /api/admin/v1/egress/toggle
// Flips the global egress master switch on/off.
// When disabled: DNS bypasses managed domains, transparent proxy stops redirects.
func (h *Handlers) AdminEgressToggle(w http.ResponseWriter, r *http.Request) {
	if h.Config == nil {
		writeError(w, http.StatusInternalServerError, "config not available")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	h.Config.Egress.Enabled = body.Enabled

	// When disabled, stop DNS and transparent proxy
	if !body.Enabled {
		if h.DNSMgmt != nil && h.DNSMgmt.IsActive() {
			if err := h.DNSMgmt.Disable(); err != nil {
				log.Printf("[egress] toggle: dns disable failed: %v", err)
			}
		}
		if h.TransparentMgr != nil {
			h.TransparentMgr.StopAll()
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": h.Config.Egress.Enabled,
	})
}

// AdminEgressStatus returns the current egress master switch state.
func (h *Handlers) AdminEgressStatus(w http.ResponseWriter, r *http.Request) {
	enabled := true
	if h.Config != nil {
		enabled = h.Config.Egress.Enabled
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": enabled,
	})
}
