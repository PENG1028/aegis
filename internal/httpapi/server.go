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
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/project"
	"aegis/internal/route"
"aegis/internal/relay"
	"aegis/internal/service"
	"aegis/internal/space"
	"aegis/internal/token"
	"aegis/internal/safety"
	"aegis/internal/trace"
	"aegis/internal/gateway_link"
)

// Services holds all application services for the HTTP API.
type Services struct {
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
	Auth          *token.AuthMiddleware
	Action        *action.ActionService
	Space         *space.AppService
	TokenRepo     *token.Repository
	AdminAuth     *adminauth.Service
	EdgeSvc       *edgemux.AppService
	ListenerSvc   *listener.Service
	NodeRepo      *node.Repository
	Gateway       *gateway.GatewayService
	DepSvc        *deployment.Service
	PendingState    *cluster.PendingState  // v1.7S
	TraceSvc        *trace.Service         // v1.7T
	GatewayLinkSvc  *gatewaylink.Service   // v1.7AB
	SafetySvc       *safety.Service        // v1.8A
RelaySvc        *relay.Resolver        // v1.8B
	RelayHTTPHandler http.Handler           // v1.8B relay dispatch
}
