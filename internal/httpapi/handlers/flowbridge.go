package handlers

import (
	"net/http"
	"strconv"

	"aegis/internal/flowbridge"
)

// ─── FlowBridge instance handlers (v1.9C-2) ───

func instanceToMap(inst *flowbridge.Instance) map[string]interface{} {
	if inst == nil {
		return nil
	}
	return map[string]interface{}{
		"id":                     inst.ID,
		"name":                   inst.Name,
		"machine_ip":             inst.MachineIP,
		"data_plane_port":        inst.DataPlanePort,
		"control_address":        inst.ControlAddress,
		"enabled":                inst.Enabled,
		"last_health_status":     inst.LastHealthStatus,
		"last_health_latency_ms": inst.LastHealthLatency,
		"last_health_message":    inst.LastHealthMessage,
		"last_checked_at":        inst.LastCheckedAt,
		"space_id":               inst.SpaceID,
		"owner_type":             inst.OwnerType,
		"owner_id":               inst.OwnerID,
		"created_by_token_id":    inst.CreatedByTokenID,
		"created_at":             inst.CreatedAt,
		"updated_at":             inst.UpdatedAt,
	}
}

// AdminListFlowBridge returns all flowbridge instances with route reference counts.
func (h *Handlers) AdminListFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	instances, err := h.FlowBridgeSvc.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list flowbridge instances: "+err.Error())
		return
	}
	limit, offset := paginationParams(r)
	result := make([]map[string]interface{}, 0, len(instances))
	for i := range instances {
		refCount := 0
		if h.Route != nil {
			routes, err := h.Route.FindRoutesByFlowBridgeID(r.Context(), instances[i].ID)
			if err == nil {
				refCount = len(routes)
			}
		}
		m := instanceToMap(&instances[i])
		m["route_ref_count"] = refCount
		result = append(result, m)
	}
	page := paginateSlice(result, limit, offset)
	if page == nil {
		page = []map[string]interface{}{}
	}
	writePaginatedJSON(w, http.StatusOK, page, len(result), limit, offset)
}

// AdminGetFlowBridge returns a single flowbridge instance.
func (h *Handlers) AdminGetFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	inst, err := h.FlowBridgeSvc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	m := instanceToMap(inst)
	if h.Route != nil {
		if routes, err := h.Route.FindRoutesByFlowBridgeID(r.Context(), inst.ID); err == nil {
			m["route_ref_count"] = len(routes)
		}
	}
	writeJSON(w, http.StatusOK, m)
}

// AdminCreateFlowBridge creates a flowbridge instance and runs an initial check.
func (h *Handlers) AdminCreateFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	var input flowbridge.CreateInstanceInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	inst, err := h.FlowBridgeSvc.Create(r.Context(), input, "", "admin", "", "")
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "FLOWBRIDGE_CREATE_FAILED", err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("flowbridge instance created: " + inst.ID)
	}
	writeJSON(w, http.StatusCreated, instanceToMap(inst))
}

// AdminUpdateFlowBridge partially updates a flowbridge instance.
func (h *Handlers) AdminUpdateFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	var input flowbridge.UpdateInstanceInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	inst, err := h.FlowBridgeSvc.Update(r.Context(), r.PathValue("id"), input)
	if err != nil {
		writeErrorCode(w, http.StatusBadRequest, "FLOWBRIDGE_UPDATE_FAILED", err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("flowbridge instance updated: " + inst.ID)
	}
	writeJSON(w, http.StatusOK, instanceToMap(inst))
}

// AdminCheckFlowBridge runs a live health check against the instance control plane.
func (h *Handlers) AdminCheckFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	inst, err := h.FlowBridgeSvc.Check(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("flowbridge instance health checked: " + inst.ID)
	}
	writeJSON(w, http.StatusOK, instanceToMap(inst))
}

// AdminDeleteFlowBridge deletes an unreferenced flowbridge instance.
func (h *Handlers) AdminDeleteFlowBridge(w http.ResponseWriter, r *http.Request) {
	if h.FlowBridgeSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "flowbridge support is unavailable")
		return
	}
	id := r.PathValue("id")
	if h.Route != nil {
		if routes, err := h.Route.FindRoutesByFlowBridgeID(r.Context(), id); err == nil && len(routes) > 0 {
			writeErrorCode(w, http.StatusConflict, "FLOWBRIDGE_REFERENCED",
				"flowbridge instance is referenced by "+strconv.Itoa(len(routes))+" route(s); unbind them first")
			return
		}
	}
	if err := h.FlowBridgeSvc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if h.PendingState != nil {
		_ = h.PendingState.MarkPending("flowbridge instance deleted: " + id)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": id})
}
