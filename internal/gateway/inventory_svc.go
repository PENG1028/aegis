package gateway

import (
	"fmt"
	"time"

	"aegis/internal/id"
)

// InventoryService provides gateway inventory operations.
type InventoryService struct {
	repo *InventoryRepository
}

// NewInventoryService creates a new inventory service.
func NewInventoryService(repo *InventoryRepository) *InventoryService {
	return &InventoryService{repo: repo}
}

// CreateGateway creates a new gateway inventory entry.
func (s *InventoryService) CreateGateway(input CreateGatewayInput) (*GatewayInventory, error) {
	if input.NodeID == "" {
		return nil, fmt.Errorf("node_id is required")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	now := time.Now()
	g := &GatewayInventory{
		GatewayID:         id.New("gw"),
		NodeID:            input.NodeID,
		Name:              input.Name,
		Type:              input.Type,
		Provider:          input.Provider,
		BindAddr:          input.BindAddr,
		Host:              input.Host,
		Port:              input.Port,
		Scheme:            input.Scheme,
		PublicAccessible:  input.PublicAccessible,
		PrivateAccessible: input.PrivateAccessible,
		Enabled:           true,
		Priority:          input.Priority,
		Status:            GWStatusUnknown,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if g.Type == "" {
		g.Type = GWTypeLocal
	}
	if g.Provider == "" {
		g.Provider = GWProviderAegis
	}
	if g.Scheme == "" {
		g.Scheme = GWSchemeHTTP
	}
	if g.Priority == 0 {
		g.Priority = 100
	}

	if err := s.repo.Create(g); err != nil {
		return nil, err
	}
	return g, nil
}

// UpdateGateway updates a gateway's mutable fields.
func (s *InventoryService) UpdateGateway(gatewayID string, input UpdateGatewayInput) (*GatewayInventory, error) {
	g, err := s.repo.FindByID(gatewayID)
	if err != nil {
		return nil, err
	}
	if g == nil {
		return nil, fmt.Errorf("gateway not found")
	}

	if input.Name != nil {
		g.Name = *input.Name
	}
	if input.Type != nil {
		g.Type = *input.Type
	}
	if input.Provider != nil {
		g.Provider = *input.Provider
	}
	if input.BindAddr != nil {
		g.BindAddr = *input.BindAddr
	}
	if input.Host != nil {
		g.Host = *input.Host
	}
	if input.Port != nil {
		g.Port = *input.Port
	}
	if input.Scheme != nil {
		g.Scheme = *input.Scheme
	}
	if input.PublicAccessible != nil {
		g.PublicAccessible = *input.PublicAccessible
	}
	if input.PrivateAccessible != nil {
		g.PrivateAccessible = *input.PrivateAccessible
	}
	if input.Enabled != nil {
		g.Enabled = *input.Enabled
	}
	if input.Priority != nil {
		g.Priority = *input.Priority
	}

	g.UpdatedAt = time.Now()
	if err := s.repo.Update(g); err != nil {
		return nil, err
	}
	return g, nil
}

// UpdateGatewayFromHeartbeat updates an existing gateway by gateway_id with node ownership enforcement.
func (s *InventoryService) UpdateGatewayFromHeartbeat(nodeID, gatewayID string, gw GatewayInventory) error {
	existing, err := s.repo.FindByID(gatewayID)
	if err != nil {
		return fmt.Errorf("find gateway: %w", err)
	}
	if existing == nil {
		return fmt.Errorf("gateway %s not found", gatewayID)
	}
	if existing.NodeID != nodeID {
		return fmt.Errorf("gateway %s belongs to node %s, not %s", gatewayID, existing.NodeID, nodeID)
	}

	// Update mutable fields from heartbeat
	existing.Host = gw.Host
	existing.Port = gw.Port
	existing.Scheme = gw.Scheme
	existing.BindAddr = gw.BindAddr
	existing.PublicAccessible = gw.PublicAccessible
	existing.PrivateAccessible = gw.PrivateAccessible
	existing.Enabled = gw.Enabled
	existing.LastError = gw.LastError

	// Status semantics: last_error triggers degraded
	if gw.LastError != "" {
		existing.Status = GWStatusDegraded
	} else if gw.Status != "" {
		existing.Status = gw.Status
	} else {
		existing.Status = GWStatusOnline
	}

	existing.UpdatedAt = time.Now()
	return s.repo.Update(existing)
}

// UpsertGatewayFromHeartbeat creates or updates a gateway from heartbeat data.
func (s *InventoryService) UpsertGatewayFromHeartbeat(nodeID string, gw GatewayInventory) error {
	if gw.Name == "" {
		return fmt.Errorf("gateway name required for heartbeat upsert")
	}
	gw.NodeID = nodeID

	// Status semantics
	if gw.LastError != "" {
		gw.Status = GWStatusDegraded
	} else if gw.Status == "" {
		gw.Status = GWStatusOnline
	}

	return s.repo.UpsertByNodeAndName(&gw)
}
