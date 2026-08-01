package flowbridge

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/core"
	"aegis/internal/logs"
)

// Service handles flowbridge instance business logic.
type Service struct {
	repo   *Repository
	logSvc logs.Logger
	probe  Probe
}

// Probe performs a single health probe against an instance's control plane.
type Probe interface {
	Check(ctx context.Context, inst *Instance) (status string, latencyMS int64, message string)
}

// NewService creates a flowbridge instance service.
func NewService(repo *Repository, logSvc logs.Logger) *Service {
	return &Service{repo: repo, logSvc: logSvc, probe: NewHTTPProbe(0)}
}

// log writes an audit entry; no-ops when no logger is configured (tests).
func (s *Service) log(ctx context.Context, action, resource, id, status, message, actor string) {
	if s.logSvc == nil {
		return
	}
	s.log(ctx, action, resource, id, status, message, actor)
}

// SetProbe overrides the health probe (used by tests).
func (s *Service) SetProbe(p Probe) {
	s.probe = p
}

// Create creates a new instance and runs an immediate health check.
func (s *Service) Create(ctx context.Context, input CreateInstanceInput, spaceID, ownerType, ownerID, tokenID string) (*Instance, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	now := time.Now()
	inst := &Instance{
		ID:               core.NewID("fb"),
		Name:             input.Name,
		MachineIP:        input.MachineIP,
		DataPlanePort:    input.DataPlanePort,
		ControlAddress:   input.ControlAddress,
		Enabled:          true,
		LastHealthStatus: HealthUnknown,
		SpaceID:          spaceID,
		OwnerType:        ownerType,
		OwnerID:          ownerID,
		CreatedByTokenID: tokenID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.Create(inst); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	s.log(ctx, "flowbridge.create", "flowbridge_instance", inst.ID, "success",
		fmt.Sprintf("created flowbridge instance %q (%s:%d)", inst.Name, inst.MachineIP, inst.DataPlanePort), "api")
	return s.Check(ctx, inst.ID)
}

// List returns all instances.
func (s *Service) List(ctx context.Context) ([]Instance, error) {
	return s.repo.FindAll()
}

// Get returns an instance by ID.
func (s *Service) Get(ctx context.Context, id string) (*Instance, error) {
	inst, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if inst == nil {
		return nil, fmt.Errorf("flowbridge instance %q not found", id)
	}
	return inst, nil
}

// Update applies a partial update and returns the updated instance.
func (s *Service) Update(ctx context.Context, id string, input UpdateInstanceInput) (*Instance, error) {
	inst, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		if *input.Name == "" {
			return nil, fmt.Errorf("name is required")
		}
		if len(*input.Name) > 100 {
			return nil, fmt.Errorf("name must be at most 100 characters")
		}
		inst.Name = *input.Name
	}
	if input.MachineIP != nil {
		inst.MachineIP = *input.MachineIP
	}
	if input.DataPlanePort != nil {
		inst.DataPlanePort = *input.DataPlanePort
	}
	if input.ControlAddress != nil {
		inst.ControlAddress = *input.ControlAddress
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if input.Enabled != nil {
		inst.Enabled = *input.Enabled
	}
	inst.UpdatedAt = time.Now()
	if err := s.repo.Update(inst); err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}
	s.log(ctx, "flowbridge.update", "flowbridge_instance", inst.ID, "success",
		fmt.Sprintf("updated flowbridge instance %q", inst.Name), "api")
	return inst, nil
}

// SetEnabled enables or disables an instance.
func (s *Service) SetEnabled(ctx context.Context, id string, enabled bool) (*Instance, error) {
	inst, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if inst.Enabled == enabled {
		return inst, nil
	}
	inst.Enabled = enabled
	inst.UpdatedAt = time.Now()
	if err := s.repo.Update(inst); err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}
	verb := "disabled"
	if enabled {
		verb = "enabled"
	}
	s.log(ctx, "flowbridge."+verb, "flowbridge_instance", inst.ID, "success",
		fmt.Sprintf("%s flowbridge instance %q", verb, inst.Name), "api")
	return inst, nil
}

// Delete removes an instance. Routes referencing it block deletion.
func (s *Service) Delete(ctx context.Context, id string) error {
	inst, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		if err == ErrReferenced {
			return fmt.Errorf("flowbridge instance %q is referenced by routes; unbind them first", inst.Name)
		}
		return fmt.Errorf("delete instance: %w", err)
	}
	s.log(ctx, "flowbridge.delete", "flowbridge_instance", id, "success",
		fmt.Sprintf("deleted flowbridge instance %q", inst.Name), "api")
	return nil
}

// Check runs a live health probe and persists the result.
func (s *Service) Check(ctx context.Context, id string) (*Instance, error) {
	inst, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	status, latency, message := s.probe.Check(ctx, inst)
	inst.LastHealthStatus = status
	inst.LastHealthLatency = latency
	inst.LastHealthMessage = message
	inst.LastCheckedAt = time.Now()
	if err := s.repo.Update(inst); err != nil {
		return nil, fmt.Errorf("persist health: %w", err)
	}
	return inst, nil
}
