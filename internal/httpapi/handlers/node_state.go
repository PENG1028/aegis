package handlers

import (
	"net/http"
	"strconv"

	"aegis/internal/nodestate"
)

// NodeDesiredState handles GET /api/node/v1/desired-state
// Node pulls its own desired state (requires node credential auth).
func (h *Handlers) NodeDesiredState(w http.ResponseWriter, r *http.Request) {
	authNodeID := h.authenticateNodeRequest(w, r)
	if authNodeID == "" {
		return
	}

	// Optional revision parameter
	revStr := r.URL.Query().Get("revision")
	if revStr != "" {
		rev, err := strconv.Atoi(revStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid revision parameter")
			return
		}
		ds, err := h.NodeStateSvc.GetDesiredStateByRevision(authNodeID, rev)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ds == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "desired state not found"})
			return
		}
		writeJSON(w, http.StatusOK, ds)
		return
	}

	ds, err := h.NodeStateSvc.GetLatestDesiredState(authNodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ds == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"node_id":  authNodeID,
			"revision": 0,
			"status":   "no_desired_state",
		})
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

// NodeActualState handles POST /api/node/v1/actual-state
// Node reports its actual applied state.
func (h *Handlers) NodeActualState(w http.ResponseWriter, r *http.Request) {
	authNodeID := h.authenticateNodeRequest(w, r)
	if authNodeID == "" {
		return
	}

	var input struct {
		NodeID           string `json:"node_id"`
		AppliedRevision  int    `json:"applied_revision"`
		StateHash        string `json:"state_hash"`
		Status           string `json:"status"`
		ProviderStatus   string `json:"provider_status"`
		RelayStatus      string `json:"relay_status"`
		GatewayStatus    string `json:"gateway_status"`
		DiagnosticsStatus string `json:"diagnostics_status"`
		LastError        string `json:"last_error"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.NodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}
	if input.NodeID != authNodeID {
		writeError(w, http.StatusForbidden, "node credential does not match node_id")
		return
	}

	as, err := h.NodeStateSvc.ReportActualState(
		input.NodeID, input.AppliedRevision, input.StateHash,
		input.Status, input.LastError,
		input.ProviderStatus, input.RelayStatus, input.GatewayStatus, input.DiagnosticsStatus,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":          as.NodeID,
		"applied_revision": as.AppliedRevision,
		"status":           "accepted",
	})
}

// ============================================================================
// Admin APIs
// ============================================================================

// AdminGetDesiredState handles GET /api/admin/v1/nodes/{id}/desired-state
func (h *Handlers) AdminGetDesiredState(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	revStr := r.URL.Query().Get("revision")
	if revStr != "" {
		rev, err := strconv.Atoi(revStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid revision")
			return
		}
		ds, err := h.NodeStateSvc.GetDesiredStateByRevision(nodeID, rev)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if ds == nil {
			writeError(w, http.StatusNotFound, "desired state not found")
			return
		}
		writeJSON(w, http.StatusOK, ds)
		return
	}
	ds, err := h.NodeStateSvc.GetLatestDesiredState(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ds == nil {
		writeJSON(w, http.StatusOK, map[string]string{"node_id": nodeID, "status": "no_desired_state"})
		return
	}
	writeJSON(w, http.StatusOK, ds)
}

// AdminCreateDesiredState handles POST /api/admin/v1/nodes/{id}/desired-state
func (h *Handlers) AdminCreateDesiredState(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	var input struct {
		StateJSON string `json:"state_json"`
		Reason    string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.StateJSON == "" {
		writeError(w, http.StatusBadRequest, "state_json is required")
		return
	}

	ds, err := h.NodeStateSvc.CreateDesiredState(nodestate.CreateDesiredStateInput{
		NodeID:    nodeID,
		StateJSON: input.StateJSON,
		Reason:    input.Reason,
		CreatedBy: "admin",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ds)
}

// AdminGetActualState handles GET /api/admin/v1/nodes/{id}/actual-state
func (h *Handlers) AdminGetActualState(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	as, err := h.NodeStateSvc.GetActualState(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if as == nil {
		writeJSON(w, http.StatusOK, map[string]string{"node_id": nodeID, "status": "no_actual_state"})
		return
	}
	writeJSON(w, http.StatusOK, as)
}

// AdminGetSyncStatus handles GET /api/admin/v1/nodes/{id}/sync-status
func (h *Handlers) AdminGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	ss, err := h.NodeStateSvc.GetSyncStatus(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ss)
}
