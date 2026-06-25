package handlers

import (
	"net/http"
)

// CreateDeployment handles POST /api/admin/v1/deployments
func (h *Handlers) CreateDeployment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Version    string   `json:"version"`
		ServiceID  string   `json:"service_id"`
		TargetNodes []string `json:"target_nodes"`
		Strategy   string   `json:"strategy"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.ServiceID == "" || len(input.TargetNodes) == 0 {
		writeError(w, http.StatusBadRequest, "service_id and target_nodes are required")
		return
	}

	d, err := h.DeploymentSvc.CreateDeployment(r.Context(), input.Version, input.ServiceID, input.TargetNodes, input.Strategy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// ListDeployments handles GET /api/admin/v1/deployments
func (h *Handlers) ListDeployments(w http.ResponseWriter, r *http.Request) {
	ds, err := h.DeploymentSvc.ListDeployments(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deployments": ds, "count": len(ds)})
}

// GetDeployment handles GET /api/admin/v1/deployments/{id}
func (h *Handlers) GetDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	d, instances, err := h.DeploymentSvc.GetDeployment(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deployment": d,
		"instances":  instances,
	})
}

// RollbackDeployment handles POST /api/admin/v1/deployments/{id}/rollback
func (h *Handlers) RollbackDeployment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.DeploymentSvc.RollbackDeployment(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "deployment rolled back", "id": id})
}
