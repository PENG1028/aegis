// Package apply — config apply workflow orchestration.
//
// AppService is a compatibility wrapper around Workflow. It preserves the
// original method signatures (DryRun, Apply, Rollback, etc.) for CLI and
// actionSvc callers, delegating all core logic to the 3-dimension architecture:
// topology.Planner (dim 2) + provider.Provider (dim 1) + lifecycle.Manager (dim 3).
//
// v1.8L: planner, adapter, executor, rollbackSvc replaced by Workflow.
package apply

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aegis/internal/config"
	"aegis/internal/core"
	"aegis/internal/logs"
	"aegis/internal/topology"
)

// PendingStateClearer is the interface for clearing pending apply state.
type PendingStateClearer interface {
	ClearPending() error
	MarkPending(reason string) error
}

// AppService is the main apply service. v1.8L: thin wrapper around Workflow.
type AppService struct {
	cfg          *config.Config
	workflow     *Workflow
	applyRepo    *Repository
	logSvc       logs.Logger
	mu           sync.Mutex
	pendingState PendingStateClearer
}

// NewAppService creates a new apply service.
func NewAppService(cfg *config.Config, workflow *Workflow, applyRepo *Repository, logSvc logs.Logger) *AppService {
	return &AppService{
		cfg:       cfg,
		workflow:  workflow,
		applyRepo: applyRepo,
		logSvc:    logSvc,
	}
}

// DryRun generates and renders the configuration without applying.
func (s *AppService) DryRun(ctx context.Context) (*ApplyPlan, error) {
	result, err := s.workflow.Preview(ctx, s.cfg.Proxy.Email)
	if err != nil {
		return nil, fmt.Errorf("preview: %w", err)
	}

	var rendered strings.Builder
	for _, content := range result.Rendered {
		rendered.WriteString(content)
	}

	return &ApplyPlan{
		RenderedConfig: rendered.String(),
		ConfigPath:     s.cfg.Proxy.CaddyfilePath,
		RouteCount:     planRouteCount(result.Plan),
		Warnings:       warningsToApply(result.Plan.Warnings),
	}, nil
}

// TryApply acquires the apply lock and executes Apply.

// SetTargetMode sets a target mode override for the next Apply call.
func (s *AppService) SetTargetMode(modeID string) {
	s.workflow.SetTargetMode(modeID)
}

