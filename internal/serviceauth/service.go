package serviceauth

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"
)

// ============================================================================
// Dependencies — everything the Service needs from the outside world.
// All fields are interfaces or concrete types defined in this package so
// the core has zero imports of Aegis (or any other project) packages.
// ============================================================================

// NodeChecker decides whether an IP address belongs to the trusted cluster.
type NodeChecker interface {
	// FindByIP returns node info if the IP is recognised, or ErrNotInCluster.
	FindByIP(ip string) (*NodeInfo, error)
}

// NodeInfo is minimal node identity returned by NodeChecker.
type NodeInfo struct {
	NodeID    string
	LocalIP   string
	PrivateIP string
}

// LogRecorder writes inter-service call records.
type LogRecorder interface {
	WriteCallLog(caller, target, api, callerHost, targetHost string, allowed bool, latencyMs int, errMsg string) error
}

// Dependencies holds every external dependency of the Service.
type Dependencies struct {
	Repo        *Repository
	NodeChecker NodeChecker // may be nil — falls back to private-IP check
	LogWriter   LogRecorder // may be nil — logs are silently dropped
	IDGen       func() string
	MasterKey   []byte // deprecated: Ed25519 keypairs replace cluster secret signing
}

// ============================================================================
// Service
// ============================================================================

// Service is the core business-logic layer for serviceauth.
// It is transport-agnostic — HTTP handlers (in serviceauthd or Aegis)
// call its methods.
type Service struct {
	deps Dependencies

	blVersion  atomic.Int64 // monotonic blocklist version
	catVersion atomic.Int64 // monotonic catalog version
}

// NewService creates a Service. No shared secret — Ed25519 keypairs are per-service.
func NewService(deps Dependencies) (*Service, error) {
	if deps.Repo == nil {
		return nil, fmt.Errorf("serviceauth: Repo is required")
	}
	if deps.IDGen == nil {
		deps.IDGen = DefaultIDGen
	}

	svc := &Service{deps: deps}
	if v, err := deps.Repo.GetBlocklistVersion(); err == nil {
		svc.blVersion.Store(v)
	}
	return svc, nil
}

// ============================================================================
// Register — called by a service on startup
// ============================================================================

// Register processes a service registration request.
func (s *Service) Register(ctx context.Context, req RegisterRequest, clientIP string) (*RegisterResponse, error) {
	if err := validateRegisterRequest(req); err != nil {
		return nil, err
	}

	if !s.isInCluster(clientIP) {
		return nil, ErrNotInCluster
	}

	apisJSON, _ := json.Marshal(req.APIs)
	now := time.Now()
	rec := &ServiceRecord{
		ID:        s.deps.IDGen(),
		Name:      req.ServiceName,
		Host:      req.Host,
		Port:      req.Port,
		NodeHost:  req.NodeHost,
		APIsJSON:  string(apisJSON),
		PublicKey: req.PublicKey,
		Status:    "active",
		LastSeen:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.deps.Repo.UpsertService(rec); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}

	// Mark instances that haven't heartbeated in 3 minutes as inactive (alpha).
	if n, _ := s.deps.Repo.MarkStale(time.Now().Add(-3 * time.Minute)); n > 0 {
		s.catVersion.Add(1)
	}

	s.catVersion.Add(1)

	allActive, _ := s.deps.Repo.ListActive()
	instances := make([]ServiceInstance, 0, len(allActive))
	allAPIs := make([]APIDef, 0)
	for _, svc := range allActive {
		instances = append(instances, ServiceInstance{
			Name:     svc.Name,
			Host:     svc.Host,
			Port:     svc.Port,
			NodeHost: svc.NodeHost,
		})
		var apis []APIDef
		if json.Unmarshal([]byte(svc.APIsJSON), &apis) == nil {
			allAPIs = append(allAPIs, apis...)
		}
	}

	publicKeys, _ := s.deps.Repo.ListPublicKeys()
	groups, _ := s.deps.Repo.ListGroups()
	policies, _ := s.deps.Repo.ListPolicies()

	blocklist, _ := s.deps.Repo.GetBlocklist()
	if blocklist == nil {
		blocklist = []BlocklistEntry{}
	}

	return &RegisterResponse{
		ServiceID:    rec.ID,
		Instances:    instances,
		PublicKeys:   publicKeys,
		Groups:       groups,
		Policies:     policies,
		APIs:         allAPIs,
		Blocklist:    blocklist,
		BlVersion:    s.blVersion.Load(),
		CatVersion:   s.catVersion.Load(),
		SyncInterval: 30,
	}, nil
}

// ============================================================================
// Sync — lightweight delta poll
// ============================================================================

// Sync returns changes since the client's last known versions.
func (s *Service) Sync(ctx context.Context, blVersion, catVersion int64) (*SyncResponse, error) {
	currentBL := s.blVersion.Load()
	currentCat := s.catVersion.Load()

	if blVersion >= currentBL && catVersion >= currentCat {
		return &SyncResponse{
			BlVersion:   currentBL,
			CatVersion:  currentCat,
			NotModified: true,
		}, nil
	}

	resp := &SyncResponse{
		BlVersion:  currentBL,
		CatVersion: currentCat,
	}

	if blVersion < currentBL {
		if entries, err := s.deps.Repo.GetBlocklistSince(blVersion); err == nil {
			resp.Blocklist = entries
		}
	}

	if catVersion < currentCat {
		allActive, err := s.deps.Repo.ListActive()
		if err == nil {
			for _, svc := range allActive {
				resp.NewInstances = append(resp.NewInstances, ServiceInstance{
					Name:     svc.Name,
					Host:     svc.Host,
					Port:     svc.Port,
					NodeHost: svc.NodeHost,
				})
			}
		}
		if pks, err := s.deps.Repo.ListPublicKeys(); err == nil {
			resp.PublicKeys = pks
		}
		if groups, err := s.deps.Repo.ListGroups(); err == nil {
			resp.Groups = groups
		}
		if policies, err := s.deps.Repo.ListPolicies(); err == nil {
			resp.Policies = policies
		}
	}

	return resp, nil
}

// ============================================================================
// Report — asynchronous call logging
// ============================================================================

// Report records an inter-service call.
func (s *Service) Report(ctx context.Context, req ReportRequest) error {
	log := &CallLog{
		ID:            s.deps.IDGen(),
		CallerService: req.CallerService,
		TargetService: req.TargetService,
		TargetAPI:     req.TargetAPI,
		CallerHost:    req.CallerHost,
		TargetHost:    req.TargetHost,
		Allowed:       req.Allowed,
		LatencyMs:     req.LatencyMs,
		ErrorMsg:      req.ErrorMsg,
		CalledAt:      time.Now(),
	}

	if err := s.deps.Repo.InsertCallLog(log); err != nil {
		return fmt.Errorf("report: %w", err)
	}

	if s.deps.LogWriter != nil {
		_ = s.deps.LogWriter.WriteCallLog(
			req.CallerService, req.TargetService, req.TargetAPI,
			req.CallerHost, req.TargetHost, req.Allowed, req.LatencyMs, req.ErrorMsg,
		)
	}

	return nil
}

