package exposure

import (
	"context"
	"fmt"
	"time"

	aerrors "aegis/internal/errors"
	"aegis/internal/id"
	"aegis/internal/logs"
)

// AppService defines the exposure application service.
type AppService struct {
	repo   *Repository
	logSvc *logs.AppService
}

// NewAppService creates a new exposure application service.
func NewAppService(repo *Repository, logSvc *logs.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateExposure creates a new exposure request.
func (s *AppService) CreateExposure(ctx context.Context, input CreateExposureInput) (*Exposure, error) {
	if input.Type == "" {
		return nil, fmt.Errorf("exposure type is required")
	}
	if input.ServiceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}
	if input.OwnerRef == "" {
		return nil, fmt.Errorf("owner_ref is required")
	}

	// Validate type
	switch input.Type {
	case TypeHTTP, TypeTCP, TypeUDP, TypeTunnel, TypeInternal:
		// valid
	default:
		return nil, fmt.Errorf("invalid exposure type: %s", input.Type)
	}

	if input.Mode == "" {
		input.Mode = ModePrivate
	}

	now := time.Now()
	e := &Exposure{
		ID:        id.New("exp"),
		Type:      input.Type,
		Mode:      input.Mode,
		Host:      input.Host,
		Port:      input.Port,
		Path:      input.Path,
		ServiceID: input.ServiceID,
		NodeID:    input.NodeID,
		OwnerRef:  input.OwnerRef,
		TargetRef: input.TargetRef,
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(e); err != nil {
		s.logSvc.Log(ctx, "exposure.create", "exposure", e.ID, "failed", err.Error(), "api")
		return nil, fmt.Errorf("create exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.create", "exposure", e.ID, "success",
		fmt.Sprintf("created %s exposure %s:%d (owner: %s)", e.Type, e.Host, e.Port, e.OwnerRef), "api")
	return e, nil
}

// ActivateExposure activates a pending exposure.
// HTTP exposures become active (will generate config).
// Non-HTTP exposures become active_recorded (record only, no config).
func (s *AppService) ActivateExposure(ctx context.Context, exposureID string, callerOwnerRef string) (*Exposure, error) {
	e, err := s.repo.FindByID(exposureID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, aerrors.NotFound("exposure not found")
	}

	// Owner check: only the owner (or admin bypass at API layer) can activate
	if callerOwnerRef != "" && e.OwnerRef != callerOwnerRef {
		return nil, aerrors.Forbidden("cannot modify exposure owned by " + e.OwnerRef)
	}

	if e.Status == StatusActive || e.Status == StatusActiveRecorded {
		return nil, fmt.Errorf("exposure is already active")
	}

	if GeneratesConfig(e.Type) {
		e.Status = StatusActive
		e.Message = "HTTP exposure active — will generate Caddy route"
	} else {
		e.Status = StatusActiveRecorded
		e.Message = fmt.Sprintf("%s exposure recorded — no proxy config generated", e.Type)
	}
	e.UpdatedAt = time.Now()

	if err := s.repo.Update(e); err != nil {
		return nil, fmt.Errorf("activate exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.activate", "exposure", e.ID, "success",
		fmt.Sprintf("activated %s exposure %s:%d (status: %s)", e.Type, e.Host, e.Port, e.Status), "api")
	return e, nil
}

// DisableExposure disables an exposure.
func (s *AppService) DisableExposure(ctx context.Context, exposureID string, callerOwnerRef string) (*Exposure, error) {
	e, err := s.repo.FindByID(exposureID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, aerrors.NotFound("exposure not found")
	}

	if callerOwnerRef != "" && e.OwnerRef != callerOwnerRef {
		return nil, aerrors.Forbidden("cannot modify exposure owned by " + e.OwnerRef)
	}

	e.Status = StatusDisabled
	e.UpdatedAt = time.Now()

	if err := s.repo.Update(e); err != nil {
		return nil, fmt.Errorf("disable exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.disable", "exposure", e.ID, "success",
		fmt.Sprintf("disabled exposure %s:%d", e.Host, e.Port), "api")
	return e, nil
}

// UpdateExposure updates exposure fields.
func (s *AppService) UpdateExposure(ctx context.Context, exposureID string, input UpdateExposureInput, callerOwnerRef string) (*Exposure, error) {
	e, err := s.repo.FindByID(exposureID)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, aerrors.NotFound("exposure not found")
	}

	if callerOwnerRef != "" && e.OwnerRef != callerOwnerRef {
		return nil, aerrors.Forbidden("cannot modify exposure owned by " + e.OwnerRef)
	}

	if input.Host != nil {
		e.Host = *input.Host
	}
	if input.Port != nil {
		e.Port = *input.Port
	}
	if input.Path != nil {
		e.Path = *input.Path
	}
	if input.Status != nil {
		e.Status = *input.Status
	}
	if input.Message != nil {
		e.Message = *input.Message
	}
	e.UpdatedAt = time.Now()

	if err := s.repo.Update(e); err != nil {
		return nil, fmt.Errorf("update exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.update", "exposure", e.ID, "success", "updated", "api")
	return e, nil
}

// ListExposures returns all exposures.
func (s *AppService) ListExposures(ctx context.Context) ([]Exposure, error) {
	exposures, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list exposures: %w", err)
	}
	if exposures == nil {
		exposures = []Exposure{}
	}
	return exposures, nil
}

// ListExposuresByOwner returns exposures for an owner.
func (s *AppService) ListExposuresByOwner(ctx context.Context, ownerRef string) ([]Exposure, error) {
	exposures, err := s.repo.FindByOwnerRef(ownerRef)
	if err != nil {
		return nil, fmt.Errorf("list exposures by owner: %w", err)
	}
	if exposures == nil {
		exposures = []Exposure{}
	}
	return exposures, nil
}

// GetExposure returns an exposure by ID.
func (s *AppService) GetExposure(ctx context.Context, id string) (*Exposure, error) {
	e, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, aerrors.NotFound("exposure not found")
	}
	return e, nil
}

// GetStats returns exposure statistics.
func (s *AppService) GetStats(ctx context.Context) (*Stats, error) {
	return s.repo.GetStats()
}
