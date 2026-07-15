package handlers

import (
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/distnode"
	"aegis/internal/dns"
	"aegis/internal/edgemux"
	"aegis/internal/acme"
	"aegis/internal/certstore"
	"aegis/internal/egress"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/hostdep/provider"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/topology"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/service"
	"aegis/internal/serviceauth"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/trace"
	"aegis/internal/transparent"
	"database/sql"
	"log"
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
	EndpointSvc   *endpoint.AppService
	Route         *route.AppService
	ManagedDomain *manageddomain.AppService
	Exposure      *exposure.AppService
	Apply         *apply.AppService
	Workflow      *apply.Workflow // v1.8L: new orchestrator
	Health        *health.AppService
	Logs          logs.Logger
	Action        *action.ActionService
	Space         *space.AppService
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	NodeSvc       *node.Service     // v1.8C
	PendingState    *cluster.PendingState  // v1.7S
	TraceSvc        *trace.Service         // v1.7T
	GatewayLinkSvc  *gateway.LinkService
	ServiceAuthSvc  *serviceauth.Service   // v1.9A
	EgressSvc       *egress.Service        // v1.9A-5
	CertStore       *certstore.Service      // v1.9C TLS certificate store
	ACMEClient     *acme.Client            // v1.9C ACME via lego (replaces certbot)
	SafetySvc       *safety.Service        // v1.7AB
	GatewayInvRepo  *gateway.InventoryRepository // v1.8C-2
	GatewayInvSvc   *gateway.InventoryService       // v1.8C-2
	DNSMgmt         *dns.Manager                    // v1.8E
	TopologySvc     *topology.Service           // v1.8C-2
		PolicySvc       *routingpolicy.Service       // v1.8C-3
		RoutingTableSvc *routingtable.Service        // v1.8C-3
	TransparentMgr  *transparent.Manager         // v1.8H
	ProvReg         *provider.Registry           // v1.8L-19 — provider registry for install/uninstall/config handlers
	Version         string // build-injected version
	BuildTime       string // build-injected timestamp
	DistNode        *distnode.DistNode // v1.9B distributed node runtime
		proxyMux        *http.ServeMux      // v1.9B cross-node view proxy
}

// SystemStatus returns enhanced system status.
func (h *Handlers) SystemStatus(w http.ResponseWriter, r *http.Request) {
	// Counts
	projects, err := h.Project.ListProjects(r.Context())
	if err != nil { log.Printf("[status] projects: %v", err) }
	services, err := h.Service.ListServices(r.Context())
	if err != nil { log.Printf("[status] services: %v", err) }
	routes, err := h.Route.ListRoutes(r.Context())
	if err != nil { log.Printf("[status] routes: %v", err) }
	mdDomains, err := h.ManagedDomain.ListManagedDomains(r.Context())
	if err != nil { log.Printf("[status] managed-domains: %v", err) }

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
	healthChecks, err := h.Health.GetLatestForAll(r.Context())
	if err != nil { log.Printf("[status] health: %v", err) }
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
		"version":     h.Version,
		"build_time":  h.BuildTime,
		"server_time": time.Now().Format(time.RFC3339),
		"proxy": map[string]interface{}{
			"provider":                  h.Config.Proxy.Provider,
			"config_path_configured":    h.Config.Proxy.CaddyfilePath != "",
			"validate_available":        h.Config.Proxy.ValidateCommand != "",
			"reload_command_configured": h.Config.Proxy.ReloadCommand != "",
		},
		"store": map[string]interface{}{
			"sqlite_configured": h.Config.Store.SQLitePath != "",
			"schema_version":    schemaVersion,
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

// RuntimeMode returns the current deployment mode and all available modes.
// GET /api/system/runtime-mode
//
// This replaces /api/system/port-policy. Instead of returning a flat list of
// port bindings, it returns the full RuntimeMode struct — the single source
// of truth consumed by both the Planner (backend) and the binding matrix (frontend).
//
// Response:
//
//	{
//	  "current": { RuntimeMode },
//	  "available_modes": [ RuntimeMode, ... ]
//	}
//
// The frontend uses this to render the binding matrix without hand-coded data.
func (h *Handlers) RuntimeMode(w http.ResponseWriter, r *http.Request) {
	states := h.ProvReg.List()
	current := provider.DetectRuntimeMode(states)
	allModes := provider.AllRuntimeModes()

	// Evaluate live composition status for each mode
	for i := range allModes {
		allModes[i].EvalAllCompositions(states)
	}
	current.EvalAllCompositions(states)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current":         current,
		"available_modes": allModes,
	})
}

// Compositions returns the canonical composition registry.
// GET /api/system/compositions
func (h *Handlers) Compositions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"compositions": provider.AllCompositions(),
	})
}
