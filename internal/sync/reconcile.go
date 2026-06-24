package sync

import (
	"fmt"
	"time"

	"aegis/internal/cluster"
	"aegis/internal/consistency"
	"aegis/internal/node"
)

// ReconcileLoop periodically checks local state against the leader and repairs drift.
type ReconcileLoop struct {
	nodeRepo    *node.Repository
	leaderSvc   *cluster.LeaderService
	stateVer    *cluster.StateVersion
	interval    time.Duration
	stopCh      chan struct{}
}

// NewReconcileLoop creates a reconcile loop.
func NewReconcileLoop(
	nodeRepo *node.Repository,
	leaderSvc *cluster.LeaderService,
	stateVer *cluster.StateVersion,
) *ReconcileLoop {
	return &ReconcileLoop{
		nodeRepo:  nodeRepo,
		leaderSvc: leaderSvc,
		stateVer:  stateVer,
		interval:  10 * time.Second,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the reconcile loop in a background goroutine.
func (rl *ReconcileLoop) Start() {
	go func() {
		ticker := time.NewTicker(rl.interval)
		defer ticker.Stop()

		for {
			select {
			case <-rl.stopCh:
				return
			case <-ticker.C:
				rl.reconcile()
			}
		}
	}()
}

// Stop stops the reconcile loop.
func (rl *ReconcileLoop) Stop() {
	close(rl.stopCh)
}

func (rl *ReconcileLoop) reconcile() {
	isLeader, _ := rl.leaderSvc.IsCurrentNodeLeader()
	if isLeader {
		return // leader doesn't need to reconcile from itself
	}

	leader, err := rl.leaderSvc.GetLeader()
	if err != nil || leader == nil {
		return // no leader to sync from
	}

	// Compare versions
	localVersion := rl.stateVer.Current()
	leaderVersion := leader.StateVersion
	if leaderVersion <= localVersion {
		return // already synced
	}

	// Drift detected — trigger repair
	fmt.Printf("reconcile: local v%d < leader v%d — syncing\n", localVersion, leaderVersion)

	// Check for drift
	report := consistency.CheckDrift(rl.nodeRepo, localVersion, leaderVersion)
	if report.HasDrift {
		fmt.Printf("reconcile: DRIFT_DETECTED severity=%s\n", report.Severity)
	}

	// Repair: set local version to leader version
	if err := rl.stateVer.Set(leaderVersion); err != nil {
		fmt.Printf("reconcile: repair failed: %v\n", err)
		return
	}

	fmt.Printf("reconcile: synced to v%d\n", leaderVersion)
}

// TriggerNow forces an immediate reconciliation.
func (rl *ReconcileLoop) TriggerNow() {
	rl.reconcile()
}

// StateDiffRequest is sent from a node to the leader to request state.
type StateDiffRequest struct {
	NodeID       string `json:"node_id"`
	LocalVersion uint64 `json:"local_version"`
}

// StateDiffResponse is the leader's response with current state.
type StateDiffResponse struct {
	LeaderID     string `json:"leader_id"`
	StateVersion uint64 `json:"state_version"`
	NeedsSync    bool   `json:"needs_sync"`
	Message      string `json:"message"`
}
