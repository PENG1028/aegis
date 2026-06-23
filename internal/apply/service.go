package apply

import (
	"context"
	"fmt"
	"os"
	"time"

	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/service"
)

// AppService is the main apply service.
type AppService struct {
	cfg         *config.Config
	adapter     proxy.ProxyAdapter
	planner     *Planner
	executor    *Executor
	rollbackSvc *RollbackService
	applyRepo   *Repository
	logSvc      *logs.AppService
}

// NewAppService creates a new apply application service.
func NewAppService(
	cfg *config.Config,
	adapter proxy.ProxyAdapter,
	routeRepo *route.Repository,
	mdRepo *manageddomain.Repository,
	serviceRepo *service.Repository,
	endpointResolver *endpoint.Resolver,
	applyRepo *Repository,
	logSvc *logs.AppService,
) *AppService {
	return &AppService{
		cfg:         cfg,
		adapter:     adapter,
		planner:     NewPlanner(routeRepo, mdRepo, serviceRepo, endpointResolver),
		executor:    NewExecutor(cfg),
		rollbackSvc: NewRollbackService(applyRepo, cfg),
		applyRepo:   applyRepo,
		logSvc:      logSvc,
	}
}

// Plan generates a full ApplyPlan without rendering.
func (s *AppService) Plan(ctx context.Context) (*ApplyPlan, error) {
	return s.planner.Plan(s.cfg.Proxy.Email)
}

// DryRun generates and renders the configuration without applying.
func (s *AppService) DryRun(ctx context.Context) (*ApplyPlan, error) {
	plan, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(proxy.GatewayConfig{
		Routes: plan.Routes,
		Email:  s.cfg.Proxy.Email,
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	plan.RenderedConfig = string(rendered)
	return plan, nil
}

// Apply executes the full staged apply flow.
func (s *AppService) Apply(ctx context.Context) (*ApplyPlan, error) {
	version := fmt.Sprintf("v%s", time.Now().Format("20060102_150405"))

	// Step 1-2: Plan
	plan, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("plan failed: %v", err))
		return nil, fmt.Errorf("plan: %w", err)
	}

	// Step 3: Render
	rendered, err := s.adapter.Render(proxy.GatewayConfig{
		Routes: plan.Routes,
		Email:  s.cfg.Proxy.Email,
	})
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("render failed: %v", err))
		return nil, fmt.Errorf("render: %w", err)
	}
	plan.RenderedConfig = string(rendered)
	plan.ConfigPath = s.cfg.Proxy.CaddyfilePath

	// Step 4: Write temp
	tempPath, err := s.executor.WriteTemp(rendered)
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("write temp failed: %v", err))
		return plan, fmt.Errorf("write temp config: %w", err)
	}
	plan.TempPath = tempPath

	// Step 5: Validate
	if err := s.executor.ValidateAdapter(s.adapter, tempPath); err != nil {
		os.Remove(tempPath)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("validate failed: %v", err))
		return plan, fmt.Errorf("validate: %w — config not replaced", err)
	}

	// Step 6: Backup
	backupPath, err := s.executor.Backup()
	if err != nil {
		os.Remove(tempPath)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("backup failed: %v", err))
		return plan, fmt.Errorf("backup: %w", err)
	}
	plan.BackupPath = backupPath

	// Step 7: Replace
	if err := s.executor.Replace(tempPath); err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("replace config failed: %v", err))
		return plan, fmt.Errorf("replace config: %w", err)
	}

	// Step 8: Reload
	if err := s.executor.ReloadAdapter(s.adapter); err != nil {
		s.logSvc.Log(ctx, "apply", "", "", "failed",
			fmt.Sprintf("reload failed, attempting restore: %v", err), "system")

		if backupPath != "" {
			// Step 9-10: Restore backup
			if restoreErr := s.executor.RestoreBackup(backupPath); restoreErr != nil {
				s.logSvc.Log(ctx, "apply", "", "", "critical",
					fmt.Sprintf("CRITICAL: restore backup also failed: %v", restoreErr), "system")
				return plan, fmt.Errorf("reload failed: %w; CRITICAL: restore backup also failed: %v", err, restoreErr)
			}
			// Step 11: Reload restored config
			if reloadErr := s.executor.ReloadAdapter(s.adapter); reloadErr != nil {
				s.logSvc.Log(ctx, "apply", "", "", "critical",
					fmt.Sprintf("CRITICAL: reload of restored config also failed: %v", reloadErr), "system")
				return plan, fmt.Errorf("reload failed, restored old config but reload also failed: %v", reloadErr)
			}
			s.logSvc.Log(ctx, "apply", "", "", "success",
				"reload failed but old config restored and reloaded successfully", "system")
		}

		s.logFailed(ctx, "apply", "", fmt.Sprintf("reload failed, restored old config: %v", err))
		return plan, fmt.Errorf("reload failed, restored old config: %v", err)
	}

	// Step 12-13: Record
	av := &ApplyVersion{
		ID:             id.New("apply"),
		Version:        version,
		ConfigPath:     s.cfg.Proxy.CaddyfilePath,
		BackupPath:     backupPath,
		RenderedConfig: string(rendered),
		Status:         "success",
		Message:        fmt.Sprintf("applied %d routes (%d managed domains)", plan.RouteCount, plan.ManagedDomainCount),
		CreatedAt:      time.Now(),
	}
	if err := s.applyRepo.Create(av); err != nil {
		s.logSvc.Log(ctx, "apply.record", "", "", "failed",
			fmt.Sprintf("failed to record apply version: %v", err), "system")
	}

	s.logSvc.Log(ctx, "apply", "", "", "success",
		fmt.Sprintf("applied version %s: %d routes, %d managed domains", version, plan.RouteCount, plan.ManagedDomainCount), "system")

	return plan, nil
}

