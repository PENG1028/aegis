package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"aegis/internal/apply"
	"aegis/internal/config"
	"aegis/internal/health"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/project"
	"aegis/internal/route"
	"aegis/internal/service"

	"github.com/spf13/cobra"
)

func newDiagnosticsCommand(
	cfg *config.Config,
	projectSvc *project.AppService,
	serviceSvc *service.AppService,
	routeSvc *route.AppService,
	mdSvc *manageddomain.AppService,
	applySvc *apply.AppService,
	healthSvc *health.AppService,
	logSvc logs.Logger,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnostics",
		Short: "Diagnostics and troubleshooting tools",
	}

	cmd.AddCommand(newDiagnosticsExportCommand(cfg, projectSvc, serviceSvc, routeSvc, mdSvc, applySvc, healthSvc, logSvc))

	return cmd
}

func newDiagnosticsExportCommand(
	cfg *config.Config,
	projectSvc *project.AppService,
	serviceSvc *service.AppService,
	routeSvc *route.AppService,
	mdSvc *manageddomain.AppService,
	applySvc *apply.AppService,
	healthSvc *health.AppService,
	logSvc logs.Logger,
) *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export diagnostics to a JSON file",
		Long:  "Exports system status, settings (redacted), all resources, health checks, apply history, and logs to a JSON file for troubleshooting.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			diag := map[string]interface{}{
				"exported_at": time.Now().Format(time.RFC3339),
			}

			// System
			diag["system"] = map[string]interface{}{
				"name":    "aegis",
				"version": "0.x",
				"proxy": map[string]interface{}{
					"provider": cfg.Proxy.Provider,
				},
			}

			// Settings (redacted — no admin_token)
			diag["settings"] = map[string]interface{}{
				"proxy": map[string]interface{}{
					"provider":         cfg.Proxy.Provider,
					"caddyfile_path":   cfg.Proxy.CaddyfilePath,
					"caddy_binary":     cfg.Proxy.CaddyBinary,
					"reload_command":   cfg.Proxy.ReloadCommand,
					"validate_command": cfg.Proxy.ValidateCommand,
					"backup_dir":       cfg.Proxy.BackupDir,
				},
				"store": map[string]interface{}{
					"sqlite_path": cfg.Store.SQLitePath,
				},
				"server": map[string]interface{}{
					"addr":        cfg.Server.Addr,
					"admin_token": "***REDACTED***",
				},
				"managed_domain": cfg.ManagedDomain,
			}

			// Projects
			projects, _ := projectSvc.ListProjects(ctx)
			diag["projects"] = projects

			// Services
			services, _ := serviceSvc.ListServices(ctx)
			diag["services"] = services

			// Routes
			routes, _ := routeSvc.ListRoutes(ctx)
			diag["routes"] = routes

			// Managed Domains
			mdDomains, _ := mdSvc.ListManagedDomains(ctx)
			diag["managed_domains"] = mdDomains

			// Latest health checks
			healthChecks, _ := healthSvc.GetLatestForAll(ctx)
			diag["latest_health_checks"] = healthChecks

			// Apply history
			applyHistory, _ := applySvc.History(ctx)
			diag["apply_history"] = applyHistory

			// Operation logs (latest 200)
			logs, _ := logSvc.ListLogs(ctx, "", "")
			if len(logs) > 200 {
				logs = logs[:200]
			}
			diag["operation_logs_latest_200"] = logs

			// Current config
			currentConfig, _ := applySvc.GetCurrentConfig()
			diag["current_config"] = currentConfig

			// Preview config
			plan, err := applySvc.DryRun(ctx)
			if err == nil {
				diag["preview_config"] = plan.RenderedConfig
				diag["warnings"] = plan.Warnings
			}

			// Write to file
			filename := fmt.Sprintf("./aegis-diagnostics-%s.json", time.Now().Format("20060102-150405"))
			data, err := json.MarshalIndent(diag, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal diagnostics: %w", err)
			}

			if err := os.WriteFile(filename, data, 0644); err != nil {
				return fmt.Errorf("write diagnostics file: %w", err)
			}

			fmt.Printf("Diagnostics exported to: %s\n", filename)
			return nil
		},
	}
}
