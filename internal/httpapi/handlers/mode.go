package handlers

import (
	"net/http"

	"aegis/internal/provider"
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
	writeJSON(w, http.StatusOK, preview)
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

	// Set target mode so Apply uses it instead of detected current mode
	h.Apply.SetTargetMode(req.TargetMode)

	// Execute Apply — the pipeline handles mode switching internally:
	//   1. PlanWithProviders regenerates config for the target mode
	//   2. Provider lifecycle (stop stale providers, start new ones)
	//   3. Validate → backup → write → reload
	//   4. Existing rollback path on failure
	plan, err := h.Apply.TryApply(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "failed",
			"error":   err.Error(),
			"rollback": "POST /api/rollback",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   "success",
		"message":  "已从 " + currentMode.Label + " 切换到 " + targetMode.Label,
		"warnings": plan.Warnings,
	})
}
