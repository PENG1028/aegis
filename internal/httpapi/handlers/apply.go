package handlers

import (
	"net/http"
	"regexp"
	"strings"
)

// redactGatewaySecrets replaces gateway link secret values in rendered config
// with ***REDACTED*** so they are never exposed through preview/dry-run/diff APIs.
var redactGatewaySecretsRe = regexp.MustCompile(`(?m)(header_up X-Aegis-Gateway-Token\s+)"[^"]*"`)

func redactGatewaySecrets(config string) string {
	return redactGatewaySecretsRe.ReplaceAllString(config, `${1}"***REDACTED***"`)
}

// v1.8L: handlers now use h.Workflow (new orchestrator) instead of h.Apply.
// The old AppService is kept for backward compat during migration.

func (h *Handlers) ConfigPreview(w http.ResponseWriter, r *http.Request) {
	result, err := h.Workflow.Preview(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Flatten rendered configs into a single string for backward compat
	var rendered strings.Builder
	for path, content := range result.Rendered {
		rendered.WriteString("# " + path + "\n")
		rendered.WriteString(content)
		rendered.WriteString("\n")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_config": redactGatewaySecrets(rendered.String()),
		"warnings":        result.Plan.Warnings,
		"route_count":     len(result.Plan.Plans),
	})
}

func (h *Handlers) ConfigCurrent(w http.ResponseWriter, r *http.Request) {
	config, err := h.Workflow.GetCurrentConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config": redactGatewaySecrets(config),
	})
}

func (h *Handlers) ConfigDiff(w http.ResponseWriter, r *http.Request) {
	current, _ := h.Workflow.GetCurrentConfig()
	result, err := h.Workflow.Preview(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var rendered strings.Builder
	for _, content := range result.Rendered {
		rendered.WriteString(content)
	}

	diff := generateUnifiedDiff(redactGatewaySecrets(current), redactGatewaySecrets(rendered.String()))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"format":   "unified",
		"diff":     diff,
		"warnings": result.Plan.Warnings,
	})
}

func (h *Handlers) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	result, err := h.Workflow.TryApplyCtx(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":   result.Status,
		"warnings": result.Warnings,
		"provider": result.Provider,
	})
}

func (h *Handlers) ApplyDryRun(w http.ResponseWriter, r *http.Request) {
	result, err := h.Workflow.Preview(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var rendered strings.Builder
	for path, content := range result.Rendered {
		rendered.WriteString("# " + path + "\n")
		rendered.WriteString(content)
		rendered.WriteString("\n")
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_config": redactGatewaySecrets(rendered.String()),
		"warnings":        result.Plan.Warnings,
		"route_count":     len(result.Plan.Plans),
	})
}

func (h *Handlers) Rollback(w http.ResponseWriter, r *http.Request) {
	if err := h.Workflow.Rollback(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rolled_back"})
}

func (h *Handlers) ApplyHistory(w http.ResponseWriter, r *http.Request) {
	history, err := h.Workflow.History(r.Context())
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

func generateUnifiedDiff(current, preview string) string {
	if current == "" {
		return "--- (empty)\n+++ preview\n" + preview
	}
	if current == preview {
		return "(no changes)"
	}
	lines := "--- current\n+++ preview\n"
	cLines := splitLines(current)
	pLines := splitLines(preview)
	maxLen := len(cLines)
	if len(pLines) > maxLen {
		maxLen = len(pLines)
	}
	for i := 0; i < maxLen; i++ {
		cLine, pLine := "", ""
		if i < len(cLines) {
			cLine = cLines[i]
		}
		if i < len(pLines) {
			pLine = pLines[i]
		}
		if cLine != pLine {
			if cLine != "" {
				lines += "- " + cLine + "\n"
			}
			if pLine != "" {
				lines += "+ " + pLine + "\n"
			}
		}
	}
	return lines
}

func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
