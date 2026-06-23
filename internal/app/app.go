package app

import (
	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/health"
	"aegis/internal/logs"
	"aegis/internal/project"
	"aegis/internal/proxy"
	routesvc "aegis/internal/route"
	servicesvc "aegis/internal/service"
	"aegis/internal/store"
)

// App is the main application container that wires all services together.
type App struct {
	Config *config.Config
	Store  *store.Store

	// Application services
	ProjectService  *project.AppService
	ServiceService  *servicesvc.AppService
	RouteService    *routesvc.AppService
	ApplyService    *apply.AppService
	HealthService   *health.AppService
	LogService      *logs.AppService

	// Proxy adapter
	ProxyAdapter proxy.ProxyAdapter
}

// Services holds references to all application services for CLI access.
type Services struct {
	Project *project.AppService
	Service *servicesvc.AppService
	Route   *routesvc.AppService
	Apply   *apply.AppService
	Health  *health.AppService
	Logs    *logs.AppService
}
