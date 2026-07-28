package apply

import (
	"context"
	"strings"
	"testing"
)

type pendingRecorder struct{ reason string }

func (p *pendingRecorder) ClearPending() error                  { return nil }
func (p *pendingRecorder) ClearPendingIfUnchanged(string) error { return nil }
func (p *pendingRecorder) MarkPending(reason string) error      { p.reason = reason; return nil }
func (p *pendingRecorder) PendingRevision() string              { return "" }

func TestTryApplyLockContentionMarksPending(t *testing.T) {
	pending := &pendingRecorder{}
	svc := &AppService{pendingState: pending}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if _, err := svc.TryApply(context.Background()); err == nil || !strings.Contains(err.Error(), "APPLY_LOCKED") {
		t.Fatalf("expected APPLY_LOCKED, got %v", err)
	}
	if pending.reason == "" {
		t.Fatal("lock contention did not preserve a pending Apply marker")
	}
}

func TestSwitchModeUsesTopLevelApplyLock(t *testing.T) {
	svc := &AppService{}
	svc.mu.Lock()
	defer svc.mu.Unlock()

	if err := svc.SwitchMode(context.Background(), "legacy"); err == nil || !strings.Contains(err.Error(), "APPLY_LOCKED") {
		t.Fatalf("expected APPLY_LOCKED, got %v", err)
	}
}
