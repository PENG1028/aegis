package handlers

import (
	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/health"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/store"
	"database/sql"
	"net/http"
	"time"
)

// Handlers holds all service dependencies for HTTP handlers.
type Handlers struct {
	DB            *sql.DB
	Config        *config.Config
	Project       *project.AppService
	Service       *service.AppService
	EndpointRepo  *endpoint.Repository
	Route         *route.AppService
	ManagedDomain *manageddomain.AppService
	Exposure      *exposure.AppService
	Apply         *apply.AppService
	Health        *health.AppService
	Logs          *logs.AppService
}

// SystemStatus returns enhanced system status.
func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	// Counts
	projects, _ := h.Project.ListProjects(r.Context())
	services, _ := h.Service.ListServices(r.Context())
	routes, _ := h.Route.ListRoutes(r.Context())
	mdDomains, _ := h.ManagedDomain.ListManagedDomains(r.Context())

	// Last apply
	lastApply := map[string]interface{}{}
	history, err := h.Apply.History(r.Context())
	if err == nil && len(history) > 0 {
		last := history[0]
		lastApply = map[string]interface{}{
			"status":     last.Status,
			"version":    last.Version,
			"created_at": last.CreatedAt.Format(time.RFC3339),
		}
	}

	// Health summary
	healthChecks, _ := h.Health.GetLatestForAll(r.Context())
	healthyCount, unhealthyCount, unknownCount := 0, 0, 0
	for _, hc := range healthChecks {
		switch hc.Status {
		case "healthy":
			healthyCount++
		case "unhealthy":
			unhealthyCount++
		default:
			unknownCount++
		}
	}

	// Schema version
	schemaVersion := "unknown"
	if h.DB != nil {
		if v, err := store.GetCurrentVersion(h.DB); err == nil {
			schemaVersion = v
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":        "aegis",
		"version":     "0.x",
		"server_time": time.Now().Format(time.RFC3339),
		"proxy": map[string]interface{}{
			"provider":                  h.Config.Proxy.Provider,
			"config_path":               h.Config.Proxy.CaddyfilePath,
			"validate_available":        h.Config.Proxy.ValidateCommand != "",
			"reload_command_configured": h.Config.Proxy.ReloadCommand != "",
		},
		"store": map[string]interface{}{
			"sqlite_path":    h.Config.Store.SQLitePath,
			"schema_version": schemaVersion,
		},
		"counts": map[string]interface{}{
			"projects":         len(projects),
			"services":         len(services),
			"endpoints":        "n/a",
			"routes":           len(routes),
			"managed_domains":  len(mdDomains),
		},
		"last_apply": lastApply,
		"health": map[string]interface{}{
			"healthy_endpoints":   healthyCount,
			"unhealthy_endpoints": unhealthyCount,
			"unknown_endpoints":   unknownCount,
		},
	})
}
