// DEPRECATED (v1.8L cleanup): This file implements a Caddy-only Apply pipeline
// hardcoded to s.adapter (CaddyAdapter). It will be replaced by a provider-iterating
// pipeline that walks provRegistry.ListAll() and calls each Provider's Render,
// Validate, WriteConfig, and Reload methods.
package apply

import (
	"aegis/internal/gateway_link"
	"aegis/internal/secrets"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aegis/internal/config"
	"aegis/internal/endpoint"
	"aegis/internal/exposure"
	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/manageddomain"
	"aegis/internal/provider"
	"aegis/internal/proxy"
	"aegis/internal/route"
	"aegis/internal/safety"
	"aegis/internal/service"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// PendingStateClearer is the interface for clearing pending apply state.
type PendingStateClearer interface {
	ClearPending() error
	MarkPending(reason string) error
}

// AppService is the main apply service.
type AppService struct {
	cfg          *config.Config
	adapter      proxy.ProxyAdapter
	planner      *Planner
	executor     *Executor
	rollbackSvc  *RollbackService
	applyRepo    *Repository
	logSvc       logs.Logger
	masterKey    *secrets.MasterKey // v1.8B-5
	mu           sync.Mutex
	pendingState PendingStateClearer // v1.7S
}

// NewAppService creates a new apply application service.
func NewAppService(
	cfg *config.Config,
	adapter proxy.ProxyAdapter,
	routeRepo *route.Repository,
	mdRepo *manageddomain.Repository,
	exposureRepo *exposure.Repository,
	serviceRepo *service.Repository,
	endpointResolver *endpoint.Resolver,
	applyRepo *Repository,
	logSvc logs.Logger,
	gwLinkRepo *gatewaylink.Repository,
	safetySvc *safety.Service, // v1.8A
	masterKey *secrets.MasterKey, // v1.8B-5
) *AppService {
	return &AppService{
		cfg:         cfg,
		adapter:     adapter,
		planner:     NewPlanner(routeRepo, mdRepo, exposureRepo, serviceRepo, endpointResolver, gwLinkRepo, safetySvc, masterKey),
		executor:    NewExecutor(cfg),
		rollbackSvc: NewRollbackService(applyRepo, cfg),
		applyRepo:   applyRepo,
		logSvc:      logSvc,
		masterKey:   masterKey,
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
		Routes:         plan.Routes,
		Email:          s.cfg.Proxy.Email,
		PortPolicyMode: provider.CurrentPortPolicyMode(),
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	plan.RenderedConfig = string(rendered)
	return plan, nil
}

// TryApply attempts to acquire the apply lock and executes Apply.
// Returns an error with code APPLY_LOCKED if another apply is in progress.
// Apply is bounded by a 60s timeout to prevent stuck locks at scale.
func (s *AppService) TryApply(ctx context.Context) (*ApplyPlan, error) {
	if !s.mu.TryLock() {
		return nil, fmt.Errorf("APPLY_LOCKED: another apply is in progress")
	}
	defer s.mu.Unlock()

	// Timeout guard: prevent stuck apply from holding lock indefinitely.
	// Critical for 5-10 node setups where a hanging reload blocks all config changes.
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	type result struct {
		plan *ApplyPlan
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		p, e := s.Apply(ctx)
		ch <- result{p, e}
	}()

	select {
	case r := <-ch:
		return r.plan, r.err
	case <-ctx.Done():
		return nil, fmt.Errorf("APPLY_TIMEOUT: apply exceeded 60s deadline: %w", ctx.Err())
	}
}

// SetPendingState sets the pending state tracker (v1.7S).
func (s *AppService) SetPendingState(ps PendingStateClearer) {
	s.pendingState = ps
}

// Apply executes the full staged apply flow.
func (s *AppService) Apply(ctx context.Context) (*ApplyPlan, error) {
	version := fmt.Sprintf("v%s", time.Now().Format("20060102_150405"))
	opID := id.New("apply")

	// v1.7W: Step-level apply log
	stepLog := newApplyStepLog()
	stepLog.record("acquire_lock", "success", "apply lock acquired")

	// State version snapshot for log
	var stateVersion uint64
	if s.applyRepo != nil {
		// Use the version timestamp as a proxy
		stateVersion = uint64(time.Now().Unix())
	}

	// Step 1-2: Plan
	stepLog.record("render_config", "started", "collecting routes and planning config")
	plan, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		stepLog.record("render_config", "failed", fmt.Sprintf("plan failed: %v", err))
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("plan: %v", err))
		s.logFailed(ctx, "apply", "", fmt.Sprintf("plan failed: %v", err))
		return nil, fmt.Errorf("plan: %w", err)
	}
	stepLog.record("render_config", "success", fmt.Sprintf("planned %d routes, %d domains", plan.RouteCount, plan.ManagedDomainCount))

	// Step 3: Render
	stepLog.record("render_config", "started", "rendering provider config")
	rendered, err := s.adapter.Render(proxy.GatewayConfig{
		Routes:         plan.Routes,
		Email:          s.cfg.Proxy.Email,
		PortPolicyMode: provider.CurrentPortPolicyMode(),
	})
	if err != nil {
		stepLog.record("render_config", "failed", fmt.Sprintf("render failed: %v", err))
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("render: %v", err))
		s.logFailed(ctx, "apply", "", fmt.Sprintf("render failed: %v", err))
		return nil, fmt.Errorf("render: %w", err)
	}
	// Prepend panel Caddyfile block so the control panel domain survives every apply.
	// PanelCaddyfile() generates the TLS-enabled panel site block (or :80 fallback).
	if s.cfg.ManagedDomain.GatewayDomain != "" {
		panelBlock := s.cfg.PanelCaddyfile()
		rendered = append([]byte(panelBlock), rendered...)
	}

	plan.RenderedConfig = string(rendered)
	plan.ConfigPath = s.cfg.Proxy.CaddyfilePath
	stepLog.record("render_config", "success", "provider config rendered")

	// Step 4: Write temp
	stepLog.record("render_config", "started", "writing temp config file")
	tempPath, err := s.executor.WriteTemp(rendered)
	if err != nil {
		stepLog.record("render_config", "failed", fmt.Sprintf("write temp: %v", err))
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("write temp: %v", err))
		s.logFailed(ctx, "apply", "", fmt.Sprintf("write temp failed: %v", err))
		return plan, fmt.Errorf("write temp config: %w", err)
	}
	plan.TempPath = tempPath
	stepLog.record("render_config", "success", "temp config written")

	// Step 5: Validate
	stepLog.record("provider_validate", "started", "validating provider config")
	if err := s.executor.ValidateAdapter(s.adapter, tempPath); err != nil {
		os.Remove(tempPath)
		errMsg := fmt.Sprintf("validate: %v", err)
		stepLog.record("provider_validate", "failed", errMsg)
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, errMsg)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("validate failed: %v", err))
		return plan, fmt.Errorf("validate: %w — config not replaced", err)
	}
	stepLog.record("provider_validate", "success", "config validation passed")

	// Step 6: Backup
	stepLog.record("render_config", "started", "backing up current config")
	backupPath, err := s.executor.Backup()
	if err != nil {
		os.Remove(tempPath)
		stepLog.record("render_config", "failed", fmt.Sprintf("backup: %v", err))
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("backup: %v", err))
		s.logFailed(ctx, "apply", "", fmt.Sprintf("backup failed: %v", err))
		return plan, fmt.Errorf("backup: %w", err)
	}
	plan.BackupPath = backupPath
	stepLog.record("render_config", "success", "current config backed up")

	// Step 7: Hash comparison (skip if identical to last applied)
	stepLog.record("config_hash_compare", "started", "comparing config hash")
	newHash := computeHash(string(rendered))
	lastSuccess, _ := s.applyRepo.FindLastSuccess()
	if lastSuccess != nil && lastSuccess.RenderedConfig != "" {
		lastHash := computeHash(lastSuccess.RenderedConfig)
		if newHash == lastHash {
			stepLog.record("config_hash_compare", "success", "config unchanged — skipping reload")
			s.writeApplyLog(opID, stateVersion, "caddy_http", "success", stepLog, "")
			s.logSvc.Log(ctx, "apply", "", "", "success",
				"config unchanged since last apply — skipping reload", "system")
			plan.RenderedConfig = string(rendered)
			return plan, nil
		}
	}
	stepLog.record("config_hash_compare", "success", fmt.Sprintf("config changed (hash: %s)", newHash[:12]))

	// Step 8: Replace
	stepLog.record("atomic_replace", "started", "replacing config file")
	if err := s.executor.Replace(tempPath); err != nil {
		stepLog.record("atomic_replace", "failed", fmt.Sprintf("replace: %v", err))
		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("replace: %v", err))
		s.logFailed(ctx, "apply", "", fmt.Sprintf("replace config failed: %v", err))
		return plan, fmt.Errorf("replace config: %w", err)
	}
	stepLog.record("atomic_replace", "success", "config file replaced")

	// Step 9: Reload
	stepLog.record("reload_provider", "started", "reloading provider")
	if err := s.executor.ReloadAdapter(s.adapter); err != nil {
		reloadErrMsg := fmt.Sprintf("reload failed: %v", err)
		stepLog.record("reload_provider", "failed", reloadErrMsg)
		s.logSvc.Log(ctx, "apply", "", "", "failed",
			fmt.Sprintf("reload failed, attempting restore: %v", err), "system")

		if backupPath != "" {
			if restoreErr := s.executor.RestoreBackup(backupPath); restoreErr != nil {
				stepLog.record("reload_provider", "failed", fmt.Sprintf("restore also failed: %v", restoreErr))
				s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("reload+restore: %v + %v", err, restoreErr))
				s.logSvc.Log(ctx, "apply", "", "", "critical",
					fmt.Sprintf("CRITICAL: restore backup also failed: %v", restoreErr), "system")
				return plan, fmt.Errorf("reload failed: %w; CRITICAL: restore backup also failed: %v", err, restoreErr)
			}
			if reloadErr := s.executor.ReloadAdapter(s.adapter); reloadErr != nil {
				stepLog.record("reload_provider", "failed", fmt.Sprintf("restored config reload also failed: %v", reloadErr))
				s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, fmt.Sprintf("restored reload: %v", reloadErr))
				s.logSvc.Log(ctx, "apply", "", "", "critical",
					fmt.Sprintf("CRITICAL: reload of restored config also failed: %v", reloadErr), "system")
				return plan, fmt.Errorf("reload failed, restored old config but reload also failed: %v", reloadErr)
			}
			stepLog.record("reload_provider", "success", "old config restored and reloaded")
			s.logSvc.Log(ctx, "apply", "", "", "success",
				"reload failed but old config restored and reloaded successfully", "system")
		}

		s.writeApplyLog(opID, stateVersion, "caddy_http", "failed", stepLog, reloadErrMsg)
		s.logFailed(ctx, "apply", "", fmt.Sprintf("reload failed, restored old config: %v", err))
		return plan, fmt.Errorf("reload failed, restored old config: %v", err)
	}
	stepLog.record("reload_provider", "success", "provider reloaded")

	// Step 10: Runtime verify — actually check proxy health
	stepLog.record("runtime_verify", "started", "verifying provider is serving")
	if healthErr := s.executor.VerifyProxyHealth(); healthErr != nil {
		stepLog.record("runtime_verify", "warning", fmt.Sprintf("proxy health check: %v — proxy may be on non-80 port", healthErr))
	} else {
		stepLog.record("runtime_verify", "success", "proxy responding on port 80")
	}
	stepLog.record("release_lock", "success", "apply lock released")

	plan.RenderedConfig = string(rendered)
	s.logSvc.Log(ctx, "apply", "", "", "success",
		fmt.Sprintf("config applied (hash: %s)", newHash[:12]), "system")

	// v1.7S: Clear pending apply on success
	if s.pendingState != nil {
		s.pendingState.ClearPending()
	}

	// v1.7W: Write step-level apply log on success
	s.writeApplyLog(opID, stateVersion, "caddy_http", "success", stepLog, "")

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

