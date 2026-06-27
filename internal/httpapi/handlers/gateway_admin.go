package handlers

import (
	"net/http"

	"aegis/internal/gateway"
)

// AdminListGateways handles GET /api/admin/v1/gateways
func (h *Handlers) AdminListGateways(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("node_id")
	var gws []gateway.GatewayInventory
	var err error
	if nodeID != "" {
		gws, err = h.GatewayInvRepo.FindByNodeID(nodeID)
	} else {
		gws, err = h.GatewayInvRepo.FindAll()
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if gws == nil {
		gws = []gateway.GatewayInventory{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"gateways": gws,
		"count":    len(gws),
	})
}

// AdminCreateGateway handles POST /api/admin/v1/gateways
func (h *Handlers) AdminCreateGateway(w http.ResponseWriter, r *http.Request) {
	var input gateway.CreateGatewayInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.NodeID == "" {
		writeError(w, http.StatusBadRequest, "node_id is required")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	gw, err := h.GatewayInvSvc.CreateGateway(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, gw)
}

// AdminGetGateway handles GET /api/admin/v1/gateways/{id}
func (h *Handlers) AdminGetGateway(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	gw, err := h.GatewayInvRepo.FindByID(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if gw == nil {
		writeError(w, http.StatusNotFound, "gateway not found")
		return
	}
	writeJSON(w, http.StatusOK, gw)
}

// AdminUpdateGateway handles PATCH /api/admin/v1/gateways/{id}
func (h *Handlers) AdminUpdateGateway(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var input gateway.UpdateGatewayInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	gw, err := h.GatewayInvSvc.UpdateGateway(id, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gw)
}

// AdminListNodeGateways handles GET /api/admin/v1/nodes/{id}/gateways
func (h *Handlers) AdminListNodeGateways(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	gws, err := h.GatewayInvRepo.FindByNodeID(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if gws == nil {
		gws = []gateway.GatewayInventory{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"gateways": gws,
		"count":    len(gws),
	})
}
