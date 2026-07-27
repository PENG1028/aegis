package topology

import (
	"fmt"
)

// Service manages topology edges.
type Service struct {
	repo *Repository
}

// NewService creates a new topology service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CreateOrUpdateEdge creates or updates a topology edge.
func (s *Service) CreateOrUpdateEdge(input CreateEdgeInput) (*TopologyEdge, error) {
	if input.FromNodeID == "" || input.ToNodeID == "" {
		return nil, fmt.Errorf("from_node_id and to_node_id are required")
	}
	if input.FromNodeID == input.ToNodeID {
		return nil, fmt.Errorf("from_node_id cannot equal to_node_id")
	}

	e := &TopologyEdge{
		FromNodeID:         input.FromNodeID,
		ToNodeID:           input.ToNodeID,
		PreferredGatewayID: input.PreferredGatewayID,
		GatewayLinkID:      input.GatewayLinkID,
		Status:             StatusUnknown,
	}
	if input.PrivateReachable != nil {
		e.PrivateReachable = *input.PrivateReachable
	}
	if input.PublicReachable != nil {
		e.PublicReachable = *input.PublicReachable
	}

	if err := s.repo.CreateOrUpdateEdge(e); err != nil {
		return nil, err
	}
	return e, nil
}

// GetEdge returns the edge between two nodes.
func (s *Service) GetEdge(fromNodeID, toNodeID string) (*TopologyEdge, error) {
	return s.repo.GetEdge(fromNodeID, toNodeID)
}

// ListEdges returns all topology edges.
func (s *Service) ListEdges() ([]TopologyEdge, error) {
	return s.repo.ListEdges()
}

// GetMatrix returns the full topology matrix (all edges for display).
func (s *Service) GetMatrix() ([]TopologyEdge, error) {
	return s.repo.ListEdges()
}

// GetPath returns the path between two nodes.
func (s *Service) GetPath(fromNodeID, toNodeID string) (*PathResult, error) {
	edge, err := s.repo.GetEdge(fromNodeID, toNodeID)
	if err != nil {
		return nil, err
	}
	if edge == nil {
		return &PathResult{
			FromNodeID: fromNodeID,
			ToNodeID:   toNodeID,
			Status:     StatusUnknown,
		}, nil
	}
	return &PathResult{
		FromNodeID:         edge.FromNodeID,
		ToNodeID:           edge.ToNodeID,
		PrivateReachable:   edge.PrivateReachable,
		PublicReachable:    edge.PublicReachable,
		PreferredGatewayID: edge.PreferredGatewayID,
		GatewayLinkID:      edge.GatewayLinkID,
		Status:             edge.Status,
	}, nil
}

// SetEdgeStatus updates an edge's status by node pair.
func (s *Service) SetEdgeStatus(fromNodeID, toNodeID, status, lastError string) error {
	edge, err := s.repo.GetEdge(fromNodeID, toNodeID)
	if err != nil {
		return err
	}
	if edge == nil {
		return fmt.Errorf("edge not found between %s and %s", fromNodeID, toNodeID)
	}
	return s.repo.SetStatus(edge.ID, status, lastError)
}
