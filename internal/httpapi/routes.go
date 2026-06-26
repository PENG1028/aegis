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
		Exposure:      svcs.Exposure,
		Apply:         svcs.Apply,
		Health:        svcs.Health,
		Logs:          svcs.Logs,
		Action:        svcs.Action,
		Space:         svcs.Space,
		TokenRepo:     svcs.TokenRepo,
		AdminAuth:     svcs.AdminAuth,
		EdgeSvc:       svcs.EdgeSvc,
		ListenerSvc:   svcs.ListenerSvc,
		NodeRepo:      svcs.NodeRepo,
		Gateway:       svcs.Gateway,
		DeploymentSvc: svcs.DepSvc,
		PendingState:  svcs.PendingState,
		TraceSvc:      svcs.TraceSvc,
		SafetySvc:     svcs.SafetySvc,
		GatewayLinkSvc: svcs.GatewayLinkSvc,
		RelayResolver: &handlers.RelayResolver{Resolver: svcs.RelaySvc},
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

	// Exposures
	mux.HandleFunc("GET /api/exposures", h.ListExposures)
	mux.HandleFunc("POST /api/exposures", h.CreateExposure)
	mux.HandleFunc("GET /api/exposures/{id}", h.GetExposure)
	mux.HandleFunc("PATCH /api/exposures/{id}", h.UpdateExposure)
	mux.HandleFunc("POST /api/exposures/{id}/activate", h.ActivateExposure)
	mux.HandleFunc("POST /api/exposures/{id}/disable", h.DisableExposure)

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

	// v1.6 Action API
	mux.HandleFunc("POST /api/v1/actions/bind-http-domain", h.BindHTTPDomain)
	mux.HandleFunc("POST /api/v1/actions/bind-tls-backend", h.BindTLSBackend)
	mux.HandleFunc("PATCH /api/v1/actions/update-target", h.UpdateTarget)
	mux.HandleFunc("POST /api/v1/actions/disable-domain", h.DisableDomain)
	mux.HandleFunc("DELETE /api/v1/actions/domain", h.DeleteDomain)

	// v1.6 My resources
	mux.HandleFunc("GET /api/v1/my/routes", h.ListMyRoutes)
	mux.HandleFunc("GET /api/v1/my/services", h.ListMyServices)
	mux.HandleFunc("GET /api/v1/my/edge-rules", h.ListMyEdgeRules)
	mux.HandleFunc("GET /api/v1/my/operations", h.ListMyOperations)

	// v1.6B Admin API
	mux.HandleFunc("POST /api/admin/v1/auth/login", h.AdminLogin)
	mux.HandleFunc("POST /api/admin/v1/auth/logout", h.AdminLogout)
	mux.HandleFunc("GET /api/admin/v1/auth/me", h.AdminMe)
	mux.HandleFunc("GET /api/admin/v1/system/overview", h.SystemOverview)
	mux.HandleFunc("GET /api/admin/v1/nodes", h.AdminListNodes)
	mux.HandleFunc("GET /api/admin/v1/routes", h.AdminListRoutes)
	mux.HandleFunc("GET /api/admin/v1/edge-rules", h.AdminListEdgeRules)
	mux.HandleFunc("GET /api/admin/v1/services", h.AdminListServices)
	mux.HandleFunc("GET /api/admin/v1/scopes", h.AdminListScopes)
	mux.HandleFunc("POST /api/admin/v1/scopes", h.AdminCreateSpace)
	mux.HandleFunc("GET /api/admin/v1/api-keys", h.AdminListAPIKeys)
	mux.HandleFunc("POST /api/admin/v1/scopes/{id}/api-keys", h.AdminCreateAPIKey)
	mux.HandleFunc("POST /api/admin/v1/api-keys/{id}/revoke", h.AdminRevokeAPIKey)
	mux.HandleFunc("POST /api/admin/v1/api-keys/{id}/rotate", h.AdminRotateAPIKey)
	mux.HandleFunc("GET /api/admin/v1/operations", h.AdminListOperations)
	mux.HandleFunc("GET /api/admin/v1/apply-logs", h.AdminListApplyLogs)
	mux.HandleFunc("GET /api/admin/v1/audit-logs", h.AdminListAuditLogs)
	mux.HandleFunc("GET /api/admin/v1/node-events", h.AdminListNodeEvents)
	mux.HandleFunc("POST /api/admin/v1/system/doctor", h.AdminSystemDoctor)
	mux.HandleFunc("POST /api/admin/v1/system/verify", h.AdminSystemVerify)
	mux.HandleFunc("POST /api/admin/v1/system/apply", h.AdminSystemApply)

	// v1.7 Node Capabilities
	mux.HandleFunc("GET /api/admin/v1/nodes/{id}/capabilities", h.GetNodeCapabilities)
	mux.HandleFunc("POST /api/admin/v1/nodes/{id}/refresh-capabilities", h.RefreshNodeCapabilities)

	// v1.7 Gateway Abstraction
	mux.HandleFunc("POST /api/admin/v1/gateway/domains", h.CreateGatewayDomain)
	mux.HandleFunc("GET /api/admin/v1/gateway/domains", h.ListGatewayDomains)
	mux.HandleFunc("POST /api/admin/v1/gateway/routes", h.AttachGatewayRoute)
	mux.HandleFunc("DELETE /api/admin/v1/gateway/routes/{id}", h.DetachGatewayRoute)
	mux.HandleFunc("GET /api/admin/v1/gateway/listeners", h.ListGatewayListeners)
	mux.HandleFunc("PUT /api/admin/v1/gateway/domains/{id}/tls", h.UpdateTLSPolicy)

	// v1.7 Deployment Versioning
	mux.HandleFunc("POST /api/admin/v1/deployments", h.CreateDeployment)
	mux.HandleFunc("GET /api/admin/v1/deployments", h.ListDeployments)
	mux.HandleFunc("GET /api/admin/v1/deployments/{id}", h.GetDeployment)
	mux.HandleFunc("POST /api/admin/v1/deployments/{id}/rollback", h.RollbackDeployment)

	// v1.7S Provider Diagnostics
	mux.HandleFunc("GET /api/admin/v1/providers", h.ListProviders)
	mux.HandleFunc("POST /api/admin/v1/providers/diagnose", h.DiagnoseAllProviders)

	// v1.7T Access Path Trace (admin only, read-only)
		// v1.8A Route Safety & Egress Trace
		mux.HandleFunc("GET /api/admin/v1/routes/{id}/safety", h.CheckRouteSafety)
		mux.HandleFunc("GET /api/admin/v1/routes/safety", h.CheckAllRoutesSafety)
		mux.HandleFunc("GET /api/admin/v1/trace/egress", h.TraceEgress)
		// v1.7AB Gateway Links
		mux.HandleFunc("POST /api/admin/v1/gateway-links", h.CreateGatewayLink)
		mux.HandleFunc("GET /api/admin/v1/gateway-links", h.ListGatewayLinks)
		mux.HandleFunc("GET /api/admin/v1/gateway-links/{id}", h.GetGatewayLink)
		mux.HandleFunc("DELETE /api/admin/v1/gateway-links/{id}", h.DeleteGatewayLink)
		mux.HandleFunc("POST /api/admin/v1/gateway-links/{id}/rotate", h.RotateGatewayLinkSecret)
	mux.HandleFunc("GET /api/admin/v1/relay/resolve", h.ResolveRelay)
	// v1.8B relay dispatch — uses GatewayLink auth
	if svcs.RelayHTTPHandler != nil {
		mux.Handle("POST /__aegis/relay", svcs.RelayHTTPHandler)
	}
	mux.HandleFunc("GET /api/admin/v1/trace/domain/{domain}", h.TraceDomain)
	mux.HandleFunc("GET /api/admin/v1/trace/sni/{sni_host}", h.TraceSNI)
	mux.HandleFunc("GET /api/admin/v1/trace/route/{route_id}", h.TraceRoute)
}
