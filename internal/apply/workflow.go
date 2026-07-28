// Package apply — workflow orchestration layer.
//
// Workflow orchestrates the full apply lifecycle: lock → plan → render →
// apply → verify → log → unlock. It sits ABOVE the 3 dimensions, coordinating
// topology.Planner (dim 2), provider.Provider (dim 1), and apply.Repository (audit).
package apply

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"aegis/internal/config"
	"aegis/internal/core"
	"aegis/internal/hostdep/provider"
	"aegis/internal/logs"
	"aegis/internal/topology"
)

// ============================================================================
// Workflow — orchestrates a complete apply operation
// ============================================================================

// Workflow coordinates the full apply lifecycle. It replaces AppService by
// delegating to topology.Planner for route resolution and provider.Provider
// for config generation + application. Locking, rollback, and audit logging
// are handled directly.
type Workflow struct {
	planner     *topology.Planner
	registry    *provider.Registry
	applyRepo   *Repository
	cfg         *config.Config
	logSvc      logs.Logger
	smokeTest   SmokeTest // optional: inject real HTTP probe for E2E mode-switch validation
	planApplied PlanAppliedHook
	mu          sync.Mutex
}

// SmokeTest validates that traffic still flows after a mode switch. nil means
// "skip" — the single-test path does not need a real network probe. Inject a
// real implementation (e.g. curl healthz + status) for production deployment.
// A failing smoke test produces a warning, not a rollback — the operator
// decides whether to roll back.
type SmokeTest func(ctx context.Context, mode provider.RuntimeMode) error

// PlanAppliedHook is called after provider configs have been applied
// successfully. It lets runtime components consume planner outputs without
// coupling them to provider rendering.
type PlanAppliedHook func(plan *topology.TopologyPlan)

// SetSmokeTest injects a smoke-test function for production deployments.
func (w *Workflow) SetSmokeTest(fn SmokeTest) {
	w.smokeTest = fn
}

// SetPlanAppliedHook injects a callback for successful apply operations.
func (w *Workflow) SetPlanAppliedHook(fn PlanAppliedHook) {
	w.planApplied = fn
}

// modeSnapshot captures enough state to roll back a failed mode switch.
type modeSnapshot struct {
	FromMode  string
	Providers []providerSnapshot
}

type providerSnapshot struct {
	ID           string
	ConfigPath   string
	ConfigBackup []byte // retained for compatibility with focused snapshot tests
	ConfigFiles  []provider.ConfigFile
	MissingPaths []string
	WasRunning   bool
}

type RollbackStatus string

const (
	RollbackNotRequired RollbackStatus = "not_required"
	RollbackComplete    RollbackStatus = "complete"
	RollbackIncomplete  RollbackStatus = "incomplete"
)

// ModeSwitchError carries recovery state across the workflow/HTTP boundary.
// WHY: operators must know whether recovery already happened before taking
// another action; parsing human-readable error strings is not reliable.
type ModeSwitchError struct {
	Reason           string
	RollbackStatus   RollbackStatus
	RollbackFailures []string
}

func (e *ModeSwitchError) Error() string {
	if e.RollbackStatus == RollbackIncomplete {
		return fmt.Sprintf("switch failed: %s; rollback incomplete: %s", e.Reason, strings.Join(e.RollbackFailures, "; "))
	}
	return fmt.Sprintf("switch_mode rolled back: %s", e.Reason)
}

// NewWorkflow creates an apply workflow orchestrator.
func NewWorkflow(
	planner *topology.Planner,
	registry *provider.Registry,
	applyRepo *Repository,
	cfg *config.Config,
	logSvc logs.Logger,
) *Workflow {
	return &Workflow{
		planner:   planner,
		registry:  registry,
		applyRepo: applyRepo,
		cfg:       cfg,
		logSvc:    logSvc,
	}
}

// ============================================================================
// Read operations
// ============================================================================

