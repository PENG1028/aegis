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

// AppService is the main apply service coordinating the full apply flow.
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

// ApplyResult holds the result of an apply operation.
type ApplyResult struct {
	Version  string
	Warnings []string
	Config   string
}

// PreviewResult holds a config preview with metadata.
type PreviewResult struct {
	Routes         []proxy.RouteConfig `json:"routes"`
	Warnings       []string            `json:"warnings"`
	RenderedConfig string              `json:"rendered_config"`
}

// DryRun generates the configuration without applying it.
func (s *AppService) DryRun(ctx context.Context) (*ApplyResult, error) {
	gwCfg, warnings, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(*gwCfg)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	return &ApplyResult{
		Version:  fmt.Sprintf("dry-run-%s", time.Now().Format("20060102_150405")),
		Warnings: warnings,
		Config:   string(rendered),
	}, nil
}

// Preview returns the full preview including structured route configs.
func (s *AppService) Preview(ctx context.Context) (*PreviewResult, error) {
	gwCfg, warnings, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(*gwCfg)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	return &PreviewResult{
		Routes:         gwCfg.Routes,
		Warnings:       warnings,
		RenderedConfig: string(rendered),
	}, nil
}

// Apply executes the full apply flow:
// 1. Plan (read routes + services + managed domains)
// 2. Render config
// 3. Write temp file
// 4. Validate
// 5. Backup
// 6. Replace
// 7. Reload
// 8. Record apply version + logs
func (s *AppService) Apply(ctx context.Context) (*ApplyResult, error) {
	version := fmt.Sprintf("v%s", time.Now().Format("20060102_150405"))
	var warnings []string

	// Step 1: Plan
	gwCfg, planWarnings, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("plan failed: %v", err))
		return nil, fmt.Errorf("plan: %w", err)
	}
	warnings = planWarnings

	// Step 2: Render
	rendered, err := s.adapter.Render(*gwCfg)
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("render failed: %v", err))
		return nil, fmt.Errorf("render: %w", err)
	}
	renderedStr := string(rendered)

	// Step 3: Write temp file
	tempPath, err := s.executor.WriteTemp(rendered)
	if err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("write temp failed: %v", err))
		return nil, fmt.Errorf("write temp config: %w", err)
	}

	// Step 4: Validate
	if err := s.executor.ValidateAdapter(s.adapter, tempPath); err != nil {
		os.Remove(tempPath)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("validate failed: %v", err))
		return nil, fmt.Errorf("validate: %w", err)
	}

	// Step 5: Backup
	backupPath, err := s.executor.Backup()
	if err != nil {
		os.Remove(tempPath)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("backup failed: %v", err))
		return nil, fmt.Errorf("backup: %w", err)
	}

	// Step 6: Replace config
	if err := s.executor.Replace(tempPath); err != nil {
		s.logFailed(ctx, "apply", "", fmt.Sprintf("replace config failed: %v", err))
		return nil, fmt.Errorf("replace config: %w", err)
	}

	// Step 7: Reload
	if err := s.executor.ReloadAdapter(s.adapter); err != nil {
		s.logSvc.Log(ctx, "apply", "", "", "failed",
			fmt.Sprintf("reload failed, attempting restore: %v", err), "cli")

		if backupPath != "" {
			if restoreErr := s.executor.RestoreBackup(backupPath); restoreErr != nil {
				s.logSvc.Log(ctx, "apply", "", "", "failed",
					fmt.Sprintf("restore backup also failed: %v", restoreErr), "cli")
				return nil, fmt.Errorf("reload failed: %w; restore backup also failed: %v", err, restoreErr)
			}
			if reloadErr := s.executor.ReloadAdapter(s.adapter); reloadErr != nil {
				s.logSvc.Log(ctx, "apply", "", "", "failed",
					fmt.Sprintf("reload of restored config also failed: %v", reloadErr), "cli")
				return nil, fmt.Errorf("reload failed, restored old config but reload also failed: %v", reloadErr)
			}
		}

		s.logFailed(ctx, "apply", "", fmt.Sprintf("reload failed, restored old config: %v", err))
		return nil, fmt.Errorf("reload failed, restored old config: %v", err)
	}

	// Step 8: Record apply version
	av := &ApplyVersion{
		ID:             id.New("apply"),
		Version:        version,
		ConfigPath:     s.cfg.Proxy.CaddyfilePath,
		BackupPath:     backupPath,
		RenderedConfig: renderedStr,
		Status:         "success",
		Message:        fmt.Sprintf("applied %d routes", len(gwCfg.Routes)),
		CreatedAt:      time.Now(),
	}
	if err := s.applyRepo.Create(av); err != nil {
		s.logSvc.Log(ctx, "apply.record", "", "", "failed",
			fmt.Sprintf("failed to record apply version: %v", err), "cli")
	}

	s.logSvc.Log(ctx, "apply", "", "", "success",
		fmt.Sprintf("applied version %s with %d routes", version, len(gwCfg.Routes)), "cli")

	return &ApplyResult{
		Version:  version,
		Warnings: warnings,
		Config:   renderedStr,
	}, nil
}

// Validate generates and validates the configuration without applying.
func (s *AppService) Validate(ctx context.Context) error {
	gwCfg, _, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(*gwCfg)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}

	tempPath, err := s.executor.WriteTemp(rendered)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	defer os.Remove(tempPath)

	if err := s.executor.ValidateAdapter(s.adapter, tempPath); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	return nil
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

// Rollback rolls back to the last successful apply.
func (s *AppService) Rollback(ctx context.Context) error {
	lastSuccess, err := s.applyRepo.FindLastSuccess()
	if err != nil {
		return fmt.Errorf("find last success: %w", err)
	}
	if lastSuccess == nil {
		return fmt.Errorf("no successful apply to rollback to")
	}

	if err := s.rollbackSvc.Rollback(s.adapter); err != nil {
		s.logSvc.Log(ctx, "rollback", "", "", "failed", err.Error(), "cli")
		return err
	}

	now := time.Now()
	rb := &ApplyVersion{
		ID:         id.New("apply"),
		Version:    fmt.Sprintf("rollback-%s", now.Format("20060102_150405")),
		ConfigPath: s.cfg.Proxy.CaddyfilePath,
		BackupPath: lastSuccess.BackupPath,
		Status:     "rolled_back",
		Message:    fmt.Sprintf("rolled back to %s", lastSuccess.Version),
		CreatedAt:  now,
	}
	_ = s.applyRepo.Create(rb)

	s.logSvc.Log(ctx, "rollback", "", "", "success",
		fmt.Sprintf("rolled back to %s", lastSuccess.Version), "cli")
	return nil
}

func (s *AppService) logFailed(ctx context.Context, action, targetID, message string) {
	s.logSvc.Log(ctx, action, "", targetID, "failed", message, "cli")
}
