package httpapi

import (
	"aegis/internal/httpapi/handlers"
	"net/http"
)

// RegisterRoutes sets up all API routes on the given mux.
func RegisterRoutes(mux *http.ServeMux, svcs *Services) {
	h := &handlers.Handlers{
		Config:        svcs.Config,
		Project:       svcs.Project,
		Service:       svcs.Service,
		EndpointRepo:  svcs.EndpointRepo,
		Route:         svcs.Route,
		ManagedDomain: svcs.ManagedDomain,
		Apply:         svcs.Apply,
		Health:        svcs.Health,
		Logs:          svcs.Logs,
	}

	// System
	mux.HandleFunc("GET /api/system/status", h.SystemStatus)

	// Projects
	mux.HandleFunc("GET /api/projects", h.ListProjects)
	mux.HandleFunc("POST /api/projects", h.CreateProject)
	mux.HandleFunc("GET /api/projects/{id}", h.GetProject)
	mux.HandleFunc("PATCH /api/projects/{id}", h.UpdateProject)
	mux.HandleFunc("POST /api/projects/{id}/archive", h.ArchiveProject)

	// Services
	mux.HandleFunc("GET /api/services", h.ListServices)
	mux.HandleFunc("POST /api/services", h.CreateService)
	mux.HandleFunc("GET /api/services/{id}", h.GetService)
	mux.HandleFunc("PATCH /api/services/{id}", h.UpdateService)
	mux.HandleFunc("POST /api/services/{id}/enable", h.EnableService)
	mux.HandleFunc("POST /api/services/{id}/disable", h.DisableService)

	// Endpoints
	mux.HandleFunc("GET /api/services/{id}/endpoints", h.ListEndpoints)
	mux.HandleFunc("POST /api/services/{id}/endpoints", h.CreateEndpoint)
	mux.HandleFunc("PATCH /api/endpoints/{id}", h.UpdateEndpoint)
	mux.HandleFunc("POST /api/endpoints/{id}/enable", h.EnableEndpoint)
	mux.HandleFunc("POST /api/endpoints/{id}/disable", h.DisableEndpoint)
	mux.HandleFunc("DELETE /api/endpoints/{id}", h.DeleteEndpoint)

	// Routes
	mux.HandleFunc("GET /api/routes", h.ListRoutes)
	mux.HandleFunc("POST /api/routes", h.CreateRoute)
	mux.HandleFunc("GET /api/routes/{id}", h.GetRoute)
	mux.HandleFunc("PATCH /api/routes/{id}", h.UpdateRoute)
	mux.HandleFunc("POST /api/routes/{id}/enable", h.EnableRoute)
	mux.HandleFunc("POST /api/routes/{id}/disable", h.DisableRoute)
	mux.HandleFunc("POST /api/routes/{id}/switch-service", h.SwitchRouteService)
	mux.HandleFunc("POST /api/routes/{id}/maintenance-on", h.RouteMaintenanceOn)
	mux.HandleFunc("POST /api/routes/{id}/maintenance-off", h.RouteMaintenanceOff)

	// Managed Domains
	mux.HandleFunc("GET /api/managed-domains", h.ListManagedDomains)
	mux.HandleFunc("POST /api/managed-domains", h.CreateManagedDomain)
	mux.HandleFunc("GET /api/managed-domains/{id}", h.GetManagedDomain)
	mux.HandleFunc("POST /api/managed-domains/{id}/verify", h.VerifyManagedDomain)
	mux.HandleFunc("POST /api/managed-domains/{id}/enable", h.EnableManagedDomain)
	mux.HandleFunc("POST /api/managed-domains/{id}/disable", h.DisableManagedDomain)
	mux.HandleFunc("DELETE /api/managed-domains/{id}", h.DeleteManagedDomain)

	// Config / Apply
	mux.HandleFunc("GET /api/config/current", h.ConfigCurrent)
	mux.HandleFunc("GET /api/config/preview", h.ConfigPreview)
	mux.HandleFunc("GET /api/config/diff", h.ConfigDiff)
	mux.HandleFunc("POST /api/apply", h.ApplyConfig)
	mux.HandleFunc("POST /api/apply/dry-run", h.ApplyDryRun)
	mux.HandleFunc("POST /api/rollback", h.Rollback)
	mux.HandleFunc("GET /api/apply/history", h.ApplyHistory)

	// Diagnostics
	mux.HandleFunc("GET /api/diagnostics/export", h.DiagnosticsExport)

	// Health
	mux.HandleFunc("GET /api/health", h.GetHealth)
	mux.HandleFunc("POST /api/health/check-all", h.CheckAllHealth)
	mux.HandleFunc("GET /api/health/services/{id}", h.GetServiceHealth)

	// Logs
	mux.HandleFunc("GET /api/logs", h.GetLogs)

	// Settings
	mux.HandleFunc("GET /api/settings", h.GetSettings)
	mux.HandleFunc("PATCH /api/settings", h.UpdateSettings)
}
