package endpoint

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// MutationHook is called after endpoint mutations to trigger desired state regeneration.
type MutationHook interface {
	OnEndpointChanged(ctx context.Context, endpointID string) error
}

// AppService provides endpoint business logic with mutation hook support.
type AppService struct {
	repo   *Repository
	logSvc *logs.AppService
	hook   MutationHook
}

// NewAppService creates a new endpoint application service.
func NewAppService(repo *Repository, logSvc *logs.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// SetMutationHook sets the mutation hook for desired state regeneration.
func (s *AppService) SetMutationHook(hook MutationHook) {
	s.hook = hook
}

// CreateEndpoint creates a new endpoint and triggers the mutation hook.
func (s *AppService) CreateEndpoint(ctx context.Context, input CreateEndpointInput) (*Endpoint, error) {
	if input.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	if input.Address == "" {
		return nil, fmt.Errorf("address is required")
	}
	if input.Type == "" {
		input.Type = "local"
	}

	now := time.Now()
	ep := &Endpoint{
		ID:        id.New("ep"),
		ServiceID: input.ServiceID,
		Type:      input.Type,
		Address:   input.Address,
		Enabled:   true,
		NodeID:    input.NodeID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ep); err != nil {
		s.logSvc.Log(ctx, "endpoint.create", "endpoint", ep.ID, "failed", err.Error(), "api")
		return nil, fmt.Errorf("create endpoint: %w", err)
	}

	s.logSvc.Log(ctx, "endpoint.create", "endpoint", ep.ID, "success",
		fmt.Sprintf("created %s endpoint %s", input.Type, input.Address), "api")

	if s.hook != nil {
		_ = s.hook.OnEndpointChanged(ctx, ep.ID)
	}

	return ep, nil
}

// EnableEndpoint enables an endpoint and triggers the mutation hook.
func (s *AppService) EnableEndpoint(ctx context.Context, endpointID string) error {
	ep, err := s.repo.FindByID(endpointID)
	if err != nil {
		return err
	}
	if ep == nil {
		return fmt.Errorf("endpoint %q not found", endpointID)
	}
	if ep.Enabled {
		return fmt.Errorf("endpoint %s is already enabled", ep.ID)
	}

	ep.Enabled = true
	ep.UpdatedAt = time.Now()
	if err := s.repo.Update(ep); err != nil {
		return fmt.Errorf("enable endpoint: %w", err)
	}

	s.logSvc.Log(ctx, "endpoint.enable", "endpoint", ep.ID, "success", "enabled endpoint", "api")

	if s.hook != nil {
		_ = s.hook.OnEndpointChanged(ctx, ep.ID)
	}

	return nil
}

// DisableEndpoint disables an endpoint and triggers the mutation hook.
func (s *AppService) DisableEndpoint(ctx context.Context, endpointID string) error {
	ep, err := s.repo.FindByID(endpointID)
	if err != nil {
		return err
	}
	if ep == nil {
		return fmt.Errorf("endpoint %q not found", endpointID)
	}
	if !ep.Enabled {
		return fmt.Errorf("endpoint %s is already disabled", ep.ID)
	}

	ep.Enabled = false
	ep.UpdatedAt = time.Now()
	if err := s.repo.Update(ep); err != nil {
		return fmt.Errorf("disable endpoint: %w", err)
	}

	s.logSvc.Log(ctx, "endpoint.disable", "endpoint", ep.ID, "success", "disabled endpoint", "api")

	if s.hook != nil {
		_ = s.hook.OnEndpointChanged(ctx, ep.ID)
	}

	return nil
}

// UpdateEndpoint updates an endpoint and triggers the mutation hook.
func (s *AppService) UpdateEndpoint(ctx context.Context, ep *Endpoint) error {
	ep.UpdatedAt = time.Now()
	if err := s.repo.Update(ep); err != nil {
		return fmt.Errorf("update endpoint: %w", err)
	}

	s.logSvc.Log(ctx, "endpoint.update", "endpoint", ep.ID, "success", "updated endpoint", "api")

	if s.hook != nil {
		_ = s.hook.OnEndpointChanged(ctx, ep.ID)
	}

	return nil
}

// DeleteEndpoint removes an endpoint and triggers the mutation hook.
func (s *AppService) DeleteEndpoint(ctx context.Context, endpointID string) error {
	if err := s.repo.Delete(endpointID); err != nil {
		return fmt.Errorf("delete endpoint: %w", err)
	}

	s.logSvc.Log(ctx, "endpoint.delete", "endpoint", endpointID, "success", "deleted endpoint", "api")

	if s.hook != nil {
		_ = s.hook.OnEndpointChanged(ctx, endpointID)
	}

	return nil
}

// --- Delegate read methods ---

// FindByID returns an endpoint by ID.
func (s *AppService) FindByID(id string) (*Endpoint, error) {
	return s.repo.FindByID(id)
}

// FindByServiceID returns all endpoints for a service.
func (s *AppService) FindByServiceID(serviceID string) ([]Endpoint, error) {
	return s.repo.FindByServiceID(serviceID)
}

// FindEnabledByServiceID returns enabled endpoints ordered by type priority.
func (s *AppService) FindEnabledByServiceID(serviceID string) ([]Endpoint, error) {
	return s.repo.FindEnabledByServiceID(serviceID)
}

// FindByNodeID returns all endpoints for a given node.
func (s *AppService) FindByNodeID(nodeID string) ([]Endpoint, error) {
	return s.repo.FindByNodeID(nodeID)
}

// Repo returns the underlying repository for components that need direct access.
func (s *AppService) Repo() *Repository {
	return s.repo
}
