package sync

import (
	"fmt"
	"time"

	"aegis/internal/cluster"
	"aegis/internal/consistency"
	"aegis/internal/node"
)

// ReconcileLoop periodically checks local state against the leader and repairs drift.
// Uses adaptive intervals: 10s fast sync → 60s partial → 300s full reconciliation.
type ReconcileLoop struct {
	nodeRepo    *node.Repository
	leaderSvc   *cluster.LeaderService
	stateVer    *cluster.StateVersion
	fastInterval time.Duration
	fullInterval time.Duration
	cycleCount   int
	stopCh       chan struct{}
}

// NewReconcileLoop creates a reconcile loop with adaptive intervals.
func NewReconcileLoop(
	nodeRepo *node.Repository,
	leaderSvc *cluster.LeaderService,
	stateVer *cluster.StateVersion,
) *ReconcileLoop {
	return &ReconcileLoop{
		nodeRepo:     nodeRepo,
		leaderSvc:    leaderSvc,
		stateVer:     stateVer,
		fastInterval: 10 * time.Second,
		fullInterval: 300 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

// Start begins the adaptive reconcile loop.
// 10s fast sync → 60s partial → 300s full reconciliation.
func (rl *ReconcileLoop) Start() {
	go func() {
		fastTicker := time.NewTicker(rl.fastInterval)
		fullTicker := time.NewTicker(rl.fullInterval)
		defer fastTicker.Stop()
		defer fullTicker.Stop()

		for {
			select {
			case <-rl.stopCh:
				return
			case <-fastTicker.C:
				rl.reconcile() // fast sync
				rl.cycleCount++
			case <-fullTicker.C:
				rl.fullReconciliation() // deep sync every 300s
			}
		}
	}()
}

// fullReconciliation performs a complete state sync from leader.
func (rl *ReconcileLoop) fullReconciliation() {
	isLeader, _ := rl.leaderSvc.IsCurrentNodeLeader()
	if isLeader {
		return
	}
	leader, err := rl.leaderSvc.GetLeader()
	if err != nil || leader == nil {
		return
	}

	localVersion := rl.stateVer.Current()
	leaderVersion := leader.StateVersion

	if leaderVersion <= localVersion {
		return
	}

	diff := leaderVersion - localVersion
	if diff <= 2 {
		return // small diff handled by fast sync
	}
	if diff <= 10 {
		// Medium diff → full sync
		fmt.Printf("reconcile: medium drift (%d versions), full sync\n", diff)
	} else {
		// Large diff → full overwrite from leader
		fmt.Printf("reconcile: LARGE drift (%d versions), full overwrite from leader\n", diff)
	}

	if err := rl.stateVer.Set(leaderVersion); err != nil {
		fmt.Printf("reconcile: full sync failed: %v\n", err)
	} else {
		fmt.Printf("reconcile: full reconciliation to v%d complete\n", leaderVersion)
	}
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
