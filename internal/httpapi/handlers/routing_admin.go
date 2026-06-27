package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"aegis/internal/nodestate"
	"aegis/internal/routingtable"
)

// AdminGetNodeRoutingTable handles GET /api/admin/v1/nodes/{id}/routing-table
func (h *Handlers) AdminGetNodeRoutingTable(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}

	ds, err := h.NodeStateSvc.GetLatestDesiredState(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ds != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"node_id":          nodeID,
			"desired_revision": ds.Revision,
			"source":           "desired_state",
			"state_json":       ds.StateJSON,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id": nodeID,
		"entries": []interface{}{},
		"source":  "no_desired_state",
	})
}

// AdminGenerateNodeRoutingTable handles POST /api/admin/v1/nodes/{id}/routing-table/generate
func (h *Handlers) AdminGenerateNodeRoutingTable(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "node id is required")
		return
	}

	var req struct {
		Persist bool   `json:"persist"`
		Reason  string `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		req.Persist = false
	}

	genInput, err := h.buildGenerateInput(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build input: %v", err))
		return
	}

	table, validation, err := h.RoutingTableSvc.Preview(*genInput)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generate: %v", err))
		return
	}

	persistedRev := 0
	if req.Persist && len(table.Entries) > 0 {
		ds, err := h.persistRoutingTable(nodeID, table, req.Reason)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("persist: %v", err))
			return
		}
		persistedRev = ds.Revision
		table.Revision = ds.Revision
	}

	resp := map[string]interface{}{
		"node_id":   nodeID,
		"revision":  persistedRev,
		"entries":   table.Entries,
		"warnings":  table.Warnings,
		"persisted": req.Persist,
	}
	if validation != nil {
		resp["validation"] = map[string]interface{}{
			"is_valid": validation.IsValid,
			"errors":   validation.Errors,
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// AdminPreviewRoute handles GET /api/admin/v1/routing/preview?from_node_id=X&domain=Y
func (h *Handlers) AdminPreviewRoute(w http.ResponseWriter, r *http.Request) {
	fromNodeID := r.URL.Query().Get("from_node_id")
	domain := r.URL.Query().Get("domain")
	if fromNodeID == "" || domain == "" {
		writeError(w, http.StatusBadRequest, "from_node_id and domain are required")
		return
	}

	genInput, err := h.buildGenerateInput(fromNodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build input: %v", err))
		return
	}

	table, _, err := h.RoutingTableSvc.Preview(*genInput)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generate: %v", err))
		return
	}

	var matching []routingtable.RoutingTableEntry
	for _, entry := range table.Entries {
		if entry.Domain == domain {
			matching = append(matching, entry)
		}
	}
	if matching == nil {
		matching = []routingtable.RoutingTableEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"from_node_id": fromNodeID,
		"domain":       domain,
		"entries":      matching,
		"count":        len(matching),
	})
}

// AdminValidateNodeRouting handles GET /api/admin/v1/routing/validate?from_node_id=X
func (h *Handlers) AdminValidateNodeRouting(w http.ResponseWriter, r *http.Request) {
	nodeID := r.URL.Query().Get("from_node_id")
	if nodeID == "" {
		writeError(w, http.StatusBadRequest, "from_node_id is required")
		return
	}

	genInput, err := h.buildGenerateInput(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("build input: %v", err))
		return
	}

	table, _, err := h.RoutingTableSvc.Preview(*genInput)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("generate: %v", err))
		return
	}

	validation := h.RoutingTableSvc.ValidateTable(table)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":     nodeID,
		"is_valid":    validation.IsValid,
		"errors":      validation.Errors,
		"warnings":    validation.Warnings,
		"entry_count": len(table.Entries),
	})
}

// buildGenerateInput builds the routing table generator input from current DB state.
func (h *Handlers) buildGenerateInput(fromNodeID string, ctxs ...interface{}) (*routingtable.GenerateInput, error) {
	input := &routingtable.GenerateInput{
		FromNodeID: fromNodeID,
	}

	// Nodes
	if h.NodeRepo != nil {
		nodes, err := h.NodeRepo.FindAll()
		if err == nil {
			for _, n := range nodes {
				input.AllNodes = append(input.AllNodes, routingtable.NodeInfo{NodeID: n.NodeID})
			}
		}
	}

	// Services
	if h.Service != nil {
		svcs, err := h.Service.ListServices(context.Background())
		if err == nil {
			for _, svc := range svcs {
				input.AllServices = append(input.AllServices, routingtable.ServiceInfo{ServiceID: svc.ID})
			}
		}
	}

	// Routes
	if h.Route != nil {
		routes, err := h.Route.ListRoutes(context.Background())
		if err == nil {
			for _, rt := range routes {
				input.AllRoutes = append(input.AllRoutes, routingtable.RouteInfo{
					RouteID:   rt.ID,
					Domain:    rt.Domain,
					ServiceID: rt.ServiceID,
				})
			}
		}
	}

	// Endpoints (collect from all services)
	if h.EndpointRepo != nil {
		for _, svc := range input.AllServices {
			eps, err := h.EndpointRepo.FindByServiceID(svc.ServiceID)
			if err == nil {
				for _, ep := range eps {
					input.AllEndpoints = append(input.AllEndpoints, routingtable.EndpointInfo{
						EndpointID: ep.ID,
						ServiceID:  ep.ServiceID,
						Type:       ep.Type,
						Address:    ep.Address,
						NodeID:     ep.NodeID,
					})
				}
			}
		}
	}

	// Gateways
	if h.GatewayInvRepo != nil {
		gateways, err := h.GatewayInvRepo.FindAll()
		if err == nil {
			for _, gw := range gateways {
				input.AllGateways = append(input.AllGateways, routingtable.GatewayInfo{
					GatewayID:         gw.GatewayID,
					NodeID:            gw.NodeID,
					Name:              gw.Name,
					Type:              gw.Type,
					Provider:          gw.Provider,
					Host:              gw.Host,
					Port:              gw.Port,
					Scheme:            gw.Scheme,
					PublicAccessible:  gw.PublicAccessible,
					PrivateAccessible: gw.PrivateAccessible,
					Enabled:           gw.Enabled,
					Priority:          gw.Priority,
					Status:            gw.Status,
				})
			}
		}
	}

	// Topology edges
	if h.TopologySvc != nil {
		edges, err := h.TopologySvc.ListEdges()
		if err == nil {
			for _, e := range edges {
				input.TopologyEdges = append(input.TopologyEdges, routingtable.TopologyEdgeInfo{
					FromNodeID:       e.FromNodeID,
					ToNodeID:         e.ToNodeID,
					PrivateReachable: e.PrivateReachable,
					PublicReachable:  e.PublicReachable,
					Status:           e.Status,
					GatewayLinkID:    e.GatewayLinkID,
				})
			}
		}
	}

	// Gateway links
	if h.GatewayLinkSvc != nil {
		gwLinks, err := h.GatewayLinkSvc.List()
		if err == nil {
			for _, gl := range gwLinks {
				input.GatewayLinks = append(input.GatewayLinks, routingtable.GatewayLinkInfo{
					ID:           gl.ID,
					SourceNodeID: "",
					TargetNodeID: gl.TargetNodeID,
				})
			}
		}
	}

	// Policy resolver
	input.ResolvePolicy = func(routeID, serviceID string) (routingtable.PolicyInfo, error) {
		if h.PolicySvc == nil {
			def := routingtable.PolicyInfo{
				Mode: "auto", AllowLocal: true, AllowPrivate: true, AllowPublic: false,
				RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
			}
			return def, nil
		}
		resolved, err := h.PolicySvc.ResolvePolicy(routeID, serviceID)
		if err != nil {
			return routingtable.PolicyInfo{}, err
		}
		return routingtable.PolicyInfo{
			Source:             resolved.Source,
			Mode:               resolved.Mode,
			PrimaryGatewayID:   resolved.PrimaryGatewayID,
			FallbackGatewayIDs: resolved.FallbackGatewayIDs,
			AllowLocal:         resolved.AllowLocal,
			AllowPrivate:       resolved.AllowPrivate,
			AllowPublic:        resolved.AllowPublic,
			RequireGatewayLink: resolved.RequireGatewayLink,
			RequireRelay:       resolved.RequireRelay,
			PreserveHost:       resolved.PreserveHost,
			TLSMode:            resolved.TLSMode,
		}, nil
	}

	return input, nil
}

// persistRoutingTable saves the routing table as a new desired state revision.
func (h *Handlers) persistRoutingTable(nodeID string, table *routingtable.RoutingTable, reason string) (*nodestate.DesiredState, error) {
	if reason == "" {
		reason = "routing table auto-generate"
	}

	state := map[string]interface{}{
		"version":              1,
		"node_id":              nodeID,
		"generated_at":         time.Now().Format(time.RFC3339),
		"gateways":             []interface{}{},
		"listeners":            []interface{}{},
		"provider_configs":     []interface{}{},
		"relay_dispatch_routes": []interface{}{},
		"gateway_links":        []interface{}{},
		"local_routing_table":  table.Entries,
		"secrets":              []interface{}{},
		"diagnostics": map[string]interface{}{
			"enabled": true,
		},
	}

	data, err := json.Marshal(state)
	if err != nil {
		return nil, fmt.Errorf("marshal state: %w", err)
	}

	return h.NodeStateSvc.CreateDesiredState(nodestate.CreateDesiredStateInput{
		NodeID:    nodeID,
		StateJSON: string(data),
		Reason:    reason,
		CreatedBy: "admin",
	})
}
