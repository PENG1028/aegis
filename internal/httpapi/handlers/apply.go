package handlers

import (
	"aegis/internal/apply"
	"net/http"
)

func (h *Handlers) ConfigPreview(w http.ResponseWriter, r *http.Request) {
	preview, err := h.Apply.Preview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (h *Handlers) ConfigDiff(w http.ResponseWriter, r *http.Request) {
	preview, err := h.Apply.Preview(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"diff":     preview.RenderedConfig,
		"warnings": preview.Warnings,
	})
}

func (h *Handlers) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	result, err := h.Apply.Apply(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) ApplyDryRun(w http.ResponseWriter, r *http.Request) {
	result, err := h.Apply.DryRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handlers) Rollback(w http.ResponseWriter, r *http.Request) {
	if err := h.Apply.Rollback(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rolled_back"})
}

func (h *Handlers) ApplyHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.Apply.History(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := make([]map[string]interface{}, len(history))
	for i, v := range history {
		result[i] = map[string]interface{}{
			"id":          v.ID,
			"version":     v.Version,
			"config_path": v.ConfigPath,
			"backup_path": v.BackupPath,
			"status":      v.Status,
			"message":     v.Message,
			"created_at":  v.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}
	writeJSON(w, http.StatusOK, result)
}

// ensure apply.AppService has Preview method
var _ = (*apply.AppService).Preview
