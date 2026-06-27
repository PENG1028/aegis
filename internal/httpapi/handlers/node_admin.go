package handlers

import (
	"net/http"
	"time"

	"aegis/internal/nodeauth"
)

// ============================================================================
// Node Join Token Admin API
// ============================================================================

// CreateJoinToken handles POST /api/admin/v1/node-join-tokens
func (h *Handlers) CreateJoinToken(w http.ResponseWriter, r *http.Request) {
	var input nodeauth.CreateJoinTokenInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	t, rawToken, err := h.NodeAuthSvc.CreateJoinToken(input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":              t.ID,
		"name":            t.Name,
		"raw_join_token":  rawToken,
		"token_redacted":  false,
		"expires_at":      t.ExpiresAt.Format(time.RFC3339),
		"allowed_roles":   t.AllowedRoles,
		"expected_node_name": t.ExpectedNodeName,
		"allowed_source_cidr": t.AllowedSourceCIDR,
		"warning":         "store this token securely — it will not be shown again",
	})
}

// ListJoinTokens handles GET /api/admin/v1/node-join-tokens
func (h *Handlers) ListJoinTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.NodeAuthSvc.ListJoinTokens()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Redact token hash in responses
	type tokenInfo struct {
		ID               string   `json:"id"`
		Name             string   `json:"name"`
		AllowedRoles     []string `json:"allowed_roles"`
		ExpectedNodeName string   `json:"expected_node_name"`
		AllowedSourceCIDR string  `json:"allowed_source_cidr"`
		ExpiresAt        string   `json:"expires_at"`
		UsedAt           string   `json:"used_at,omitempty"`
		UsedByNodeID     string   `json:"used_by_node_id,omitempty"`
		RevokedAt        string   `json:"revoked_at,omitempty"`
		CreatedAt        string   `json:"created_at"`
	}
	infos := make([]tokenInfo, 0, len(tokens))
	for _, t := range tokens {
		info := tokenInfo{
			ID:                t.ID,
			Name:              t.Name,
			AllowedRoles:      t.AllowedRoles,
			ExpectedNodeName:  t.ExpectedNodeName,
			AllowedSourceCIDR: t.AllowedSourceCIDR,
			ExpiresAt:         t.ExpiresAt.Format(time.RFC3339),
			CreatedAt:         t.CreatedAt.Format(time.RFC3339),
		}
		if !t.UsedAt.IsZero() {
			info.UsedAt = t.UsedAt.Format(time.RFC3339)
			info.UsedByNodeID = t.UsedByNodeID
		}
		if !t.RevokedAt.IsZero() {
			info.RevokedAt = t.RevokedAt.Format(time.RFC3339)
		}
		infos = append(infos, info)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"join_tokens": infos,
		"count":       len(infos),
	})
}

// RevokeJoinToken handles POST /api/admin/v1/node-join-tokens/{id}/revoke
func (h *Handlers) RevokeJoinToken(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "token id is required")
		return
	}

	if err := h.NodeAuthSvc.RevokeJoinToken(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "join token revoked",
		"id":      id,
	})
}

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

	// Compute offline status if heartbeat is stale (> 60s)
	status := n.Status
	if status == "online" && !n.LastHeartbeatAt.IsZero() {
		if time.Since(n.LastHeartbeatAt) > 60*time.Second {
			status = "offline"
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

	// Compute health status from heartbeat
	now := time.Now()
	heartbeatAge := now.Sub(n.LastHeartbeatAt)
	healthStatus := "unknown"
	if n.LastHeartbeatAt.IsZero() {
		healthStatus = "never_contacted"
	} else if heartbeatAge < 30*time.Second {
		healthStatus = "healthy"
	} else if heartbeatAge < 60*time.Second {
		healthStatus = "stale"
	} else {
		healthStatus = "unreachable"
	}

	healthy := healthStatus == "healthy"
	if n.Status == "degraded" {
		healthStatus = "degraded"
		healthy = false
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":         n.NodeID,
		"healthy":         healthy,
		"health_status":   healthStatus,
		"status":          n.Status,
		"last_heartbeat":  formatTime(n.LastHeartbeatAt),
		"heartbeat_age_s": int(heartbeatAge.Seconds()),
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
