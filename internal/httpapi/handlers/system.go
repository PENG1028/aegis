package handlers

import (
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/deployment"
	"aegis/internal/dns"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/gateway_link"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/nodeauth"
	"aegis/internal/nodestate"
	"aegis/internal/provider"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/topology"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/store"
	"aegis/internal/token"
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
	Health        *health.AppService
	Logs          logs.Logger
	Action        *action.ActionService
	Space         *space.AppService
	TokenRepo     *token.Repository
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	NodeSvc       *node.Service     // v1.8C
	NodeAuthSvc   *nodeauth.Service // v1.8C
	Gateway       *gateway.GatewayService
	DeploymentSvc *deployment.Service
	PendingState    *cluster.PendingState  // v1.7S
	TraceSvc        *trace.Service         // v1.7T
	GatewayLinkSvc  *gatewaylink.Service
	SafetySvc       *safety.Service        // v1.7AB
	RelayResolver   *RelayResolver         // v1.8B
	NodeStateSvc    *nodestate.Service        // v1.8C-2
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

// PortPolicy returns the current port allocation strategy for this node.
// GET /api/system/port-policy
//
// Returns the active port policy mode (legacy or edge_mux) and the list of
// port bindings that result from that mode. This is a public endpoint — no
// authentication required (the data is non-sensitive node metadata).
//
// Modes:
//
//	"legacy"   — Caddy owns :80 + :443 (no HAProxy detected)
//	"edge_mux" — HAProxy owns :443, Caddy owns :80 + :8443
//
// The frontend uses this to show which gateway types are available and
// whether TCP/UDP forwarding requires installing HAProxy first.
func (h *Handlers) PortPolicy(w http.ResponseWriter, r *http.Request) {
	mode := provider.CurrentPortPolicyMode()
	var policy provider.PortPolicy
	if mode == "edge_mux" {
		policy = provider.DefaultEdgeMuxPortPolicy()
	} else {
		policy = provider.DefaultLegacyPortPolicy()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mode":     policy.Mode,
		"bindings": policy.Bindings,
	})
}