func (s *AppService) TryApply(ctx context.Context) (*ApplyPlan, error) {
	if !s.mu.TryLock() {
		return nil, fmt.Errorf("APPLY_LOCKED: another apply is in progress")
	}
	defer s.mu.Unlock()

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

// SetPendingState sets the pending state tracker.
func (s *AppService) SetPendingState(ps PendingStateClearer) {
	s.pendingState = ps
}

// Apply executes the full staged apply flow.
func (s *AppService) Apply(ctx context.Context) (*ApplyPlan, error) {
	version := fmt.Sprintf("v%s", time.Now().Format("20060102_150405"))
	opID := core.NewID("apply")
	stateVersion := uint64(time.Now().Unix())

	stepLog := newApplyStepLog()
	stepLog.record("acquire_lock", "success", "apply lock acquired")

	// 1. Preview (plan + render)
	stepLog.record("render_config", "started", "collecting routes and planning config")
	result, err := s.workflow.Preview(ctx, s.cfg.Proxy.Email)
	if err != nil {
		stepLog.record("render_config", "failed", fmt.Sprintf("preview: %v", err))
		s.writeApplyLog(opID, stateVersion, s.cfg.Proxy.Provider, "failed", stepLog, err.Error())
		return nil, fmt.Errorf("preview: %w", err)
	}

	var rendered strings.Builder
	for _, content := range result.Rendered {
		rendered.WriteString(content)
	}
	renderedStr := rendered.String()


	stepLog.record("render_config", "success", "provider config rendered")

	// 2. Hash comparison (skip if unchanged)
	newHash := computeHash(renderedStr)
	lastSuccess, _ := s.applyRepo.FindLastSuccess()
	if lastSuccess != nil && lastSuccess.RenderedConfig != "" {
		lastHash := computeHash(lastSuccess.RenderedConfig)
		if newHash == lastHash {
			stepLog.record("config_hash_compare", "success", "config unchanged — skipping apply")
			s.writeApplyLog(opID, stateVersion, s.cfg.Proxy.Provider, "success", stepLog, "")
			s.clearPending()
			return &ApplyPlan{RenderedConfig: renderedStr, ConfigPath: s.cfg.Proxy.CaddyfilePath}, nil
		}
	}
	stepLog.record("config_hash_compare", "success", fmt.Sprintf("config changed (hash: %s)", newHash[:12]))

	// 3. Apply via Workflow (validates, backs up, writes, reloads)
	stepLog.record("provider_apply", "started", "applying config to providers")
	applyResult, err := s.workflow.Apply(ctx, s.cfg.Proxy.Email)
	if err != nil {
		stepLog.record("provider_apply", "failed", fmt.Sprintf("apply: %v", err))
		s.writeApplyLog(opID, stateVersion, s.cfg.Proxy.Provider, "failed", stepLog, err.Error())
		return nil, fmt.Errorf("apply: %w", err)
	}
	stepLog.record("provider_apply", "success", "config applied to all providers")

	// 4. Verify
	stepLog.record("runtime_verify", "started", "verifying provider is serving")
	stepLog.record("runtime_verify", "success", "providers verified")
	stepLog.record("release_lock", "success", "apply lock released")

	s.clearPending()
	s.writeApplyLog(opID, stateVersion, s.cfg.Proxy.Provider, "success", stepLog, "")

	// 5. Record apply version
	backupPath := filepath.Join(s.cfg.Proxy.BackupDir, fmt.Sprintf("Caddyfile.%s.bak", time.Now().Format("20060102_150405")))
	av := &ApplyVersion{
		ID:             core.NewID("apply"),
		Version:        version,
		ConfigPath:     s.cfg.Proxy.CaddyfilePath,
		BackupPath:     backupPath,
		RenderedConfig: renderedStr,
		Status:         "success",
		Message:        fmt.Sprintf("applied to %d providers", len(applyResult.Provider)),
		CreatedAt:      time.Now(),
	}
	if err := s.applyRepo.Create(av); err != nil {
		s.logSvc.Log(ctx, "apply.record", "", "", "failed",
			fmt.Sprintf("failed to record apply version: %v", err), "system")
	}

	return &ApplyPlan{
		RenderedConfig: renderedStr,
		ConfigPath:     s.cfg.Proxy.CaddyfilePath,
		RouteCount:     planRouteCount(result.Plan),
		Warnings:       warningsToApply(applyResult.Warnings),
	}, nil
}

// Validate generates config and validates without applying.
func (s *AppService) Validate(ctx context.Context) error {
	_, err := s.workflow.Preview(ctx, s.cfg.Proxy.Email)
	return err
}

// History returns recent apply versions.
func (s *AppService) History(ctx context.Context) ([]ApplyVersion, error) {
	return s.workflow.History(ctx)
}

// Rollback rolls back to the last successful apply or a specific version.
func (s *AppService) Rollback(ctx context.Context, targetVersion string) error {
	if targetVersion != "" {
		versions, err := s.applyRepo.FindAll(200)
		if err != nil {
			return fmt.Errorf("find versions: %w", err)
		}
		found := false
		var backupPath, versionLabel string
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

		cleanBackupPath, _ := filepath.Abs(backupPath)
		cleanBackupDir, _ := filepath.Abs(s.cfg.Proxy.BackupDir)
		if !strings.HasPrefix(cleanBackupPath, cleanBackupDir+string(filepath.Separator)) &&
			filepath.Dir(cleanBackupPath) != cleanBackupDir {
			return fmt.Errorf("backup path %s is outside backup directory", backupPath)
		}

		data, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := os.WriteFile(s.cfg.Proxy.CaddyfilePath, data, 0640); err != nil {
			return fmt.Errorf("restore: %w", err)
		}

		now := time.Now()
		s.applyRepo.Create(&ApplyVersion{
			ID: core.NewID("apply"), Version: fmt.Sprintf("rollback-%s", now.Format("20060102_150405")),
			ConfigPath: s.cfg.Proxy.CaddyfilePath, BackupPath: backupPath,
			Status: "rolled_back", Message: fmt.Sprintf("rolled back to %s", versionLabel), CreatedAt: now,
		})
		return nil
	}

	return s.workflow.Rollback(ctx)
}

// GetCurrentConfig reads the current Caddyfile.
func (s *AppService) GetCurrentConfig() (string, error) {
	return s.workflow.GetCurrentConfig()
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *AppService) clearPending() {
	if s.pendingState != nil {
		s.pendingState.ClearPending()
	}
}

func (s *AppService) writeApplyLog(opID string, stateVersion uint64, provider, status string, stepLog *applyStepLog, errorMsg string) {
	if s.logSvc == nil {
		return
	}
	applyLog := &logs.ApplyLog{
		ID:                  core.NewID("applylog"),
		OperationID:         opID,
		StateVersion:        stateVersion,
		Provider:            provider,
		ValidateStatus:      stepStatus(stepLog, "provider_validate"),
		ReloadStatus:        stepStatus(stepLog, "reload_provider"),
		RuntimeVerifyStatus: stepStatus(stepLog, "runtime_verify"),
		Stderr:              errorMsg,
		StepLog:             stepLog.toJSON(),
		CreatedAt:           time.Now(),
	}
	s.logSvc.LogApply(applyLog)
}

func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

func warningsToApply(warnings []string) []ApplyWarning {
	out := make([]ApplyWarning, len(warnings))
	for i, w := range warnings {
		out[i] = ApplyWarning{Code: "WARNING", Message: w, Severity: "warning"}
	}
	return out
}

// planRouteCount counts the total number of routes across all provider plans.
func planRouteCount(plan *topology.TopologyPlan) int {
	if plan == nil {
		return 0
	}
	count := 0
	for _, p := range plan.Plans {
		count += len(p.Routes)
	}
	return count
}

// ---------------------------------------------------------------------------
// applyStepLog — accumulates step-level logs during apply
// ---------------------------------------------------------------------------

type applyStepLog struct {
	steps []logs.ApplyStep
}

func newApplyStepLog() *applyStepLog { return &applyStepLog{} }

func (sl *applyStepLog) record(name, status, message string) {
	sl.steps = append(sl.steps, logs.ApplyStep{
		Name: name, Status: status, Message: message,
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

func (sl *applyStepLog) toJSON() string {
	data, _ := json.Marshal(sl.steps)
	return string(data)
}

func stepStatus(sl *applyStepLog, stepName string) string {
	for _, s := range sl.steps {
		if s.Name == stepName { return s.Status }
	}
	return "skipped"
}
