package exposure

import (
	"context"
	"fmt"
	"time"

	aerrors "aegis/internal/errors"
	"aegis/internal/id"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/provider"
)

// AppService defines the exposure application service.
type AppService struct {
	repo      *Repository
	logSvc    *logs.AppService
	provReg   *provider.Registry
	listenerSvc *listener.Service
}

// NewAppService creates a new exposure application service.
func NewAppService(repo *Repository, logSvc *logs.AppService, provReg *provider.Registry, listenerSvc *listener.Service) *AppService {
	return &AppService{repo: repo, logSvc: logSvc, provReg: provReg, listenerSvc: listenerSvc}
}

// CreateExposure creates a new exposure with provider auto-selection and listener conflict check.
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

	switch input.Type {
	case TypeHTTP, TypeTCP, TypeUDP, TypeTunnel, TypeInternal:
	default:
		return nil, fmt.Errorf("invalid exposure type: %s", input.Type)
	}

	if input.Mode == "" {
		input.Mode = ModePrivate
	}

	// Check listener conflict FIRST (regardless of provider availability)
	if input.Host != "" && input.Port > 0 {
		if err := s.listenerSvc.CheckConflict("", input.Type, input.Host, input.Port); err != nil {
			return nil, err // LISTENER_CONFLICT
		}
	}

	// Auto-select provider
	selectedProvider, provOk := s.provReg.SelectForProtocol(input.Type)

	var provName string
	var status string
	var msg string

	if !provOk {
		provName = ""
		status = StatusPending
		msg = fmt.Sprintf("no provider available for protocol %s", input.Type)
	} else if selectedProvider.Info().Status == "unavailable" {
		provName = selectedProvider.Info().Name
		status = "pending_provider"
		msg = fmt.Sprintf("provider %s is unavailable: %s", provName, selectedProvider.Info().Message)
	} else {
		provName = selectedProvider.Info().Name
		status = StatusPending
	}

	now := time.Now()
	e := &Exposure{
		ID:             id.New("exp"),
		Type:           input.Type,
		Mode:           input.Mode,
		Host:           input.Host,
		Port:           input.Port,
		Path:           input.Path,
		TargetHost:     input.TargetHost,
		TargetPort:     input.TargetPort,
		ServiceID:      input.ServiceID,
		NodeID:         input.NodeID,
		OwnerRef:       input.OwnerRef,
		TargetRef:      input.TargetRef,
		AllowPublicTCP: input.AllowPublicTCP,
		Provider:       provName,
		Status:         status,
		Message:        msg,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.repo.Create(e); err != nil {
		s.logSvc.Log(ctx, "exposure.create", "exposure", e.ID, "failed", err.Error(), "api")
		return nil, fmt.Errorf("create exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.create", "exposure", e.ID, "success",
		fmt.Sprintf("created %s exposure %s:%d (provider: %s, owner: %s)", e.Type, e.Host, e.Port, provName, e.OwnerRef), "api")
	return e, nil
}

// ActivateExposure activates a pending exposure.
func (s *AppService) ActivateExposure(ctx context.Context, exposureID string, callerOwnerRef string) (*Exposure, error) {
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

	if e.Status == StatusActive || e.Status == StatusActiveRecorded {
		return nil, fmt.Errorf("exposure is already active")
	}

	if e.Status == "pending_provider" {
		return nil, fmt.Errorf("cannot activate exposure: provider %s is unavailable", e.Provider)
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

	if input.Host != nil { e.Host = *input.Host }
	if input.Port != nil { e.Port = *input.Port }
	if input.Path != nil { e.Path = *input.Path }
	if input.Status != nil { e.Status = *input.Status }
	if input.Message != nil { e.Message = *input.Message }
	e.UpdatedAt = time.Now()

	if err := s.repo.Update(e); err != nil {
		return nil, fmt.Errorf("update exposure: %w", err)
	}

	s.logSvc.Log(ctx, "exposure.update", "exposure", e.ID, "success", "updated", "api")
	return e, nil
}

func (s *AppService) ListExposures(ctx context.Context) ([]Exposure, error) {
	exposures, err := s.repo.FindAll()
	if err != nil { return nil, err }
	if exposures == nil { exposures = []Exposure{} }
	return exposures, nil
}

func (s *AppService) ListExposuresByOwner(ctx context.Context, ownerRef string) ([]Exposure, error) {
	exposures, err := s.repo.FindByOwnerRef(ownerRef)
	if err != nil { return nil, err }
	if exposures == nil { exposures = []Exposure{} }
	return exposures, nil
}

func (s *AppService) GetExposure(ctx context.Context, id string) (*Exposure, error) {
	e, err := s.repo.FindByID(id)
	if err != nil { return nil, err }
	if e == nil { return nil, aerrors.NotFound("exposure not found") }
	return e, nil
}

func (s *AppService) GetStats(ctx context.Context) (*Stats, error) {
	return s.repo.GetStats()
}
