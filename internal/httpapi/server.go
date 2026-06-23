package httpapi

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
	"aegis/internal/token"
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
}
