package handlers

import (
	"net/http"
	"time"
)

// DiagnosticsExport generates a full diagnostics JSON for troubleshooting.
func (h *Handlers) DiagnosticsExport(w http.ResponseWriter, r *http.Request) {
	diag := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
	}

	// System status
	diag["system"] = map[string]interface{}{
		"name":    "aegis",
		"version": "0.x",
		"proxy": map[string]interface{}{
			"provider": h.Config.Proxy.Provider,
		},
	}

	// Settings (redacted)
	diag["settings"] = map[string]interface{}{
		"proxy": map[string]interface{}{
			"provider":        h.Config.Proxy.Provider,
			"caddyfile_path":  h.Config.Proxy.CaddyfilePath,
			"caddy_binary":    h.Config.Proxy.CaddyBinary,
			"reload_command":  h.Config.Proxy.ReloadCommand,
			"validate_command": h.Config.Proxy.ValidateCommand,
			"backup_dir":      h.Config.Proxy.BackupDir,
			// email is safe; admin_token intentionally omitted
		},
		"store": map[string]interface{}{
			"sqlite_path": h.Config.Store.SQLitePath,
		},
		// admin_token REDACTED
		"managed_domain": h.Config.ManagedDomain,
	}

	// Projects
	projects, _ := h.Project.ListProjects(r.Context())
	diag["projects"] = projects

	// Services
	services, _ := h.Service.ListServices(r.Context())
	diag["services"] = services

	// Endpoints (simplified)
	diag["endpoints"] = "use /api/services/:id/endpoints for details"

	// Routes
	routes, _ := h.Route.ListRoutes(r.Context())
	diag["routes"] = routes

	// Managed Domains
	mdDomains, _ := h.ManagedDomain.ListManagedDomains(r.Context())
	diag["managed_domains"] = mdDomains

	// Latest health checks
	healthChecks, _ := h.Health.GetLatestForAll(r.Context())
	diag["latest_health_checks"] = healthChecks

	// Apply history
	applyHistory, _ := h.Apply.History(r.Context())
	diag["apply_history"] = applyHistory

	// Operation logs (latest 200)
	logs, _ := h.Logs.ListLogs(r.Context(), "", "")
	if len(logs) > 200 {
		logs = logs[:200]
	}
	diag["operation_logs_latest_200"] = logs

	// Current config
	currentConfig, _ := h.Apply.GetCurrentConfig()
	diag["current_config"] = currentConfig

	// Preview config
	plan, err := h.Apply.DryRun(r.Context())
	if err == nil {
		diag["preview_config"] = plan.RenderedConfig
		// Collect warnings across all routes
		diag["warnings"] = plan.Warnings
	}

	writeJSON(w, http.StatusOK, diag)
}