// v1.7W: writeApplyLog writes a step-level apply log entry.
func (s *AppService) writeApplyLog(opID string, stateVersion uint64, provider, status string, stepLog *applyStepLog, errorMsg string) {
	if s.logSvc == nil {
		return
	}
	stderr := errorMsg
	applyLog := &logs.ApplyLog{
		ID:                  id.New("applylog"),
		OperationID:         opID,
		StateVersion:        stateVersion,
		ConfigHashBefore:    "",
		ConfigHashAfter:     "",
		Provider:            provider,
		ValidateStatus:      stepStatus(stepLog, "provider_validate"),
		ReloadStatus:        stepStatus(stepLog, "reload_provider"),
		RuntimeVerifyStatus: stepStatus(stepLog, "runtime_verify"),
		Stderr:              stderr,
		StepLog:             stepLog.toJSON(),
		CreatedAt:           time.Now(),
	}
	s.logSvc.LogApply(applyLog)
}

// stepStatus extracts the status of a named step from the step log.
func stepStatus(sl *applyStepLog, stepName string) string {
	for _, s := range sl.steps {
		if s.Name == stepName {
			return s.Status
		}
	}
	return "skipped"
}

// Validate generates and validates without applying.
func (s *AppService) Validate(ctx context.Context) error {
	plan, err := s.planner.Plan(s.cfg.Proxy.Email)
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	rendered, err := s.adapter.Render(proxy.GatewayConfig{
		Routes:         plan.Routes,
		Email:          s.cfg.Proxy.Email,
		PortPolicyMode: provider.CurrentPortPolicyMode(),
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

	// Validate backupPath is within the configured backup directory to prevent
	// path traversal via a manipulated database record.
	cleanBackupPath, err := filepath.Abs(backupPath)
	if err != nil {
		return fmt.Errorf("resolve backup path: %w", err)
	}
	cleanBackupDir, err := filepath.Abs(s.cfg.Proxy.BackupDir)
	if err != nil {
		return fmt.Errorf("resolve backup dir: %w", err)
	}
	if !strings.HasPrefix(cleanBackupPath, cleanBackupDir+string(filepath.Separator)) &&
		cleanBackupPath != cleanBackupDir+string(filepath.Separator)+filepath.Base(backupPath) {
		// Re-check: ensure the backup path is a direct child of the backup directory
		if filepath.Dir(cleanBackupPath) != cleanBackupDir {
			return fmt.Errorf("backup path %s is outside backup directory", backupPath)
		}
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

func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func (s *AppService) logFailed(ctx context.Context, action, targetID, message string) {
	s.logSvc.Log(ctx, action, "", targetID, "failed", message, "system")
}

// applyStepLog accumulates step-level logs during an apply operation.
type applyStepLog struct {
	steps []logs.ApplyStep
}

func newApplyStepLog() *applyStepLog {
	return &applyStepLog{}
}

func (sl *applyStepLog) record(name, status, message string) {
	sl.steps = append(sl.steps, logs.ApplyStep{
		Name:      name,
		Status:    status,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (sl *applyStepLog) toJSON() string {
	data, _ := json.Marshal(sl.steps)
	return string(data)
}
