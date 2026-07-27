package handlers

import (
	"net/http"

	"aegis/internal/node"
)

// GetNodeCapabilities handles GET /api/admin/v1/nodes/{id}/capabilities
func (h *Handlers) GetNodeCapabilities(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	n, err := h.NodeRepo.FindByNodeID(nodeID)
	if err != nil || n == nil {
		writeError(w, http.StatusNotFound, "node not found: "+nodeID)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":       n.NodeID,
		"capabilities":  n.Capabilities,
		"disabled_actions": n.Capabilities.DisabledActions(),
	})
}

// RefreshNodeCapabilities handles POST /api/admin/v1/nodes/{id}/refresh-capabilities
func (h *Handlers) RefreshNodeCapabilities(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	n, err := h.NodeRepo.FindByNodeID(nodeID)
	if err != nil || n == nil {
		writeError(w, http.StatusNotFound, "node not found: "+nodeID)
		return
	}
	oldCaps := n.Capabilities
	n.Capabilities = node.DetectCapabilities()
	if err := h.NodeRepo.UpdateCapabilities(nodeID, n.Capabilities); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	diff := node.DiffCapabilities(oldCaps, n.Capabilities)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":      n.NodeID,
		"capabilities": n.Capabilities,
		"diff":         diff,
		"has_changes":  diff.HasDiff(),
	})
}
