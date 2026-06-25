package cli

import (
	"aegis/internal/action"
	"aegis/internal/apply"
	"aegis/internal/cluster"
	"aegis/internal/config"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/health"
	"aegis/internal/listener"
	"aegis/internal/httpapi"
	"aegis/internal/node"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/smoke"
	"aegis/internal/space"
	"aegis/internal/trace"
	"database/sql"

	"github.com/spf13/cobra"
)

// Services holds all application services for CLI commands.
type Services struct {
	Config        *config.Config
	Project       *project.AppService
	Service       *service.AppService
	Route         *route.AppService
	EndpointRepo  *endpoint.Repository
	ManagedDomain *manageddomain.AppService
	Exposure      *exposure.AppService
	ListenerSvc   *listener.Service
	EdgeSvc       *edgemux.AppService
	LeaderSvc     *cluster.LeaderService
	NodeRepo      *node.Repository
	StateVer      *cluster.StateVersion
	DB            *sql.DB
	Apply         *apply.AppService
	Health        *health.AppService
	Logs          *logs.AppService
	Action        *action.ActionService
	Space         *space.AppService
	HTTPServices  *httpapi.Services
	PendingState  *cluster.PendingState
	TraceSvc      *trace.Service
}

// NewRootCommand creates the root aegis CLI command.
func NewRootCommand(svcs *Services) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aegis",
		Short: "Aegis - Infrastructure Gateway Control",
		Long: `Aegis manages proxy gateway configuration for multiple projects.
It handles Projects, Services, Endpoints, Routes, Managed Domains,
and safely applies configuration to Caddy (or Nginx in the future).

v0.x — Production-hardened gateway control with HTTP API.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Register subcommands
	cmd.AddCommand(newInitCommand())
	cmd.AddCommand(newBootstrapCommand(svcs.Config, svcs.ListenerSvc))
	cmd.AddCommand(newDoctorCommand(svcs.Config, svcs.ListenerSvc))
	cmd.AddCommand(newSnapshotCommand(svcs.Apply, svcs.Route, svcs.EdgeSvc, svcs.ListenerSvc, svcs.LeaderSvc, svcs.NodeRepo, svcs.StateVer))
	cmd.AddCommand(newVerifyCommand(svcs.Apply, svcs.Route, svcs.EdgeSvc, svcs.ListenerSvc))
	cmd.AddCommand(newCleanupCommand(svcs.DB))
	cmd.AddCommand(newProjectCommand(svcs.Project))
	cmd.AddCommand(newServiceCommand(svcs.Service, svcs.Project))
	cmd.AddCommand(newEndpointCommand(svcs.EndpointRepo, svcs.Service, svcs.Logs))
	cmd.AddCommand(newRouteCommand(svcs.Route, svcs.Service, svcs.Project))
	cmd.AddCommand(newManagedDomainCommand(svcs.ManagedDomain, svcs.Service))
	cmd.AddCommand(newExposureCommand(svcs.Exposure, svcs.Service))
	cmd.AddCommand(newListenerCommand(svcs.ListenerSvc))
	cmd.AddCommand(newEdgeCommand(svcs.EdgeSvc))
	cmd.AddCommand(newApplyCommand(svcs.Apply))
	cmd.AddCommand(newValidateCommand(svcs.Apply))
	cmd.AddCommand(newRollbackCommand(svcs.Apply))
	cmd.AddCommand(newConfigCommand(svcs.Apply))
	cmd.AddCommand(newHealthCommand(svcs.Health, svcs.Service))
	cmd.AddCommand(newMaintenanceCommand(svcs.Route))
	cmd.AddCommand(newLogsCommand(svcs.Logs))
	cmd.AddCommand(newSettingsCommand(svcs.Config))
	cmd.AddCommand(newDiagnosticsCommand(
		svcs.Config, svcs.Project, svcs.Service, svcs.Route,
		svcs.ManagedDomain, svcs.Apply, svcs.Health, svcs.Logs,
	))

	if svcs.Action != nil {
		cmd.AddCommand(newActionCommand(svcs.Action))
	}

	if svcs.HTTPServices != nil {
		cmd.AddCommand(newServeCommand(svcs.Config, svcs.HTTPServices))
		if svcs.HTTPServices.TraceSvc != nil {
			cmd.AddCommand(newTraceCommand(svcs.HTTPServices.TraceSvc))
		}
	}

	// Trace from svcs directly (for CLI use without serve)
	if svcs.TraceSvc != nil && svcs.HTTPServices == nil {
		cmd.AddCommand(newTraceCommand(svcs.TraceSvc))
	}

	// Smoke commands
	smokeSvc := buildSmokeService(svcs)
	if smokeSvc != nil {
		cmd.AddCommand(newSmokeCommand(smokeSvc))
	}

	return cmd
}

// buildSmokeService creates a smoke service from CLI services.
// Returns nil if required dependencies are missing.
func buildSmokeService(svcs *Services) *smoke.Service {
	if svcs.DB == nil {
		return nil
	}

	deps := smoke.Dependencies{
		Config:      svcs.Config,
		DB:          svcs.DB,
		ApplySvc:    svcs.Apply,
		HealthSvc:   svcs.Health,
		LogSvc:      svcs.Logs,
		RouteSvc:    svcs.Route,
		ListenerSvc: svcs.ListenerSvc,
		StateVer:    svcs.StateVer,
		PendingSt:   svcs.PendingState,
	}

	// Trace service from HTTP services or direct
	if svcs.HTTPServices != nil && svcs.HTTPServices.TraceSvc != nil {
		deps.TraceSvc = svcs.HTTPServices.TraceSvc
	} else if svcs.TraceSvc != nil {
		deps.TraceSvc = svcs.TraceSvc
	}

	return smoke.NewService(deps)
}
