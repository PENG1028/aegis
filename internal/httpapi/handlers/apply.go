package handlers

import (
	"aegis/internal/apply"
	"net/http"
	"regexp"
)

// redactGatewaySecrets replaces gateway link secret values in rendered config
// with ***REDACTED*** so they are never exposed through preview/dry-run/diff APIs.
var redactGatewaySecretsRe = regexp.MustCompile(`(?m)(header_up X-Aegis-Gateway-Token\s+)"[^"]*"`)

func redactGatewaySecrets(config string) string {
	return redactGatewaySecretsRe.ReplaceAllString(config, `${1}"***REDACTED***"`)
}

func (h *Handlers) ConfigPreview(w http.ResponseWriter, r *http.Request) {
	plan, err := h.Apply.DryRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_config":      redactGatewaySecrets(plan.RenderedConfig),
		"warnings":             plan.Warnings,
		"route_count":          plan.RouteCount,
		"managed_domain_count": plan.ManagedDomainCount,
		"skipped_count":        plan.SkippedCount,
	})
}

func (h *Handlers) ConfigCurrent(w http.ResponseWriter, r *http.Request) {
	config, err := h.Apply.GetCurrentConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"config": redactGatewaySecrets(config),
	})
}

func (h *Handlers) ConfigDiff(w http.ResponseWriter, r *http.Request) {
	current, _ := h.Apply.GetCurrentConfig()
	plan, err := h.Apply.DryRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	diff := generateUnifiedDiff(redactGatewaySecrets(current), redactGatewaySecrets(plan.RenderedConfig))
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"format":   "unified",
		"diff":     diff,
		"warnings": plan.Warnings,
	})
}

func (h *Handlers) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	plan, err := h.Apply.Apply(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":              plan.RenderedConfig[:min(20, len(plan.RenderedConfig))],
		"warnings":             plan.Warnings,
		"route_count":          plan.RouteCount,
		"managed_domain_count": plan.ManagedDomainCount,
	})
}

func (h *Handlers) ApplyDryRun(w http.ResponseWriter, r *http.Request) {
	plan, err := h.Apply.DryRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rendered_config":      redactGatewaySecrets(plan.RenderedConfig),
		"warnings":             plan.Warnings,
		"route_count":          plan.RouteCount,
		"managed_domain_count": plan.ManagedDomainCount,
		"skipped_count":        plan.SkippedCount,
	})
}

func (h *Handlers) Rollback(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Version string `json:"version"`
	}
	decodeJSON(r, &input)

	if err := h.Apply.Rollback(r.Context(), input.Version); err != nil {
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

func generateUnifiedDiff(current, preview string) string {
	if current == "" {
		return "--- (empty)\n+++ preview\n" + preview
	}
	if current == preview {
		return "(no changes)"
	}
	lines := "--- current\n+++ preview\n"
	// Simple line-by-line diff
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

// ensure apply.AppService is compatible
var _ = (*apply.AppService).Plan
