package exposure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"aegis/internal/credential"
	aerrors "aegis/internal/core"
	"aegis/internal/id"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/provider"
	"aegis/internal/tcp"
	"aegis/internal/udp"
)

// AppService defines the exposure application service.
type AppService struct {
	repo        *Repository
	logSvc      logs.Logger
	provReg     *provider.Registry
	listenerSvc *listener.Service
	tcpMgr      *tcp.Manager
	udpMgr      *udp.Manager
	credSvc     *credential.Service
}

// NewAppService creates a new exposure application service.
func NewAppService(repo *Repository, logSvc logs.Logger, provReg *provider.Registry, listenerSvc *listener.Service) *AppService {
	return &AppService{repo: repo, logSvc: logSvc, provReg: provReg, listenerSvc: listenerSvc}
}

// SetTCPManager wires the TCP proxy manager to the exposure service.
// When set, activating a TCP exposure will start a direct port forwarder.
func (s *AppService) SetTCPManager(mgr *tcp.Manager) {
	s.tcpMgr = mgr
}

// SetUDPManager wires the UDP proxy manager to the exposure service.
func (s *AppService) SetUDPManager(mgr *udp.Manager) {
	s.udpMgr = mgr
}

// SetCredentialService wires the credential resolver to the exposure service.
// When set, TCP exposures can reference credential://alias as their target.
func (s *AppService) SetCredentialService(svc *credential.Service) {
	s.credSvc = svc
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
	selectedProvider, provErr := s.provReg.SelectForProtocol(input.Type)

	var provName string
	var status string
	var msg string

	if provErr != nil {
		provName = ""
		status = StatusPending
		msg = fmt.Sprintf("no provider available for protocol %s", input.Type)
	} else if !selectedProvider.State().Running {
		provName = selectedProvider.State().Name
		status = "pending_provider"
		msg = fmt.Sprintf("provider %s is not running", provName)
	} else {
		provName = selectedProvider.State().Name
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
		e.Message = "HTTP exposure active - will generate Caddy route"
	} else if e.Type == TypeTCP && s.tcpMgr != nil {
		// Start a direct TCP port forwarder
		entryHost := e.Host
		if entryHost == "" {
			entryHost = "127.0.0.1"
		}
		targetHost := e.TargetHost
		if targetHost == "" {
			targetHost = "127.0.0.1"
		}
		targetPort := e.TargetPort
		if targetPort == 0 {
			targetPort = e.Port
		}

		// Resolve credential:// alias if target references one
		if strings.HasPrefix(targetHost, "credential://") {
			if s.credSvc == nil {
				e.Status = StatusFailed
				e.Message = "credential resolver not available - master key may be missing"
			} else {
				alias := strings.TrimPrefix(targetHost, "credential://")
				info, credErr := s.credSvc.DecryptAndResolve(ctx, alias)
				if credErr != nil {
					e.Status = StatusFailed
					e.Message = fmt.Sprintf("resolve credential %q: %v", alias, credErr)
				} else {
					targetHost = info.Host
					if info.Port > 0 {
						targetPort = info.Port
					}
					e.Message = fmt.Sprintf("TCP proxy active: %s:%d -> [%s] %s:%d", entryHost, e.Port, alias, targetHost, targetPort)
				}
			}
		}

		if e.Status != StatusFailed {
			if err := s.tcpMgr.StartProxy(e.ID, entryHost, e.Port, targetHost, targetPort); err != nil {
				e.Status = StatusFailed
				e.Message = fmt.Sprintf("TCP proxy start failed: %v", err)
			} else {
				e.Status = StatusActive
				if e.Message == "" {
					e.Message = fmt.Sprintf("TCP proxy active: %s:%d -> %s:%d", entryHost, e.Port, targetHost, targetPort)
				}
			}
		}
	} else if e.Type == TypeUDP && s.udpMgr != nil {
		// Start a direct UDP port forwarder
		entryHost := e.Host
		if entryHost == "" {
			entryHost = "127.0.0.1"
		}
		targetHost := e.TargetHost
		if targetHost == "" {
			targetHost = "127.0.0.1"
		}
		targetPort := e.TargetPort
		if targetPort == 0 {
			targetPort = e.Port
		}

		if err := s.udpMgr.StartProxy(e.ID, entryHost, e.Port, targetHost, targetPort); err != nil {
			e.Status = StatusFailed
			e.Message = fmt.Sprintf("UDP proxy start failed: %v", err)
		} else {
			e.Status = StatusActive
			e.Message = fmt.Sprintf("UDP proxy active: %s:%d -> %s:%d", entryHost, e.Port, targetHost, targetPort)
		}
	} else {
		e.Status = StatusActiveRecorded
		e.Message = fmt.Sprintf("%s exposure recorded: no proxy config generated", e.Type)
	}
	e.UpdatedAt = time.Now()

	if err := s.repo.Update(e); err != nil {
		// Clean up started proxy on DB failure
		if e.Status == StatusActive && e.Type == TypeTCP && s.tcpMgr != nil {
			s.tcpMgr.StopProxy(e.ID)
		}
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

	// Stop TCP/UDP proxy if running
	if e.Type == TypeTCP && s.tcpMgr != nil {
		if err := s.tcpMgr.StopProxy(e.ID); err != nil {
			s.logSvc.Log(ctx, "exposure.disable.tcp", "exposure", e.ID, "warning",
				fmt.Sprintf("stop tcp proxy: %v", err), "api")
		}
	}
	if e.Type == TypeUDP && s.udpMgr != nil {
		if err := s.udpMgr.StopProxy(e.ID); err != nil {
			s.logSvc.Log(ctx, "exposure.disable.udp", "exposure", e.ID, "warning",
				fmt.Sprintf("stop udp proxy: %v", err), "api")
		}
	}

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
