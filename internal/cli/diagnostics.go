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

			// Section load failures are collected here and surfaced in the
			// export (and stderr) instead of being silently dropped.
			var diagErrors []string
			collect := func(err error, value interface{}) interface{} {
				if err != nil {
					diagErrors = append(diagErrors, err.Error())
					return map[string]interface{}{"error": err.Error()}
				}
				return value
			}

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
					"caddy_data_dir":   cfg.Proxy.CaddyDataDir,
					"acme_server":      cfg.Proxy.ACMEServer,
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
			projects, err := projectSvc.ListProjects(ctx)
			diag["projects"] = collect(err, projects)

			// Services
			services, err := serviceSvc.ListServices(ctx)
			diag["services"] = collect(err, services)

			// Routes
			routes, err := routeSvc.ListRoutes(ctx)
			diag["routes"] = collect(err, routes)

			// Managed Domains
			mdDomains, err := mdSvc.ListManagedDomains(ctx)
			diag["managed_domains"] = collect(err, mdDomains)

			// Latest health checks
			healthChecks, err := healthSvc.GetLatestForAll(ctx)
			diag["latest_health_checks"] = collect(err, healthChecks)

			// Apply history
			applyHistory, err := applySvc.History(ctx)
			diag["apply_history"] = collect(err, applyHistory)

			// Operation logs (latest 200)
			logEntries, err := logSvc.ListLogs(ctx, "", "")
			diag["operation_logs_latest_200"] = collect(err, logEntries)
			if l, ok := diag["operation_logs_latest_200"].([]logs.OperationLog); ok && len(l) > 200 {
				diag["operation_logs_latest_200"] = l[:200]
			}

			// Current config
			currentConfig, err := applySvc.GetCurrentConfig()
			diag["current_config"] = collect(err, currentConfig)

			// Preview config
			plan, err := applySvc.DryRun(ctx)
			if err != nil {
				diag["preview_config"] = map[string]interface{}{"error": err.Error()}
			} else {
				diag["preview_config"] = plan.RenderedConfig
				diag["warnings"] = plan.Warnings
			}

			// Surface every section that failed so the exported JSON is
			// honest instead of silently missing chunks.
			if len(diagErrors) > 0 {
				diag["section_errors"] = diagErrors
				fmt.Fprintf(os.Stderr, "warning: diagnostics: %d section(s) failed to load (see section_errors)\n", len(diagErrors))
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
