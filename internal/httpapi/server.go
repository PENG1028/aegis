package httpapi

import (
	"net/http"

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
	"aegis/internal/gateway_link"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/nodeauth"
	"aegis/internal/topology"
	"aegis/internal/nodestate"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/project"
	"aegis/internal/relay"
	"aegis/internal/dns"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/token"
	"aegis/internal/trace"
)

// Services holds all application services for the HTTP API.
type Services struct {
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
	Auth          *token.AuthMiddleware
	Action        *action.ActionService
	Space         *space.AppService
	TokenRepo     *token.Repository
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	NodeSvc       *node.Service        // v1.8C
	NodeAuthSvc   *nodeauth.Service    // v1.8C
	Gateway       *gateway.GatewayService
	DepSvc        *deployment.Service
	PendingState    *cluster.PendingState  // v1.7S
	TraceSvc        *trace.Service         // v1.7T
	GatewayLinkSvc  *gatewaylink.Service   // v1.7AB
	SafetySvc       *safety.Service        // v1.8A
	RelaySvc        *relay.Resolver        // v1.8B
	NodeStateSvc    *nodestate.Service        // v1.8C-2
	GatewayInvRepo  *gateway.InventoryRepository // v1.8C-2
	GatewayInvSvc   *gateway.InventoryService   // v1.8C-2
	TopologySvc     *topology.Service           // v1.8C-2
	PolicySvc       *routingpolicy.Service       // v1.8C-3
	RoutingTableSvc *routingtable.Service        // v1.8C-3
	RelayHTTPHandler http.Handler           // v1.8B relay dispatch
	DNSMgmt         *dns.Manager            // v1.8E DNS resolver
}
