package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"aegis/internal/hostdep/provider"
)

// ModePreview shows the impact of switching to a different runtime mode.
// POST /api/admin/v1/mode/preview?target=legacy
func (h *Handlers) ModePreview(w http.ResponseWriter, r *http.Request) {
	if h.ProvReg == nil || h.Route == nil {
		writeError(w, http.StatusNotImplemented, "mode preview not available")
		return
	}

	states := h.ProvReg.List()
	currentMode := provider.DetectRuntimeMode(states)

	targetModeID := r.URL.Query().Get("target")
	if targetModeID == "" {
		writeError(w, http.StatusBadRequest, "target mode is required (?target=legacy|edge_mux)")
		return
	}

	var targetMode *provider.RuntimeMode
	for _, m := range provider.AllRuntimeModes() {
		if m.ID == targetModeID {
			targetMode = &m
			break
		}
	}
	if targetMode == nil {
		writeError(w, http.StatusBadRequest, "unknown target mode: "+targetModeID)
		return
	}

	dbRoutes, err := h.Route.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	routes := make([]provider.RouteSpec, 0, len(dbRoutes))
	for _, rt := range dbRoutes {
		def := rt.CompDef()
		if def == nil {
			continue
		}
		routes = append(routes, provider.RouteSpec{
			Transport:   def.Transport,
			TLSMode:     def.TLSMode,
			AppProtocol: def.AppProtocol,
			Match: provider.MatchSpec{
				Host: rt.Domain,
				Path: rt.PathPrefix,
			},
		})
	}

	preview := provider.AnalyseModeSwitch(routes, currentMode, *targetMode)

	// RPCB: check each route's source_capabilities against target mode providers
	type routeConflict struct {
		RouteID         string   `json:"route_id"`
		Domain          string   `json:"domain"`
		Capabilities    []string `json:"capabilities"`
		Compatible      bool     `json:"compatible"`
		Reason          string   `json:"reason,omitempty"`
	}
	var rpcbConflicts []routeConflict
	for _, rt := range dbRoutes {
		if rt.SourceCapabilities == "" {
			continue
		}
		var caps []string
		if err := json.Unmarshal([]byte(rt.SourceCapabilities), &caps); err != nil || len(caps) == 0 {
			continue
		}
		compat := targetModeHasAllCaps(targetMode, caps)
		rc := routeConflict{
			RouteID:      rt.ID,
			Domain:       rt.Domain,
			Capabilities: caps,
			Compatible:   compat,
		}
		if !compat {
			rc.Reason = "缺少必需能力: " + missingCapsDesc(targetMode, caps)
		}
		rpcbConflicts = append(rpcbConflicts, rc)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"preview":         preview,
		"rpcb_conflicts":  rpcbConflicts,
	})
}

// ModeSwitch triggers a safe mode switch by running the Apply pipeline.
// The Apply pipeline already handles: mode detection → provider stop/start → config regeneration.
// POST /api/admin/v1/mode/switch
func (h *Handlers) ModeSwitch(w http.ResponseWriter, r *http.Request) {
	if h.ProvReg == nil || h.Apply == nil {
		writeError(w, http.StatusNotImplemented, "mode switch not available")
		return
	}

	var req struct {
		TargetMode   string `json:"target_mode"`
		ConfirmRisks bool   `json:"confirm_risks"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TargetMode == "" {
		writeError(w, http.StatusBadRequest, "target_mode is required")
		return
	}
	if !req.ConfirmRisks {
		writeError(w, http.StatusBadRequest, "you must confirm risks by setting confirm_risks=true")
		return
	}

	// Validate target mode exists
	states := h.ProvReg.List()
	currentMode := provider.DetectRuntimeMode(states)

	var targetMode *provider.RuntimeMode
	for _, m := range provider.AllRuntimeModes() {
		if m.ID == req.TargetMode {
			targetMode = &m
			break
		}
	}
	if targetMode == nil {
		writeError(w, http.StatusBadRequest, "unknown target mode: "+req.TargetMode)
		return
	}
	if currentMode.ID == targetMode.ID {
		writeError(w, http.StatusBadRequest, "already in target mode")
		return
	}

	// RPCB hard check: any route with incompatible capabilities → refuse
	dbRoutes, err := h.Route.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list routes: "+err.Error())
		return
	}
	var blocked []map[string]interface{}
	for _, rt := range dbRoutes {
		if rt.SourceCapabilities == "" {
			continue
		}
		var caps []string
		json.Unmarshal([]byte(rt.SourceCapabilities), &caps)
		if len(caps) > 0 && !targetModeHasAllCaps(targetMode, caps) {
			blocked = append(blocked, map[string]interface{}{
				"route_id":     rt.ID,
				"domain":       rt.Domain,
				"capabilities": caps,
				"reason":       "缺少能力：" + missingCapsDesc(targetMode, caps),
			})
		}
	}
	if len(blocked) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":    "RPCB_MODE_SWITCH_BLOCKED",
			"reason":   "存在路由所需能力在切换目标模式下无 Provider 支持",
			"blocked":  blocked,
			"hint":     "请先修改或删除冲突路由再重试",
		})
		return
	}

	// Execute standalone mode switch with snapshot/rollback.
	if err := h.Apply.SwitchMode(r.Context(), req.TargetMode); err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "failed",
			"error":    err.Error(),
			"rollback": "POST /api/rollback",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "已从 " + currentMode.Label + " 切换到 " + targetMode.Label,
	})
}

// targetModeHasAllCaps checks whether any provider in the target mode supports
// ALL of the given capabilities.
func targetModeHasAllCaps(mode *provider.RuntimeMode, caps []string) bool {
	if len(caps) == 0 {
		return true
	}
	for _, p := range mode.Providers {
		if provHasAllCaps(&p, caps) {
			return true
		}
	}
	return false
}

func provHasAllCaps(p *provider.ProviderAtoms, caps []string) bool {
	for _, cap := range caps {
		if _, ok := p.Bindings[cap]; !ok {
			return false
		}
	}
	return true
}

func missingCapsDesc(mode *provider.RuntimeMode, caps []string) string {
	var missing []string
	for _, cap := range caps {
		found := false
		for _, p := range mode.Providers {
			if _, ok := p.Bindings[cap]; ok {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, cap)
		}
	}
	return strings.Join(missing, ", ")
}
