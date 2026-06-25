package handlers

import (
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/deployment"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/node"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/token"
	"aegis/internal/trace"
	"database/sql"
	"net/http"
	"time"
)

const rfc3339 = time.RFC3339

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
	Action        *action.ActionService
	Space         *space.AppService
	TokenRepo     *token.Repository
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	Gateway       *gateway.GatewayService
	DeploymentSvc *deployment.Service
	PendingState  *cluster.PendingState // v1.7S
	TraceSvc      *trace.Service        // v1.7T
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

	// v1.7S: Pending apply status
	pendingApply := map[string]interface{}{"pending": false}
	if h.PendingState != nil {
		ps := h.PendingState.Status()
		pendingApply = map[string]interface{}{
			"pending": ps.Pending,
			"since":   ps.Since,
			"reason":  ps.Reason,
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
		"last_apply":    lastApply,
		"pending_apply": pendingApply,
		"health": map[string]interface{}{
			"healthy_endpoints":   healthyCount,
			"unhealthy_endpoints": unhealthyCount,
			"unknown_endpoints":   unknownCount,
		},
	})
}
