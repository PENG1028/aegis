package handlers

import (
	"net/http"
)

// AdminListTransparentRules handles GET /api/admin/v1/transparent/rules
func (h *Handlers) AdminListTransparentRules(w http.ResponseWriter, r *http.Request) {
	if h.TransparentMgr == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"rules":  []interface{}{},
			"count":  0,
			"message": "transparent proxy not configured",
		})
		return
	}

	rules := h.TransparentMgr.ListStatus()

	result := make([]map[string]interface{}, len(rules))
	for i, rs := range rules {
		result[i] = map[string]interface{}{
			"id":               rs.Rule.ID,
			"original_ip":      rs.Rule.OriginalIP,
			"original_port":    rs.Rule.OriginalPort,
			"local_proxy_port": rs.Rule.LocalProxyPort,
			"target_service":   rs.Rule.TargetServiceID,
			"target_node":      rs.Rule.TargetNodeID,
			"description":      rs.Rule.Description,
			"active":           rs.Active,
			"bytes_in":         rs.BytesIn,
			"bytes_out":        rs.BytesOut,
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rules": result,
		"count": len(result),
	})
}

// AdminDeleteTransparentRule handles DELETE /api/admin/v1/transparent/rules/{id}
func (h *Handlers) AdminDeleteTransparentRule(w http.ResponseWriter, r *http.Request) {
	if h.TransparentMgr == nil {
		writeError(w, http.StatusServiceUnavailable, "transparent proxy not configured")
		return
	}

	ruleID := r.PathValue("id")
	if ruleID == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	if err := h.TransparentMgr.StopRedirect(ruleID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "removed",
		"rule_id": ruleID,
	})
}
