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
	"sync"
	"time"

	"aegis/internal/config"
	"aegis/internal/logs"
	"aegis/internal/provider"
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
	planner   *topology.Planner
	registry  *provider.Registry
	applyRepo *Repository
	cfg       *config.Config
	logSvc    logs.Logger
	mu        sync.Mutex
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

// Plan generates a topology plan without applying.
func (w *Workflow) Plan(ctx context.Context, email string) (*topology.TopologyPlan, error) {
	states := w.registry.List()
	return w.planner.PlanWithProviders(email, states)
}

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
	caddy := w.registry.Get("caddy")
	if caddy == nil {
		return "", fmt.Errorf("caddy provider not found")
	}
	if reader, ok := caddy.(provider.ConfigReader); ok {
		return reader.GetCurrentConfig()
	}
	return "", fmt.Errorf("caddy provider does not support config reading")
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

	w.logApply(ctx, "all", "success", "")
	return result, nil
}

// ============================================================================
// Rollback — restore last successful config via Provider
// ============================================================================

// Rollback restores the most recent successful apply backup.
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
	if last == nil || last.BackupPath == "" {
		return fmt.Errorf("no successful apply to rollback to")
	}
	if _, err := os.Stat(last.BackupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", last.BackupPath)
	}

	// Restore backup to Caddyfile path and reload
	data, err := os.ReadFile(last.BackupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	caddyPath := w.cfg.Proxy.CaddyfilePath
	if err := os.WriteFile(caddyPath, data, 0640); err != nil {
		return fmt.Errorf("write restored config: %w", err)
	}

	// Reload via the Caddy provider
	caddy := w.registry.Get("caddy")
	if caddy == nil {
		return fmt.Errorf("caddy provider not found for reload")
	}
	if reloadable, ok := caddy.(provider.ReloadableProvider); ok {
		if err := reloadable.Reload(); err != nil {
			return fmt.Errorf("reload after rollback: %w", err)
		}
	}

	w.logApply(ctx, "caddy", "rollback", fmt.Sprintf("restored from %s", last.BackupPath))
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
