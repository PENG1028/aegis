package service

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// MutationHook is called after service mutations to trigger desired state regeneration.
type MutationHook interface {
	OnServiceChanged(ctx context.Context, serviceID string) error
}

// AppService defines the service application service interface.
type AppService struct {
	repo   *Repository
	logSvc logs.Logger
	hook   MutationHook
}

// NewAppService creates a new service application service.
func NewAppService(repo *Repository, logSvc logs.Logger) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// SetMutationHook sets the mutation hook for desired state regeneration.
func (s *AppService) SetMutationHook(hook MutationHook) {
	s.hook = hook
}

// CreateService creates a new backend service.
func (s *AppService) CreateService(ctx context.Context, input CreateServiceInput) (*Service, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("service name is required")
	}
	if input.ProjectID == "" {
		return nil, fmt.Errorf("project is required")
	}
	if input.Kind == "" {
		input.Kind = "http"
	}
	if input.Env == "" {
		input.Env = "prod"
	}

	// Check for duplicate name
	existing, err := s.repo.FindByName(input.Name)
	if err != nil {
		return nil, fmt.Errorf("check duplicate service name: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("service with name %q already exists", input.Name)
	}

	now := time.Now()
	svc := &Service{
		ID:        id.New("svc"),
		ProjectID: input.ProjectID,
		Name:      input.Name,
		Kind:      input.Kind,
		Env:       input.Env,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(svc); err != nil {
		s.logSvc.Log(ctx, "service.create", "service", svc.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("create service: %w", err)
	}

	if s.hook != nil {
		if err := s.hook.OnServiceChanged(ctx, svc.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "service", svc.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "service.create", "service", svc.ID, "success",
		fmt.Sprintf("created service %q (kind=%s) in env %s", svc.Name, svc.Kind, svc.Env), "cli")
	return svc, nil
}

// ListServices returns all services, optionally filtered by project.
func (s *AppService) ListServices(ctx context.Context) ([]Service, error) {
	services, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	if services == nil {
		services = []Service{}
	}
	return services, nil
}

// GetService finds a service by ID or name.
func (s *AppService) GetService(ctx context.Context, idOrName string) (*Service, error) {
	svc, err := s.repo.FindByID(idOrName)
	if err != nil {
		return nil, fmt.Errorf("find service: %w", err)
	}
	if svc != nil {
		return svc, nil
	}

	svc, err = s.repo.FindByName(idOrName)
	if err != nil {
		return nil, fmt.Errorf("find service: %w", err)
	}
	if svc == nil {
		return nil, fmt.Errorf("service %q not found", idOrName)
	}
	return svc, nil
}

// EnableService enables a disabled service.
func (s *AppService) EnableService(ctx context.Context, idOrName string) error {
	svc, err := s.GetService(ctx, idOrName)
	if err != nil {
		return err
	}

	if svc.Status == "active" {
		return fmt.Errorf("service %q is already active", svc.Name)
	}

	svc.Status = "active"
	svc.UpdatedAt = time.Now()

	if err := s.repo.Update(svc); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}

	if s.hook != nil {
		if err := s.hook.OnServiceChanged(ctx, svc.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "service", svc.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "service.enable", "service", svc.ID, "success",
		fmt.Sprintf("enabled service %q", svc.Name), "cli")
	return nil
}

// DisableService disables a service.
func (s *AppService) DisableService(ctx context.Context, idOrName string) error {
	svc, err := s.GetService(ctx, idOrName)
	if err != nil {
		return err
	}

	if svc.Status == "disabled" {
		return fmt.Errorf("service %q is already disabled", svc.Name)
	}

	svc.Status = "disabled"
	svc.UpdatedAt = time.Now()

	if err := s.repo.Update(svc); err != nil {
		return fmt.Errorf("disable service: %w", err)
	}

	if s.hook != nil {
		if err := s.hook.OnServiceChanged(ctx, svc.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "service", svc.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "service.disable", "service", svc.ID, "success",
		fmt.Sprintf("disabled service %q", svc.Name), "cli")
	return nil
}

// CreateServiceDirect creates a pre-built service directly via the repository.
// Used by the action service to create services with ownership fields set.
func (s *AppService) CreateServiceDirect(svc *Service) error {
	if err := s.repo.Create(svc); err != nil {
		return err
	}
	if s.hook != nil {
		if err := s.hook.OnServiceChanged(context.Background(), svc.ID); err != nil {
			s.logSvc.Log(context.Background(), "desired-state.regen", "service", svc.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}
	return nil
}

// ListServicesBySpaceID returns all services for a specific space.
func (s *AppService) ListServicesBySpaceID(ctx context.Context, spaceID string) ([]Service, error) {
	services, err := s.repo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("list services by space: %w", err)
	}
	if services == nil {
		services = []Service{}
	}
	return services, nil
}

// UpdateService updates a service's fields.
func (s *AppService) UpdateService(ctx context.Context, idOrName string, input UpdateServiceInput) (*Service, error) {
	svc, err := s.GetService(ctx, idOrName)
	if err != nil {
		return nil, err
	}

	changed := false
	if input.Kind != nil {
		svc.Kind = *input.Kind
		changed = true
	}
	if input.Env != nil {
		svc.Env = *input.Env
		changed = true
	}
	if input.Note != nil {
		svc.Note = *input.Note
		changed = true
	}

	if !changed {
		return svc, nil
	}

	svc.UpdatedAt = time.Now()

	if err := s.repo.Update(svc); err != nil {
		s.logSvc.Log(ctx, "service.update", "service", svc.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("update service: %w", err)
	}

	if s.hook != nil {
		if err := s.hook.OnServiceChanged(ctx, svc.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "service", svc.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "service.update", "service", svc.ID, "success",
		fmt.Sprintf("updated service %q", svc.Name), "cli")
	return svc, nil
}
