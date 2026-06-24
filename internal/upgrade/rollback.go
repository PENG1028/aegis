package upgrade

import (
	"fmt"

	"aegis/internal/cluster"
)

// RollbackController handles automated rollback on upgrade failure.
type RollbackController struct {
	session   *Session
	tracker   *Tracker
	leaderSvc *cluster.LeaderService
	stateVer  *cluster.StateVersion
}

// NewRollbackController creates a rollback controller.
func NewRollbackController(
	session *Session,
	tracker *Tracker,
	leaderSvc *cluster.LeaderService,
	stateVer *cluster.StateVersion,
) *RollbackController {
	return &RollbackController{
		session:   session,
		tracker:   tracker,
		leaderSvc: leaderSvc,
		stateVer:  stateVer,
	}
}

// ShouldRollback determines if a rollback is needed based on the error and state.
func (rc *RollbackController) ShouldRollback(err error, driftSeverity string, healthGatePassed bool) bool {
	if err != nil {
		return true
	}
	if driftSeverity == "HIGH" {
		return true
	}
	if !healthGatePassed {
		return true
	}
	return false
}

// Execute performs the rollback sequence.
func (rc *RollbackController) Execute(reason string) error {
	rc.tracker.Record(StepRollback, "running", reason)

	// 1. Revert state_version to start
	if rc.session.StateVersionStart > 0 {
		if err := rc.stateVer.Set(rc.session.StateVersionStart); err != nil {
			rc.tracker.RecordFailed(StepRollback, fmt.Errorf("revert state_version: %w", err))
			return err
		}
		rc.tracker.RecordSuccess(StepRollback, fmt.Sprintf("state_version reverted to %d", rc.session.StateVersionStart))
	}

	// 2. Mark session as rolled back
	rc.session.MarkRolledBack(reason)

	return nil
}
