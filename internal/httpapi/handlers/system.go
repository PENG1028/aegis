package handlers

import (
	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/health"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"
	"net/http"
)

// Handlers holds all service dependencies for HTTP handlers.
type Handlers struct {
	Config        *config.Config
	Project       *project.AppService
	Service       *service.AppService
	EndpointRepo  *endpoint.Repository
	Route         *route.AppService
	ManagedDomain *manageddomain.AppService
	Apply         *apply.AppService
	Health        *health.AppService
	Logs          *logs.AppService
}

// SystemStatus returns the system health status.
func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"version": "0.1.0",
		"proxy":   h.Config.Proxy.Provider,
	})
}
