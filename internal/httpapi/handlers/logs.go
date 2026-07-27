package handlers

import (
	"aegis/internal/logs"
	"net/http"
)

func (h *Handlers) GetLogs(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	target := r.URL.Query().Get("target")

	entries, err := h.Logs.ListLogs(r.Context(), action, target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(entries))
	for i, e := range entries {
		result[i] = operationLogToMap(e)
	}
	writeJSON(w, http.StatusOK, result)
}

func operationLogToMap(l logs.OperationLog) map[string]interface{} {
	return map[string]interface{}{
		"id":          l.ID,
		"action":      l.Action,
		"target_type": l.TargetType,
		"target_id":   l.TargetID,
		"result":      l.Result,
		"message":     l.Message,
		"actor":       l.Actor,
		"created_at":  l.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
