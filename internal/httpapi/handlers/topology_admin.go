package handlers

import (
	"net/http"

	"aegis/internal/topology"
)

// AdminGetTopologyMatrix handles GET /api/admin/v1/topology/matrix
func (h *Handlers) AdminGetTopologyMatrix(w http.ResponseWriter, r *http.Request) {
	edges, err := h.TopologySvc.GetMatrix()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if edges == nil {
		edges = []topology.TopologyEdge{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"matrix": edges,
		"count":  len(edges),
	})
}

// AdminGetTopologyPath handles GET /api/admin/v1/topology/path
func (h *Handlers) AdminGetTopologyPath(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "from and to query parameters are required")
		return
	}
	path, err := h.TopologySvc.GetPath(from, to)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, path)
}

// AdminCreateTopologyEdge handles POST /api/admin/v1/topology/edges
func (h *Handlers) AdminCreateTopologyEdge(w http.ResponseWriter, r *http.Request) {
	var input topology.CreateEdgeInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	edge, err := h.TopologySvc.CreateOrUpdateEdge(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, edge)
}

// AdminUpdateTopologyEdge handles PATCH /api/admin/v1/topology/edges/{id}
func (h *Handlers) AdminUpdateTopologyEdge(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	// Edge ID in path - find the edge first
	allEdges, err := h.TopologySvc.ListEdges()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var found bool
	for _, edge := range allEdges {
		if edge.ID == id {
			found = true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "edge not found")
		return
	}

	// Update via full create-or-update from body
	var input topology.CreateEdgeInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	edge, err := h.TopologySvc.CreateOrUpdateEdge(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, edge)
}