// Validate generates and validates without applying.
func (s *AppService) Validate(ctx context.Context) error {
	plan, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(proxy.GatewayConfig{
		Routes: plan.Routes,
		Email:  s.cfg.Proxy.Email,
	})
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	tempPath, err := s.executor.WriteTemp(rendered)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	defer os.Remove(tempPath)

	return s.executor.ValidateAdapter(s.adapter, tempPath)
}

// History returns recent apply versions.
func (s *AppService) History(ctx context.Context) ([]ApplyVersion, error) {
	versions, err := s.applyRepo.FindAll(50)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	if versions == nil {
		versions = []ApplyVersion{}
	}
	return versions, nil
}

// Rollback rolls back to the last successful apply or a specific version.
func (s *AppService) Rollback(ctx context.Context, targetVersion string) error {
	var backupPath string
	var versionLabel string

	if targetVersion != "" {
		// Find specific version
		versions, err := s.applyRepo.FindAll(200)
		if err != nil {
			return fmt.Errorf("find versions: %w", err)
		}
		found := false
		for _, v := range versions {
			if v.Version == targetVersion && v.Status == "success" && v.BackupPath != "" {
				backupPath = v.BackupPath
				versionLabel = v.Version
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("version %s not found or has no backup", targetVersion)
		}
	} else {
		lastSuccess, err := s.applyRepo.FindLastSuccess()
		if err != nil {
			return fmt.Errorf("find last success: %w", err)
		}
		if lastSuccess == nil {
			return fmt.Errorf("no successful apply to rollback to")
		}
		if lastSuccess.BackupPath == "" {
			return fmt.Errorf("no backup available for last successful apply")
		}
		backupPath = lastSuccess.BackupPath
		versionLabel = lastSuccess.Version
	}

	// Restore backup
	if err := s.executor.RestoreBackup(backupPath); err != nil {
		s.logSvc.Log(ctx, "rollback", "", "", "failed",
			fmt.Sprintf("restore backup failed: %v", err), "system")
		return fmt.Errorf("restore backup: %w", err)
	}

	// Validate restored config
	if err := s.executor.ValidateAdapter(s.adapter, s.cfg.Proxy.CaddyfilePath); err != nil {
		s.logSvc.Log(ctx, "rollback", "", "", "failed",
			fmt.Sprintf("validate restored config failed: %v", err), "system")
		return fmt.Errorf("validate restored config: %w", err)
	}

	// Reload
	if err := s.executor.ReloadAdapter(s.adapter); err != nil {
		s.logSvc.Log(ctx, "rollback", "", "", "failed",
			fmt.Sprintf("reload after rollback failed: %v", err), "system")
		return fmt.Errorf("reload after rollback: %w", err)
	}

	// Record rollback
	now := time.Now()
	rb := &ApplyVersion{
		ID:         id.New("apply"),
		Version:    fmt.Sprintf("rollback-%s", now.Format("20060102_150405")),
		ConfigPath: s.cfg.Proxy.CaddyfilePath,
		BackupPath: backupPath,
		Status:     "rolled_back",
		Message:    fmt.Sprintf("rolled back to %s", versionLabel),
		CreatedAt:  now,
	}
	if err := s.applyRepo.Create(rb); err != nil {
		s.logSvc.Log(ctx, "rollback.record", "", "", "failed",
			fmt.Sprintf("failed to record rollback: %v", err), "system")
	}

	s.logSvc.Log(ctx, "rollback", "", "", "success",
		fmt.Sprintf("rolled back to %s", versionLabel), "system")
	return nil
}

// GetCurrentConfig reads the current configuration file.
func (s *AppService) GetCurrentConfig() (string, error) {
	data, err := os.ReadFile(s.cfg.Proxy.CaddyfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read current config: %w", err)
	}
	return string(data), nil
}

func (s *AppService) logFailed(ctx context.Context, action, targetID, message string) {
	s.logSvc.Log(ctx, action, "", targetID, "failed", message, "system")
}