// Preview renders configuration for all providers without applying.
func (w *Workflow) Preview(ctx context.Context, email string) (*PreviewResult, error) {
	states := w.registry.List()
	plan, err := w.planner.PlanWithProviders(email, states)
	if err != nil {
		return nil, err
	}

	result := &PreviewResult{
		Plan:     plan,
		Rendered: make(map[string]string),
	}

	for provID, pPlan := range plan.Plans {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		configs, err := p.Render(pPlan)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", provID, err)
		}
		for _, cf := range configs {
			result.Rendered[cf.Path] = string(cf.Content)
		}
	}

	return result, nil
}

// ============================================================================
// Write operations
// ============================================================================

// TryApplyCtx acquires the lock and executes Apply using the stored config email.
// Matches the old AppService.TryApply(ctx) signature for drop-in replacement.
func (w *Workflow) TryApplyCtx(ctx context.Context) (*ApplyResult, error) {
	return w.TryApply(ctx, w.cfg.Proxy.Email)
}

// GetCurrentConfig returns the current Caddyfile content.
func (w *Workflow) GetCurrentConfig() (string, error) {
	cfgProvider := w.registry.FindByCapability(provider.CapHotReload)
	if cfgProvider == nil {
		return "", fmt.Errorf("no hot-reloadable provider found")
	}
	if reader, ok := cfgProvider.(provider.ConfigReader); ok {
		return reader.GetCurrentConfig()
	}
	return "", fmt.Errorf("provider does not support config reading")
}

// TryApply acquires the apply lock and executes Apply.
func (w *Workflow) TryApply(ctx context.Context, email string) (*ApplyResult, error) {
	if !w.mu.TryLock() {
		return nil, fmt.Errorf("APPLY_LOCKED: another apply is in progress")
	}
	defer w.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	return w.apply(ctx, email)
}

