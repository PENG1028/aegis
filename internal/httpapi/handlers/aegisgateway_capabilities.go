package handlers

import (
	"net/http"

	"aegis/internal/action"
	"aegis/internal/aegisgateway"
)

func (h *Handlers) ServiceCapabilities(w http.ResponseWriter, r *http.Request) {
	if !isServiceCaller(r) {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	reg, err := h.newCapabilityRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": reg.List(),
	})
}

func (h *Handlers) ServiceCapabilityCall(w http.ResponseWriter, r *http.Request) {
	if !isServiceCaller(r) {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	name := r.PathValue("name")
	var req aegisgateway.CapabilityRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
	}
	reg, err := h.newCapabilityRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp, err := reg.Invoke(r.Context(), name, req)
	if err != nil {
		writeError(w, aegisgateway.ErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handlers) newCapabilityRegistry() (*aegisgateway.CapabilityRegistry, error) {
	reg := aegisgateway.NewCapabilityRegistry()
	if err := aegisgateway.RegisterNodeCapabilities(reg, h.NodeSvc); err != nil {
		return nil, err
	}
	return reg, nil
}

func isServiceCaller(r *http.Request) bool {
	ac := action.GetActionContext(r.Context())
	return ac != nil && ac.IsService()
}
