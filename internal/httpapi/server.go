package httpapi

import (
	"database/sql"

	"aegis/internal/acme"
	"aegis/internal/action"
	"aegis/internal/adminauth"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/credential"
	"aegis/internal/distnode"
	"aegis/internal/edgemux"
	"aegis/internal/egress"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/gateway"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/topology"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/project"
	"aegis/internal/hostdep/provider"
	"aegis/internal/dns"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/service"
	"aegis/internal/serviceauth"
	"aegis/internal/space"
	"aegis/internal/token"
	"aegis/internal/trace"
	"aegis/internal/certstore"
	"aegis/internal/transparent"
)

// Services holds all application services for the HTTP API.
type Services struct {
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
	Workflow      *apply.Workflow // v1.8L: new orchestrator (replaces Apply)
	Health        *health.AppService
	Logs          logs.Logger
	Auth          *token.AuthMiddleware
	Action        *action.ActionService
	Space         *space.AppService
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	NodeSvc       *node.Service        // v1.8C
	PendingState    *cluster.PendingState  // v1.7S
	TraceSvc        *trace.Service         // v1.7T
	GatewayLinkSvc  *gateway.LinkService   // v1.7AB
	SafetySvc       *safety.Service        // v1.8A
	RelaySvc        *gateway.Resolver        // v1.8B
	GatewayInvRepo  *gateway.InventoryRepository // v1.8C-2
	GatewayInvSvc   *gateway.InventoryService   // v1.8C-2
	TopologySvc     *topology.Service           // v1.8C-2
	PolicySvc       *routingpolicy.Service       // v1.8C-3
	RoutingTableSvc *routingtable.Service        // v1.8C-3
	DNSMgmt         *dns.Manager            // v1.8E DNS resolver
	CertStore       *certstore.Service       // v1.9C TLS certificate store
	ACMEClient     *acme.Client            // v1.9C ACME auto-cert manager
	TransparentMgr  *transparent.Manager    // v1.8H transparent IP:port proxy
	CredentialSvc   *credential.Service     // v1.8K encrypted connection strings
	ServiceAuthSvc  *serviceauth.Service    // v1.9A
	EgressSvc       *egress.Service         // v1.9A-5
	ProvReg         *provider.Registry      // v1.8L-19 — provider registry for install/uninstall/config handlers
	Version         string                  // build-injected version
	BuildTime       string                  // build-injected timestamp
	DistNode        *distnode.DistNode      // v1.9B distributed node runtime
	OnShutdown      func()                  // graceful shutdown hook — stops DNS, backups, reconcile, proxies
}
