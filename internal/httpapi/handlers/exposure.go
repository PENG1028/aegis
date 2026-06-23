package handlers

import (
	"aegis/internal/exposure"
	"net/http"
)

func (h *Handlers) ListExposures(w http.ResponseWriter, r *http.Request) {
	ownerRef := r.URL.Query().Get("owner_ref")
	var exposures []exposure.Exposure
	var err error

	if ownerRef != "" {
		exposures, err = h.Exposure.ListExposuresByOwner(r.Context(), ownerRef)
	} else {
		exposures, err = h.Exposure.ListExposures(r.Context())
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(exposures))
	for i, e := range exposures {
		result[i] = exposureToMap(e)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) CreateExposure(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Type      string `json:"type"`
		Mode      string `json:"mode"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Path      string `json:"path"`
		ServiceID string `json:"service_id"`
		NodeID    string `json:"node_id"`
		OwnerRef  string `json:"owner_ref"`
		TargetRef string `json:"target_ref"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	e, err := h.Exposure.CreateExposure(r.Context(), exposure.CreateExposureInput{
		Type: input.Type, Mode: input.Mode, Host: input.Host, Port: input.Port,
		Path: input.Path, ServiceID: input.ServiceID, NodeID: input.NodeID,
		OwnerRef: input.OwnerRef, TargetRef: input.TargetRef,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, exposureToMap(*e))
}

func (h *Handlers) GetExposure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	e, err := h.Exposure.GetExposure(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exposureToMap(*e))
}

func (h *Handlers) UpdateExposure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input struct {
		Host    *string `json:"host"`
		Port    *int    `json:"port"`
		Path    *string `json:"path"`
		Status  *string `json:"status"`
		Message *string `json:"message"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// OwnerRef from caller context (simplified: empty = admin)
	callerOwner := r.URL.Query().Get("caller_owner")
	e, err := h.Exposure.UpdateExposure(r.Context(), id, exposure.UpdateExposureInput{
		Host: input.Host, Port: input.Port, Path: input.Path,
		Status: input.Status, Message: input.Message,
	}, callerOwner)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exposureToMap(*e))
}

func (h *Handlers) ActivateExposure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	callerOwner := r.URL.Query().Get("caller_owner")
	e, err := h.Exposure.ActivateExposure(r.Context(), id, callerOwner)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exposureToMap(*e))
}

func (h *Handlers) DisableExposure(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	callerOwner := r.URL.Query().Get("caller_owner")
	e, err := h.Exposure.DisableExposure(r.Context(), id, callerOwner)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, exposureToMap(*e))
}

func (h *Handlers) GetExposureStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.Exposure.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func exposureToMap(e exposure.Exposure) map[string]interface{} {
	return map[string]interface{}{
		"id":         e.ID,
		"type":       e.Type,
		"mode":       e.Mode,
		"host":       e.Host,
		"port":       e.Port,
		"path":       e.Path,
		"service_id": e.ServiceID,
		"node_id":    e.NodeID,
		"owner_ref":  e.OwnerRef,
		"target_ref": e.TargetRef,
		"status":     e.Status,
		"message":    e.Message,
		"generates_config": exposure.GeneratesConfig(e.Type),
		"created_at": e.CreatedAt.Format("2006-01-02T15:04:05Z"),
		"updated_at": e.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
