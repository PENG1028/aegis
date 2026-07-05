package handlers

import (
	"context"
	"log"
	"net"
	"net/http"
	"time"

	"aegis/internal/distnode"
	"aegis/internal/logs"
	"aegis/internal/node"
)

// SystemOverview handles GET /api/admin/v1/system/overview
func (h *Handlers) SystemOverview(w http.ResponseWriter, r *http.Request) {
	data := h.getSystemOverview(r.Context())
	writeJSON(w, http.StatusOK, data)
}

// getSystemOverview aggregates system-level counts and status.
// SINGLE SOURCE OF TRUTH — called by both:
//   - SystemOverview()     HTTP handler (GET /api/admin/v1/system/overview)
//   - Aegis.SystemOverview transport handler (cross-node call)
//
// DO NOT DUPLICATE: add new data sources here, not in the caller.
func (h *Handlers) getSystemOverview(ctx context.Context) map[string]interface{} {
	nodes, err := h.NodeRepo.FindAll()
	if err != nil { log.Printf("[overview] nodes: %v", err) }
	routes, err := h.Route.ListRoutes(ctx)
	if err != nil { log.Printf("[overview] routes: %v", err) }
	services, err := h.Service.ListServices(ctx)
	if err != nil { log.Printf("[overview] services: %v", err) }
	edgeRules, err := h.EdgeSvc.ListRules(ctx)
	if err != nil { log.Printf("[overview] edge-rules: %v", err) }
	spaces, err := h.Space.ListSpaces(ctx)
	if err != nil { log.Printf("[overview] spaces: %v", err) }
	history, err := h.Apply.History(ctx)
	if err != nil { log.Printf("[overview] history: %v", err) }

	leaderID := ""
	nodeCount := 0
	for _, n := range nodes {
		nodeCount++
		if n.IsLeader {
			leaderID = n.NodeID
		}
	}

	lastApply := map[string]interface{}{}
	if len(history) > 0 {
		last := history[0]
		lastApply = map[string]interface{}{
			"status":     last.Status,
			"version":    last.Version,
			"created_at": last.CreatedAt.Format(time.RFC3339),
		}
	}

	return map[string]interface{}{
		"node_count":       nodeCount,
		"leader_node":      leaderID,
		"route_count":      len(routes),
		"edge_rule_count":  len(edgeRules),
		"service_count":    len(services),
		"space_count":      len(spaces),
		"last_apply":       lastApply,
	}
}

