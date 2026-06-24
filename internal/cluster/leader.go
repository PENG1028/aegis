package cluster

import (
	"fmt"
	"time"

	"aegis/internal/node"
)

// LeaderService handles lightweight leader election.
type LeaderService struct {
	nodeRepo *node.Repository
}

// NewLeaderService creates a new leader service.
func NewLeaderService(nodeRepo *node.Repository) *LeaderService {
	return &LeaderService{nodeRepo: nodeRepo}
}

// GetLeader returns the current leader node, or nil if none exists.
func (s *LeaderService) GetLeader() (*node.NodeRecord, error) {
	nodes, err := s.nodeRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("find nodes: %w", err)
	}
	for i := range nodes {
		if nodes[i].IsLeader {
			return &nodes[i], nil
		}
	}
	return nil, nil
}

// ElectLeader selects a leader from available nodes.
// Strategy: picks the most recently seen node that is marked as current.
func (s *LeaderService) ElectLeader() (*node.NodeRecord, error) {
	nodes, err := s.nodeRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("find nodes: %w", err)
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("no nodes available for election")
	}

	// Clear existing leader flags
	for i := range nodes {
		if nodes[i].IsLeader {
			nodes[i].IsLeader = false
			nodes[i].UpdatedAt = time.Now()
			_ = s.nodeRepo.Update(&nodes[i])
		}
	}

	// Pick the current node with most recent last_seen
	var best *node.NodeRecord
	for i := range nodes {
		n := &nodes[i]
		if !n.IsCurrent {
			continue
		}
		if best == nil || n.LastSeen.After(best.LastSeen) {
			best = n
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no current node found for election")
	}

	best.IsLeader = true
	best.LeaderElectedAt = time.Now()
	best.UpdatedAt = time.Now()
	if err := s.nodeRepo.Update(best); err != nil {
		return nil, fmt.Errorf("update leader: %w", err)
	}

	return best, nil
}

// EnsureSingleLeader checks there is exactly one leader. Returns error if not.
func (s *LeaderService) EnsureSingleLeader() error {
	nodes, err := s.nodeRepo.FindAll()
	if err != nil {
		return err
	}

	leaderCount := 0
	var leaderName string
	for i := range nodes {
		if nodes[i].IsLeader {
			leaderCount++
			leaderName = nodes[i].NodeID
		}
	}

	if leaderCount == 0 {
		return fmt.Errorf("no leader elected — run ElectLeader()")
	}
	if leaderCount > 1 {
		return fmt.Errorf("SPLIT_BRAIN: %d leaders detected (%s among them)", leaderCount, leaderName)
	}
	return nil
}

// IsCurrentNodeLeader returns true if the current node is the leader.
func (s *LeaderService) IsCurrentNodeLeader() (bool, error) {
	current, err := s.nodeRepo.FindCurrent()
	if err != nil || current == nil {
		return false, err
	}
	return current.IsLeader, nil
}

// StepDown removes leadership from all nodes (for shutdown or maintenance).
func (s *LeaderService) StepDown() error {
	nodes, err := s.nodeRepo.FindAll()
	if err != nil {
		return err
	}
	for i := range nodes {
		if nodes[i].IsLeader {
			nodes[i].IsLeader = false
			nodes[i].UpdatedAt = time.Now()
			if err := s.nodeRepo.Update(&nodes[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
