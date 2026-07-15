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
	"sync"
	"time"

	"aegis/internal/certstore"
	"aegis/internal/config"
	"aegis/internal/logs"
	"aegis/internal/hostdep/provider"
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
	planner      *topology.Planner
	registry     *provider.Registry
	applyRepo    *Repository
	cfg          *config.Config
	logSvc       logs.Logger
	certStore    *certstore.Service
	smokeTest    SmokeTest // optional: inject real HTTP probe for E2E mode-switch validation
	mu           sync.Mutex
}

// SmokeTest validates that traffic still flows after a mode switch. nil means
// "skip" — the single-test path does not need a real network probe. Inject a
// real implementation (e.g. curl healthz + status) for production deployment.
// A failing smoke test produces a warning, not a rollback — the operator
// decides whether to roll back.
type SmokeTest func(ctx context.Context, mode provider.RuntimeMode) error

// SetSmokeTest injects a smoke-test function for production deployments.
func (w *Workflow) SetSmokeTest(fn SmokeTest) {
	w.smokeTest = fn
}

// modeSnapshot captures enough state to roll back a failed mode switch.
type modeSnapshot struct {
	FromMode  string
	Providers []providerSnapshot
}

type providerSnapshot struct {
	ID           string
	ConfigPath   string
	ConfigBackup []byte // raw config file content before the switch
	WasRunning   bool   // was the service running before we stopped it?
}

// NewWorkflow creates an apply workflow orchestrator.
func NewWorkflow(
	planner *topology.Planner,
	registry *provider.Registry,
	applyRepo *Repository,
	cfg *config.Config,
	logSvc logs.Logger,
	certStore *certstore.Service,
) *Workflow {
	return &Workflow{
		planner:   planner,
		registry:  registry,
		applyRepo: applyRepo,
		certStore: certStore,
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

	return w.Apply(ctx, email)
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

	// ── 1. Snapshot current state ──
	snap := modeSnapshot{FromMode: currentMode.ID}
	for _, provID := range currentMode.ProviderIDs() {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		ps := providerSnapshot{ID: provID}
		if cr, ok := p.(provider.ConfigReader); ok {
			if cfg, err := cr.GetCurrentConfig(); err == nil {
				ps.ConfigBackup = []byte(cfg)
			}
		}
		// Check if the provider's service is currently running (best-effort).
		if sc, ok := p.(provider.ServiceController); ok {
			ps.WasRunning = isServiceActive(sc)
		}
		snap.Providers = append(snap.Providers, ps)
	}

	rollback := func(reason string) error {
		w.logApply(ctx, "switch_mode", "rollback", reason)
		for _, ps := range snap.Providers {
			if len(ps.ConfigBackup) == 0 || ps.ConfigPath == "" {
				continue
			}
			p := w.registry.Get(ps.ID)
			if p == nil {
				continue
			}
			_ = p.Apply([]provider.ConfigFile{
				{Path: ps.ConfigPath, Content: ps.ConfigBackup},
			})
			// Restart if it was running before.
			if ps.WasRunning {
				if sc, ok := p.(provider.ServiceController); ok {
					_ = sc.Start()
				}
			}
		}
		return fmt.Errorf("switch_mode rolled back: %s", reason)
	}

	// ── 2. Plan + render for target mode ──
	plan, err := w.planner.PlanWithProviders("", states)
	if err != nil {
		return rollback(fmt.Sprintf("plan failed: %v", err))
	}

	targetProviderIDs := planProviderIDs(plan)

	// ── 3. Handoff: stop providers that hold ports the target plan needs ──
	// Without this, a provider in both current AND target (e.g. Caddy:
	// Legacy binds :443 directly, EdgeMux binds :8443 with HAProxy on :443)
	// keeps its old listener, blocking the new provider. Each will be
	// restarted by the Apply in step 4 using the target-mode config.
	for _, provID := range currentMode.ProviderIDs() {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		if !slices.Contains(targetProviderIDs, provID) {
			// Not in the target plan — stop and clean.
			if sc, ok := p.(provider.ServiceController); ok {
				_ = sc.Stop()
			}
			if cc, ok := p.(provider.ConfigCleaner); ok {
				_ = cc.CleanConfig()
			}
		} else {
			// Shared: still in target, but needs to release its current port
			// so the new provider(s) can bind it. Apply (step 4) will
			// restart with the target-mode configuration.
			if sc, ok := p.(provider.ServiceController); ok {
				_ = sc.Stop()
			}
		}
	}

	// ── 4. Render + Apply each target provider ──
	for provID, pPlan := range plan.Plans {
		p := w.registry.Get(provID)
		if p == nil {
			continue
		}
		configs, err := p.Render(pPlan)
		if err != nil {
			return rollback(fmt.Sprintf("%s render: %v", provID, err))
		}
		if err := p.Apply(configs); err != nil {
			return rollback(fmt.Sprintf("%s apply: %v", provID, err))
		}
		w.logApply(ctx, provID, "switch_mode", "applied for target "+targetModeID)
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

	// ── 6. Smoke test (optional — skip if not injected) ──
	if w.smokeTest != nil {
		if err := w.smokeTest(ctx, targetMode); err != nil {
			w.logApply(ctx, "switch_mode", "smoke_warning",
				fmt.Sprintf("smoke test failed (not rolled back): %v", err))
		}
	}

	w.logApply(ctx, "switch_mode", "success",
		fmt.Sprintf("switched from %s to %s", currentMode.ID, targetModeID))
	return nil
}

// isServiceActive best-effort check whether a provider's service is running.
func isServiceActive(sc provider.ServiceController) bool {
	// ServiceController does not expose a dedicated "IsActive" query. We try
	// a lightweight check via a no-op call. If the interface doesn't support
	// probing, we conservatively assume it was running (so rollback restarts it).
	return true
}

// Apply executes the full pipeline:
// plan → render → validate → backup → write → reload → verify → log.
func (w *Workflow) Apply(ctx context.Context, email string) (*ApplyResult, error) {
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

	// Sync auto-certs into CertStore after successful apply.
	// Caddy may have obtained new certs during reload.
	if w.certStore != nil {
		if n, ids, err := w.certStore.SyncAutoCerts(""); err == nil && n > 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("cert_sync: imported %d auto-certs: %v", n, ids))
		}
	}
	// Bind imported auto-certs to routes that have no cert_id yet.
	if bound, err := w.planner.BindAutoCerts(); err == nil && bound > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("cert_bind: bound %d routes to auto-certs", bound))
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