// SwitchMode atomically switches the gateway runtime mode between Legacy and
// EdgeMux. It snapshots current provider state before making changes so a
// mid-flight failure can be rolled back — the operator is not left with a
// half-switched gateway.
//
// SmokeTest (optional): when set, runs after the switch. A failing smoke test
// produces a warning, not a rollback — the operator decides.
func (w *Workflow) SwitchMode(ctx context.Context, targetModeID string) error {
	if !w.mu.TryLock() {
		return fmt.Errorf("APPLY_LOCKED")
	}
	defer w.mu.Unlock()

	states := w.registry.List()
	currentMode := provider.DetectRuntimeMode(states)
	if currentMode.ID == targetModeID {
		return fmt.Errorf("already in target mode: %s", targetModeID)
	}

	var targetMode provider.RuntimeMode
	for _, m := range provider.AllRuntimeModes() {
		if m.ID == targetModeID {
			targetMode = m
			break
		}
	}
	if targetMode.ID == "" {
		return fmt.Errorf("unknown target mode: %s", targetModeID)
	}

	// Plan and render before any process or file mutation.
	plan, err := w.planner.PlanForMode("", states, targetMode)
	if err != nil {
		return fmt.Errorf("target mode plan failed: %w", err)
	}
	targetProviderIDs := planProviderIDs(plan)
	rendered := make(map[string][]provider.ConfigFile, len(plan.Plans))
	for provID, pPlan := range plan.Plans {
		p := w.registry.Get(provID)
		if p == nil {
			return fmt.Errorf("target provider %s is unavailable", provID)
		}
		if _, ok := p.(provider.ConfigStager); !ok {
			return fmt.Errorf("target provider %s cannot stage configuration safely", provID)
		}
		if _, ok := p.(provider.ServiceController); !ok {
			return fmt.Errorf("target provider %s cannot control its service", provID)
		}
		configs, err := p.Render(pPlan)
		if err != nil {
			return fmt.Errorf("%s render: %w", provID, err)
		}
		rendered[provID] = configs
	}

	// Snapshot every provider touched by either side, including target-only
	// config paths, so rollback can restore files before restarting services.
	allIDs := append([]string(nil), currentMode.ProviderIDs()...)
	for _, id := range targetProviderIDs {
		if !slices.Contains(allIDs, id) {
			allIDs = append(allIDs, id)
		}
	}
	snap := modeSnapshot{FromMode: currentMode.ID}
	for _, id := range allIDs {
		p := w.registry.Get(id)
		if p == nil {
			continue
		}
		state := p.State()
		ps := providerSnapshot{ID: id, ConfigPath: state.ConfigPath, WasRunning: state.Running}
		paths := []string{state.ConfigPath}
		for _, cf := range rendered[id] {
			if cf.Path != "" && !slices.Contains(paths, cf.Path) {
				paths = append(paths, cf.Path)
			}
		}
		for _, path := range paths {
			if path == "" {
				continue
			}
			content, err := os.ReadFile(path)
			if os.IsNotExist(err) {
				ps.MissingPaths = append(ps.MissingPaths, path)
				continue
			}
			if err != nil {
				return fmt.Errorf("snapshot %s config %s: %w", id, path, err)
			}
			ps.ConfigFiles = append(ps.ConfigFiles, provider.ConfigFile{Path: path, Content: content})
			if path == state.ConfigPath {
				ps.ConfigBackup = content
			}
		}
		snap.Providers = append(snap.Providers, ps)
	}

	rollback := func(reason string) error {
		var rollbackFailures []string
		for _, id := range targetProviderIDs {
			if sc, ok := w.registry.Get(id).(provider.ServiceController); ok {
				if err := sc.Stop(); err != nil {
					rollbackFailures = append(rollbackFailures, id+" stop: "+err.Error())
				}
			}
		}
		for _, ps := range snap.Providers {
			p := w.registry.Get(ps.ID)
			if stager, ok := p.(provider.ConfigStager); ok && len(ps.ConfigFiles) > 0 {
				if err := stager.StageConfig(ps.ConfigFiles); err != nil {
					rollbackFailures = append(rollbackFailures, ps.ID+" config restore: "+err.Error())
				}
			}
			for _, path := range ps.MissingPaths {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					rollbackFailures = append(rollbackFailures, ps.ID+" remove staged config: "+err.Error())
				}
			}
			if sc, ok := p.(provider.ServiceController); ok {
				if ps.WasRunning {
					if err := sc.Start(); err != nil {
						rollbackFailures = append(rollbackFailures, ps.ID+" restart: "+err.Error())
					}
				} else {
					if err := sc.Stop(); err != nil {
						rollbackFailures = append(rollbackFailures, ps.ID+" stop: "+err.Error())
					}
				}
			}
		}
		if len(rollbackFailures) > 0 {
			switchErr := &ModeSwitchError{
				Reason:           reason,
				RollbackStatus:   RollbackIncomplete,
				RollbackFailures: rollbackFailures,
			}
			detail := switchErr.Error()
			w.logSwitchAudit("failed", detail)
			w.logApply(ctx, "switch_mode", "rollback_failed", detail)
			return switchErr
		}
		w.logSwitchAudit("failed", fmt.Sprintf("rolled back: %s", reason))
		w.logApply(ctx, "switch_mode", "rollback", reason)
		return &ModeSwitchError{Reason: reason, RollbackStatus: RollbackComplete}
	}

	// Phase 1: validate and write every target config while current services
	// still serve traffic. StageConfig never reloads or changes listeners.
	for _, id := range targetProviderIDs {
		stager := w.registry.Get(id).(provider.ConfigStager)
		if err := stager.StageConfig(rendered[id]); err != nil {
			return rollback(fmt.Sprintf("%s stage: %v", id, err))
		}
	}

	// Phase 2: release old listeners, then start target services from staged files.
	for _, id := range allIDs {
		if sc, ok := w.registry.Get(id).(provider.ServiceController); ok {
			if err := sc.Stop(); err != nil {
				return rollback(fmt.Sprintf("%s stop: %v", id, err))
			}
		}
	}
	for _, id := range targetMode.ProviderIDs() {
		sc := w.registry.Get(id).(provider.ServiceController)
		if err := sc.Start(); err != nil {
			return rollback(fmt.Sprintf("%s start: %v", id, err))
		}
		w.logApply(ctx, id, "switch_mode", "started for target "+targetModeID)
	}

	// ── 5. Post-switch diagnostic ──
	for _, provID := range targetProviderIDs {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		diag := p.Diagnose()
		if diag.LastErrorCode != "" {
			return rollback(fmt.Sprintf("%s: post-switch diag: %s — %s",
				provID, diag.LastErrorCode, diag.LastErrorMessage))
		}
	}
	if detected := provider.DetectRuntimeMode(w.registry.List()); detected.ID != targetMode.ID {
		return rollback(fmt.Sprintf("runtime mode verification: got %s, want %s", detected.ID, targetMode.ID))
	}

	// ── 6. Smoke test (optional — skip if not injected) ──
	if w.planApplied != nil {
		w.planApplied(plan)
	}

	if w.smokeTest != nil {
		if err := w.smokeTest(ctx, targetMode); err != nil {
			w.logApply(ctx, "switch_mode", "smoke_warning",
				fmt.Sprintf("smoke test failed (not rolled back): %v", err))
		}
	}

	w.logSwitchAudit("success", fmt.Sprintf("switched from %s to %s", currentMode.ID, targetModeID))
	w.logApply(ctx, "switch_mode", "success",
		fmt.Sprintf("switched from %s to %s", currentMode.ID, targetModeID))
	return nil
}