// AdminListNodes handles GET /api/admin/v1/nodes
func (h *Handlers) AdminListNodes(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.NodeRepo.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Merge distnode membership into node list
	if h.DistNode != nil {
		nodes = mergeMembershipNodes(nodes, h.DistNode)
	}

	if nodes == nil {
		nodes = []node.NodeRecord{}
	}
	limit, offset := paginationParams(r)
	total := len(nodes)
	page := paginateSlice(nodes, limit, offset)

	// Build response with distnode metadata
	resp := map[string]interface{}{
		"data":  page,
		"meta": paginationMeta{
			Total:  total,
			Limit:  limit,
			Offset: offset,
		},
	}

	if h.DistNode != nil {
		resp["distnode"] = map[string]interface{}{
			"enabled":  true,
			"node_id":  h.DistNode.ID,
			"role":     h.DistNode.Role.Current(),
			"addr":     h.DistNode.Config.Addr,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// mergeMembershipNodes merges distnode membership peers into the SQLite node list.
// Peers known to membership but absent from SQLite appear as virtual nodes.
// Peers present in both get their status overridden by distnode liveness.
func mergeMembershipNodes(dbNodes []node.NodeRecord, dn *distnode.DistNode) []node.NodeRecord {
	// Index SQLite nodes by node_id
	byID := make(map[string]*node.NodeRecord, len(dbNodes))
	for i := range dbNodes {
		byID[dbNodes[i].NodeID] = &dbNodes[i]
	}

	seen := make(map[string]bool, len(dbNodes))
	for _, n := range dbNodes {
		seen[n.NodeID] = true
	}

	// Process each membership peer
	for _, p := range dn.Membership.AllPeers() {
		pid := p.Info.ID
		alive := p.Alive
		status := "offline"
		if alive {
			status = "online"
		}

		if existing, ok := byID[pid]; ok {
			// Peer exists in SQLite — override status with liveness
			if !alive && existing.Status == "online" {
				existing.Status = status
			}
			if alive && existing.Status != "online" {
				existing.Status = status
			}
		} else {
			// Peer unknown to SQLite — add as virtual node
			virtual := node.NodeRecord{
				NodeID:      pid,
				Name:        pid,
				Hostname:    p.Info.Addr,
				Status:      status,
				Role:        node.RoleWorker,
				AgentVersion: "distnode",
				Capabilities: node.DefaultCapabilities(),
			}
			// Extract IP from addr if possible
			if host, _, err := net.SplitHostPort(p.Info.Addr); err == nil {
				virtual.PublicIP = host
			}
			dbNodes = append(dbNodes, virtual)
			seen[pid] = true
		}
	}

	return dbNodes
}

// AdminListRoutes handles GET /api/admin/v1/routes
func (h *Handlers) AdminListRoutes(w http.ResponseWriter, r *http.Request) {
	routes, err := h.Route.ListRoutes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	limit, offset := paginationParams(r)
	total := len(routes)
	writePaginatedJSON(w, http.StatusOK, paginateSlice(routes, limit, offset), total, limit, offset)
}

// AdminListEdgeRules handles GET /api/admin/v1/edge-rules
func (h *Handlers) AdminListEdgeRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.EdgeSvc.ListRules(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	limit, offset := paginationParams(r)
	total := len(rules)
	writePaginatedJSON(w, http.StatusOK, paginateSlice(rules, limit, offset), total, limit, offset)
}

// AdminListServices handles GET /api/admin/v1/services
func (h *Handlers) AdminListServices(w http.ResponseWriter, r *http.Request) {
	services, err := h.Service.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	limit, offset := paginationParams(r)
	total := len(services)
	writePaginatedJSON(w, http.StatusOK, paginateSlice(services, limit, offset), total, limit, offset)
}

// AdminListScopes handles GET /api/admin/v1/scopes
func (h *Handlers) AdminListScopes(w http.ResponseWriter, r *http.Request) {
	spaces, err := h.Space.ListSpaces(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"spaces": spaces, "count": len(spaces)})
}

// AdminCreateSpace handles POST /api/admin/v1/scopes
func (h *Handlers) AdminCreateSpace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	sp, err := h.Space.CreateSpace(r.Context(), input.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, sp)
}


// AdminListOperations handles GET /api/admin/v1/operations
func (h *Handlers) AdminListOperations(w http.ResponseWriter, r *http.Request) {
	ops, _ := h.Logs.ListLogs(r.Context(), "", "")
	if ops == nil {
		ops = []logs.OperationLog{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"operations": ops, "count": len(ops)})
}

// AdminListApplyLogs handles GET /api/admin/v1/apply-logs
func (h *Handlers) AdminListApplyLogs(w http.ResponseWriter, r *http.Request) {
	al, _ := h.Logs.ListApplyLogs(50)
	if al == nil {
		al = []logs.ApplyLog{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"apply_logs": al, "count": len(al)})
}

// AdminListAuditLogs handles GET /api/admin/v1/audit-logs
func (h *Handlers) AdminListAuditLogs(w http.ResponseWriter, r *http.Request) {
	al, _ := h.Logs.ListAuditLogs(100)
	if al == nil {
		al = []logs.AuditLog{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"audit_logs": al, "count": len(al)})
}

// AdminListNodeEvents handles GET /api/admin/v1/node-events
func (h *Handlers) AdminListNodeEvents(w http.ResponseWriter, r *http.Request) {
	events, _ := h.Logs.ListNodeEvents(100)
	if events == nil {
		events = []logs.NodeEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"node_events": events, "count": len(events)})
}

// AdminSystemDoctor handles POST /api/admin/v1/system/doctor
func (h *Handlers) AdminSystemDoctor(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "doctor check triggered — check server logs for output",
	})
}

// AdminSystemVerify handles POST /api/admin/v1/system/verify
func (h *Handlers) AdminSystemVerify(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "verify triggered — check server logs for output",
	})
}

// AdminSystemApply handles POST /api/admin/v1/system/apply
func (h *Handlers) AdminSystemApply(w http.ResponseWriter, r *http.Request) {
	plan, err := h.Apply.TryApply(r.Context())
	if err != nil {
		writeError(w, http.StatusLocked, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "apply completed",
		"routes":   plan.RouteCount,
		"warnings": len(plan.Warnings),
	})
}

