package handlers

import (
	"net/http"
	"time"

	"aegis/internal/node"
)

// ============================================================================
// Node Admin API
// ============================================================================

// GetNode handles GET /api/admin/v1/nodes/{id}
func (h *Handlers) GetNode(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	n, err := h.NodeRepo.FindByNodeID(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		// Try by internal id
		n, err = h.NodeRepo.FindByID(nodeID)
		if err != nil || n == nil {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
	}

	// Compute effective status: heartbeat staleness + distnode liveness
	status := heartbeatEffectiveStatus(n.Status, n.LastHeartbeatAt)

	// Enrich with distnode membership status
	var distnodeAlive *bool
	var distnodeAddr string
	if h.DistNode != nil {
		distnodeAddr = h.DistNode.Config.Addr
		if p := h.DistNode.Membership.GetPeer(n.NodeID); p != nil {
			a := p.Alive
			distnodeAlive = &a
			if !p.Alive {
				status = "offline"
			} else if status == "unknown" {
				status = "online"
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node": map[string]interface{}{
			"id":                n.ID,
			"node_id":           n.NodeID,
			"name":              n.Name,
			"role":              n.Role,
			"status":            status,
			"hostname":          n.Hostname,
			"public_ip":         n.PublicIP,
			"private_ip":        n.PrivateIP,
			"region":            n.Region,
			"network_id":        n.NetworkID,
			"os":                n.OS,
			"arch":              n.Arch,
			"agent_version":     n.AgentVersion,
			"capabilities":      n.Capabilities,
			"is_leader":         n.IsLeader,
			"last_heartbeat_at": formatTime(n.LastHeartbeatAt),
			"last_seen":         formatTime(n.LastSeen),
			"last_error":        n.LastError,
			"created_at":        formatTime(n.CreatedAt),
			"updated_at":        formatTime(n.UpdatedAt),
			"distnode_alive":    distnodeAlive,
			"distnode_addr":     distnodeAddr,
		},
	})
}

// GetNodeHealth handles GET /api/admin/v1/nodes/{id}/health
func (h *Handlers) GetNodeHealth(w http.ResponseWriter, r *http.Request) {
	nodeID := r.PathValue("id")
	n, err := h.NodeRepo.FindByNodeID(nodeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n == nil {
		n, err = h.NodeRepo.FindByID(nodeID)
		if err != nil || n == nil {
			writeError(w, http.StatusNotFound, "node not found")
			return
		}
	}

	// Compute health status from heartbeat using shared helpers
	effStatus := heartbeatEffectiveStatus(n.Status, n.LastHeartbeatAt)
	healthStatus, healthy := nodeHealthAge(n.LastHeartbeatAt, effStatus)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":         n.NodeID,
		"healthy":         healthy,
		"health_status":   healthStatus,
		"status":          n.Status,
		"last_heartbeat":  formatTime(n.LastHeartbeatAt),
		"heartbeat_age_s": int(time.Since(n.LastHeartbeatAt).Seconds()),
		"agent_version":   n.AgentVersion,
		"last_error":      n.LastError,
	})
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// heartbeatEffectiveStatus returns the effective node status based on
// the stored status and heartbeat freshness.
//
// A node whose last heartbeat is older than 60s is treated as offline
// even if its DB status says "online". This is the canonical check —
// always use this function instead of duplicating the 60s timeout logic.
//
// DO NOT DUPLICATE THIS LOGIC. If you need to check node liveness,
// call heartbeatEffectiveStatus() or use distnode.Membership.GetPeer().
func heartbeatEffectiveStatus(dbStatus string, lastHeartbeat time.Time) string {
	if dbStatus == node.StatusOnline && !lastHeartbeat.IsZero() {
		if time.Since(lastHeartbeat) > 60*time.Second {
			return node.StatusOffline
		}
	}
	return dbStatus
}

// nodeHealthAge returns the health status string and healthy bool based on
// heartbeat freshness and effective status.
//
// DO NOT DUPLICATE THIS LOGIC. It is the single source of truth for
// translating heartbeat timestamps into health classifications.
func nodeHealthAge(lastHeartbeat time.Time, effectiveStatus string) (healthStatus string, healthy bool) {
	now := time.Now()
	age := now.Sub(lastHeartbeat)

	switch {
	case lastHeartbeat.IsZero():
		healthStatus = "never_contacted"
	case age < 30*time.Second:
		healthStatus = "healthy"
	case age < 60*time.Second:
		healthStatus = "stale"
	default:
		healthStatus = "unreachable"
	}
	healthy = healthStatus == "healthy"

	if effectiveStatus == node.StatusDegraded {
		healthStatus = "degraded"
		healthy = false
	}
	return
}
