package handlers

import (
	"errors"
	"net/http"
	"slices"

	"aegis/internal/apply"
	"aegis/internal/hostdep/provider"
	"aegis/internal/route"
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
		RouteID         string                     `json:"route_id"`
		Domain          string                     `json:"domain"`
		Capabilities    []string                   `json:"capabilities"`
		Compatible      bool                       `json:"compatible"`
		Reason          string                     `json:"reason,omitempty"`
		CurrentExecutor string                     `json:"current_executor,omitempty"`
		TargetExecutor  string                     `json:"target_executor,omitempty"`
		StateClass      provider.StateClass        `json:"state_class,omitempty"`
		Migration       provider.MigrationStrategy `json:"migration,omitempty"`
	}
	var rpcbConflicts []routeConflict
	for _, rt := range dbRoutes {
		caps := rt.CapabilityKeys()
		targetExecutor, semantics := routeExecutionForMode(rt, *targetMode, h.ProvReg)
		compat := targetExecutor != ""
		rc := routeConflict{
			RouteID:         rt.ID,
			Domain:          rt.Domain,
			Capabilities:    caps,
			Compatible:      compat,
			CurrentExecutor: currentRouteExecutor(rt),
			TargetExecutor:  targetExecutor,
			StateClass:      semantics.StateClass,
			Migration:       semantics.Migration,
		}
		if !compat {
			rc.Reason = "目标模式不支持组合：" + rt.Composition
		}
		rpcbConflicts = append(rpcbConflicts, rc)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"preview":        preview,
		"rpcb_conflicts": rpcbConflicts,
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
		if !routeSupportedInMode(rt, *targetMode, h.ProvReg) {
			blocked = append(blocked, map[string]interface{}{
				"route_id":     rt.ID,
				"domain":       rt.Domain,
				"composition":  rt.Composition,
				"capabilities": rt.CapabilityKeys(),
				"reason":       "目标模式不支持该路由组合",
			})
		}
	}
	if len(blocked) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":   "RPCB_MODE_SWITCH_BLOCKED",
			"reason":  "存在路由所需能力在切换目标模式下无 Provider 支持",
			"blocked": blocked,
			"hint":    "请先修改或删除冲突路由再重试",
		})
		return
	}

	// Execute standalone mode switch with snapshot/rollback.
	if err := h.Apply.SwitchMode(r.Context(), req.TargetMode); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"status":          "failed",
			"error":           err.Error(),
			"rollback_status": modeSwitchRollbackStatus(err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "success",
		"message": "已从 " + currentMode.Label + " 切换到 " + targetMode.Label,
	})
}

func modeSwitchRollbackStatus(err error) apply.RollbackStatus {
	var switchErr *apply.ModeSwitchError
	if errors.As(err, &switchErr) {
		return switchErr.RollbackStatus
	}
	return apply.RollbackNotRequired
}

func routeSupportedInMode(rt route.Route, mode provider.RuntimeMode, registry *provider.Registry) bool {
	if !provider.CompKeySupported(provider.CompKey(rt.Composition), mode) {
		return false
	}
	providerID, _ := routeExecutionForMode(rt, mode, registry)
	return providerID != ""
}

func currentRouteExecutor(rt route.Route) string {
	if rt.TLSBindingMode == route.TLSBindingProviderAuto && rt.TLSProvider != "" {
		return rt.TLSProvider
	}
	return rt.SourceProvider
}

// routeExecutionForMode preserves the current executor when possible, then
// selects a target executor according to lifecycle semantics. A portable asset
// may be reloaded elsewhere; provider-managed automatic TLS must be recreated.
func routeExecutionForMode(rt route.Route, mode provider.RuntimeMode, registry *provider.Registry) (string, provider.CapabilitySemantics) {
	var required provider.Capability
	preferred := rt.SourceProvider
	switch rt.TLSBindingMode {
	case route.TLSBindingProviderAuto:
		required = provider.CapAutoCert
		preferred = rt.TLSProvider
	case route.TLSBindingCertificate:
		required = provider.CapLoadCert
	default:
		def := rt.CompDef()
		switch {
		case def != nil && def.TLSMode == "passthrough":
			required = provider.CapTLSPassthrough
		case def != nil && def.Transport == "udp":
			required = provider.CapRawUDP
		case def != nil && def.AppProtocol == "http":
			required = provider.CapRouteHost
		default:
			required = provider.CapRawTCP
		}
	}
	semantics := provider.SemanticsOf(required)
	if registry == nil {
		return "", semantics
	}
	if preferred != "" && slices.Contains(mode.ProviderIDs(), preferred) {
		if p := registry.Get(preferred); p != nil {
			state := p.State()
			if state.Installed && state.HasCapability(required) {
				return preferred, semantics
			}
		}
	}
	for _, providerID := range mode.ProviderIDs() {
		p := registry.Get(providerID)
		if p == nil {
			continue
		}
		state := p.State()
		if state.Installed && state.HasCapability(required) {
			return providerID, semantics
		}
	}
	return "", semantics
}