// Apply executes the full pipeline:
// plan → render → validate → backup → write → reload → verify → log.
func (w *Workflow) Apply(ctx context.Context, email string) (*ApplyResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.apply(ctx, email)
}

func (w *Workflow) apply(ctx context.Context, email string) (*ApplyResult, error) {
	result := &ApplyResult{
		Started:  time.Now(),
		Provider: make(map[string]string),
	}

	// 1. Plan
	states := w.registry.List()
	plan, err := w.planner.PlanWithProviders(email, states)
	if err != nil {
		result.Status = "plan_failed"
		result.Error = err.Error()
		return result, err
	}
	result.Warnings = plan.Warnings

	// 2. Render + Apply each provider
	for provID, pPlan := range plan.Plans {
		p := w.registry.Get(provID)
		if p == nil {
			result.Provider[provID] = "skipped: not_found"
			continue
		}

		configs, err := p.Render(pPlan)
		if err != nil {
			result.Status = "render_failed"
			result.Error = fmt.Sprintf("%s: %v", provID, err)
			result.Provider[provID] = "failed: render"
			w.logApply(ctx, provID, "failed", result.Error)
			return result, err
		}

		if err := p.Apply(configs); err != nil {
			result.Status = "apply_failed"
			result.Error = fmt.Sprintf("%s: %v", provID, err)
			result.Provider[provID] = "failed: apply"
			w.logApply(ctx, provID, "failed", result.Error)
			return result, err
		}

		result.Provider[provID] = "success"
	}

	// 3. Post-apply diagnostic verify
	for provID := range plan.Plans {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		diag := p.Diagnose()
		if diag.LastErrorCode != "" {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%s: %s — %s", provID, diag.LastErrorCode, diag.LastErrorMessage))
		}
	}

	result.Status = "success"
	result.Completed = time.Now()
	if w.planApplied != nil {
		w.planApplied(plan)
	}

	w.logApply(ctx, "all", "success", "")
	return result, nil
}

// ============================================================================
// Rollback — restore last successful config via Provider
// ============================================================================

