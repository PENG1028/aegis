package nodestate

import (
	"encoding/json"
	"fmt"
	"time"

	"aegis/internal/id"
)

// Service manages node desired and actual state.
type Service struct {
	repo *Repository
}

// NewService creates a new nodestate service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ============================================================================
// Desired State Operations
// ============================================================================

// CreateDesiredState creates a new desired state for a node with auto-revision.
func (s *Service) CreateDesiredState(input CreateDesiredStateInput) (*DesiredState, error) {
	// Validate state_json is valid JSON
	if !json.Valid([]byte(input.StateJSON)) {
		return nil, fmt.Errorf("invalid JSON in state_json")
	}

	// Validate node exists (responsibility of caller to check)
	if input.NodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}

	// Compute hash
	hash, err := ComputeStateHash(input.StateJSON)
	if err != nil {
		return nil, fmt.Errorf("compute state hash: %w", err)
	}

	// Get next revision
	latestRev, err := s.repo.GetLatestRevision(input.NodeID)
	if err != nil {
		return nil, fmt.Errorf("get latest revision: %w", err)
	}
	nextRev := latestRev + 1

	// Supersede previous active state
	if latestRev > 0 {
		if err := s.repo.SupersedePrevious(input.NodeID, nextRev); err != nil {
			return nil, fmt.Errorf("supersede previous: %w", err)
		}
	}

	now := time.Now()
	ds := &DesiredState{
		ID:        id.New("ds"),
		NodeID:    input.NodeID,
		Revision:  nextRev,
		StateHash: hash,
		StateJSON: input.StateJSON,
		Status:    DSStatusActive,
		Reason:    input.Reason,
		CreatedBy: input.CreatedBy,
		CreatedAt: now,
	}

	if err := s.repo.CreateDesiredState(ds); err != nil {
		return nil, err
	}

	return ds, nil
}

// GetLatestDesiredState returns the latest active desired state for a node.
func (s *Service) GetLatestDesiredState(nodeID string) (*DesiredState, error) {
	return s.repo.GetLatestDesiredState(nodeID)
}

// GetDesiredStateByRevision returns a specific revision for a node.
func (s *Service) GetDesiredStateByRevision(nodeID string, revision int) (*DesiredState, error) {
	return s.repo.GetDesiredStateByRevision(nodeID, revision)
}

// ListDesiredStates returns all desired states for a node.
func (s *Service) ListDesiredStates(nodeID string) ([]DesiredState, error) {
	return s.repo.ListDesiredStates(nodeID)
}

// GetLatestRevision returns the latest revision for a node.
func (s *Service) GetLatestRevision(nodeID string) (int, error) {
	return s.repo.GetLatestRevision(nodeID)
}

// ============================================================================
// Actual State Operations
// ============================================================================

// ReportActualState records a node's actual applied state.
func (s *Service) ReportActualState(nodeID string, appliedRevision int, stateHash, status, lastError, providerStatus, relayStatus, gatewayStatus, diagnosticsStatus string) (*ActualState, error) {
	now := time.Now()

	as := &ActualState{
		ID:               id.New("as"),
		NodeID:           nodeID,
		AppliedRevision:  appliedRevision,
		StateHash:        stateHash,
		Status:           status,
		LastError:        lastError,
		ProviderStatus:   providerStatus,
		RelayStatus:      relayStatus,
		GatewayStatus:    gatewayStatus,
		DiagnosticsStatus: diagnosticsStatus,
		ReportedAt:       now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if status == ASStatusApplied {
		as.LastApplyAt = now
		as.LastSuccessAt = now
	} else if status == ASStatusFailed || status == ASStatusDegraded {
		as.LastApplyAt = now
	}

	if err := s.repo.UpsertActualState(as); err != nil {
		return nil, err
	}

	return as, nil
}

// GetActualState returns the latest actual state for a node.
func (s *Service) GetActualState(nodeID string) (*ActualState, error) {
	return s.repo.GetActualState(nodeID)
}

// ============================================================================
// Sync Status
// ============================================================================

// GetSyncStatus compares desired and actual state and returns sync status.
func (s *Service) GetSyncStatus(nodeID string) (*SyncStatus, error) {
	ss := &SyncStatus{NodeID: nodeID}

	desired, err := s.repo.GetLatestDesiredState(nodeID)
	if err != nil {
		return nil, fmt.Errorf("get desired state: %w", err)
	}

	actual, err := s.repo.GetActualState(nodeID)
	if err != nil {
		return nil, fmt.Errorf("get actual state: %w", err)
	}

	if desired == nil && actual == nil {
		ss.Status = SyncNoDesiredState
		return ss, nil
	}

	if desired == nil {
		ss.Status = SyncNoDesiredState
		if actual != nil {
			ss.AppliedRevision = actual.AppliedRevision
			ss.ActualHash = actual.StateHash
		}
		return ss, nil
	}

	ss.DesiredRevision = desired.Revision
	ss.DesiredHash = desired.StateHash

	if actual == nil {
		ss.Status = SyncNoActualState
		return ss, nil
	}

	ss.AppliedRevision = actual.AppliedRevision
	ss.ActualHash = actual.StateHash

	// Determine sync status
	switch actual.Status {
	case ASStatusFailed:
		ss.Status = SyncFailed
		ss.LastError = actual.LastError
		return ss, nil
	case ASStatusDegraded:
		ss.Status = SyncDegraded
		ss.LastError = actual.LastError
		return ss, nil
	}

	if actual.AppliedRevision < desired.Revision {
		ss.Status = SyncOutdated
		return ss, nil
	}

	if actual.StateHash == desired.StateHash {
		ss.Status = SyncInSync
	} else {
		ss.Status = SyncOutdated // hash mismatch even if revision matches
	}

	return ss, nil
}

// CompareNodeRevision returns revision comparison for use in heartbeat responses.
func (s *Service) CompareNodeRevision(nodeID string, appliedRevision int) (latestRevision int, desiredAvailable bool, outdated bool, err error) {
	latest, err := s.repo.GetLatestRevision(nodeID)
	if err != nil {
		return 0, false, false, err
	}
	if latest > 0 {
		desiredAvailable = true
	}
	if appliedRevision < latest {
		outdated = true
	}
	return latest, desiredAvailable, outdated, nil
}
