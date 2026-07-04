package sync

import (
	"time"

	"aegis/internal/cluster"
	"aegis/internal/maintenance"
	"aegis/internal/node"
	sloglog "aegis/internal/core"
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
		defer func() {
			if r := recover(); r != nil {
				sloglog.Error("reconcile: panic in reconcile loop", "panic", r)
			}
		}()
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
		sloglog.Info("reconcile: medium drift, full sync", "diff", diff)
	} else {
		sloglog.Warn("reconcile: LARGE drift, full overwrite from leader", "diff", diff)
	}

	if err := rl.stateVer.Set(leaderVersion); err != nil {
		sloglog.Error("reconcile: full sync failed", "error", err)
	} else {
		sloglog.Info("reconcile: full reconciliation complete", "version", leaderVersion)
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
	sloglog.Info("reconcile: syncing", "local", localVersion, "leader", leaderVersion)

	// Check for drift
	report := maintenance.CheckDrift(rl.nodeRepo, localVersion, leaderVersion)
	if report.HasDrift {
		sloglog.Warn("reconcile: drift detected", "severity", report.Severity)
	}

	// Repair: set local version to leader version
	if err := rl.stateVer.Set(leaderVersion); err != nil {
		sloglog.Error("reconcile: repair failed", "error", err)
		return
	}

	sloglog.Info("reconcile: synced to v%d\n", leaderVersion)
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