// Rollback restores the most recent successful apply backup.
// v1.8L-20: supports multi-provider rollback via BackupsPaths.
func (w *Workflow) Rollback(ctx context.Context) error {
	if !w.mu.TryLock() {
		return fmt.Errorf("APPLY_LOCKED")
	}
	defer w.mu.Unlock()

	// Find last successful apply
	last, err := w.applyRepo.FindLastSuccess()
	if err != nil {
		return fmt.Errorf("find last success: %w", err)
	}
	if last == nil {
		return fmt.Errorf("no successful apply to rollback to")
	}

	// Multi-provider rollback (v1.8L-20)
	if len(last.BackupPaths) > 0 {
		for provID, backupPath := range last.BackupPaths {
			if _, err := os.Stat(backupPath); os.IsNotExist(err) {
				return fmt.Errorf("backup file not found for %s: %s", provID, backupPath)
			}
			data, err := os.ReadFile(backupPath)
			if err != nil {
				return fmt.Errorf("read backup for %s: %w", provID, err)
			}
			p := w.registry.Get(provID)
			if p == nil {
				continue
			}
			// Restore config via Apply with the backed-up data
			cf := provider.ConfigFile{Path: backupPath, Content: data}
			if err := p.Apply([]provider.ConfigFile{cf}); err != nil {
				return fmt.Errorf("restore config for %s: %w", provID, err)
			}
		}
		w.logApply(ctx, "all", "rollback", fmt.Sprintf("restored %d providers", len(last.BackupPaths)))
		return nil
	}

	// Legacy single-provider rollback
	if last.BackupPath == "" {
		return fmt.Errorf("no successful apply to rollback to")
	}
	if _, err := os.Stat(last.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", last.BackupPath)
	}

	data, err := os.ReadFile(last.BackupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	caddyPath := w.cfg.Proxy.CaddyfilePath
	if err := os.WriteFile(caddyPath, data, 0640); err != nil {
		return fmt.Errorf("write restored config: %w", err)
	}

	// Reload via the Caddy provider
	reloadProv := w.registry.FindByCapability(provider.CapHotReload)
	if reloadProv == nil {
		return fmt.Errorf("hot-reload provider not found for reload")
	}
	if reloadable, ok := reloadProv.(provider.ReloadableProvider); ok {
		if err := reloadable.Reload(); err != nil {
			return fmt.Errorf("reload after rollback: %w", err)
		}
	}

	// Log rollback against the actual provider that was reloaded (capability-based).
	rollbackProvID := "unknown"
	if reloadProv != nil {
		rollbackProvID = reloadProv.State().ID
	}
	w.logApply(ctx, rollbackProvID, "rollback", fmt.Sprintf("restored from %s", last.BackupPath))
	return nil
}

// ============================================================================
// History
// ============================================================================

// History returns recent apply versions.
func (w *Workflow) History(ctx context.Context) ([]ApplyVersion, error) {
	return w.applyRepo.FindAll(50)
}

// ============================================================================
// Internal
// ============================================================================

func (w *Workflow) logApply(ctx context.Context, provider, status, errMsg string) {
	if w.logSvc == nil {
		return
	}
	w.logSvc.Log(ctx, "apply", "provider", provider, status, errMsg, "system")
}

// logSwitchAudit writes a mode-switch entry to the apply audit log so it
// appears in /api/admin/v1/apply-logs (same table as writeApplyLog uses).
// Uses the same logSvc.LogApply interface as writeApplyLog, not applyRepo.
func (w *Workflow) logSwitchAudit(status, detail string) {
	if w.logSvc == nil {
		return
	}
	w.logSvc.LogApply(&logs.ApplyLog{
		ID:             core.NewID("applylog"),
		Provider:       "switch_mode",
		ValidateStatus: status,
		Stderr:         detail,
		StepLog:        "[]",
		CreatedAt:      time.Now(),
	})
}

// ============================================================================
// Result types
// ============================================================================

// ApplyResult is the outcome of a Workflow.Apply operation.
type ApplyResult struct {
	Status    string            `json:"status"`
	Error     string            `json:"error,omitempty"`
	Warnings  []string          `json:"warnings,omitempty"`
	Provider  map[string]string `json:"provider"`
	Started   time.Time         `json:"started"`
	Completed time.Time         `json:"completed"`
}

// PreviewResult is the outcome of a Workflow.Preview operation.
type PreviewResult struct {
	Plan     *topology.TopologyPlan `json:"plan"`
	Rendered map[string]string      `json:"rendered"`
}

// ============================================================================
// Helpers
// ============================================================================

// planProviderIDs extracts unique provider IDs from a topology plan.
func planProviderIDs(plan *topology.TopologyPlan) []string {
	seen := make(map[string]bool)
	for provID := range plan.Plans {
		seen[provID] = true
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}
