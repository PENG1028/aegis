package serviceauth

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// ─── Dependencies ─────────────────────────────────────────────────────────

type NodeChecker interface {
	FindByIP(ip string) (*NodeInfo, error)
}

type NodeInfo struct {
	NodeID    string
	LocalIP   string
	PrivateIP string
}

type LogRecorder interface {
	WriteCallLog(caller, target, api, callerHost, targetHost string, allowed bool, latencyMs int, errMsg string) error
}

type Dependencies struct {
	Repo        *Repository
	NodeChecker NodeChecker
	LogWriter   LogRecorder
	IDGen       func() string
	MasterKey   []byte
}

// ─── Service ──────────────────────────────────────────────────────────────

type Service struct {
	deps      Dependencies
	blVersion atomic.Int64
}

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

// ─── Register ─────────────────────────────────────────────────────────────

func (s *Service) Register(ctx context.Context, req RegisterRequest, clientIP string) (*RegisterResponse, error) {
	if err := validateRegisterRequest(req); err != nil {
		return nil, err
	}
	if !s.isInCluster(clientIP) {
		return nil, ErrNotInCluster
	}

	now := time.Now()
	rec := &ServiceRecord{
		ID:         s.deps.IDGen(),
		Name:       req.ServiceName,
		PublicKey:  req.PublicKey,
		InstanceID: req.InstanceID,
		Status:     "active",
		LastSeen:   now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.deps.Repo.UpsertService(rec); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}

	var warnings []string
	existing, _ := s.deps.Repo.FindByName(req.ServiceName)
	for _, e := range existing {
		if e.PublicKey != req.PublicKey && e.Status == "active" {
			warnings = append(warnings,
				fmt.Sprintf("服务 %s 已有不同的公钥注册（可能是密钥轮换或多实例，请确认）", req.ServiceName))
			break
		}
	}
	keyUsers, _ := s.deps.Repo.FindByPublicKey(req.PublicKey)
	for _, u := range keyUsers {
		if u.Name != req.ServiceName {
			warnings = append(warnings,
				fmt.Sprintf("此公钥已在服务 %s 上使用，两个服务共享同一私钥", u.Name))
		}
	}

	publicKeys, _ := s.deps.Repo.ListPublicKeys()
	blocklist, _ := s.deps.Repo.GetBlocklist()
	if blocklist == nil {
		blocklist = []BlocklistEntry{}
	}

	return &RegisterResponse{
		ServiceID:    rec.ID,
		PublicKeys:   publicKeys,
		Blocklist:    blocklist,
		BlVersion:    s.blVersion.Load(),
		SyncInterval: 30,
		Warnings:     warnings,
	}, nil
}

// ─── Sync ─────────────────────────────────────────────────────────────────

func (s *Service) Sync(ctx context.Context, blVersion int64, _ int64) (*SyncResponse, error) {
	// catVersion (2nd param) was for the removed groups/policies — kept for backward compat.
	currentBL := s.blVersion.Load()

	if blVersion >= currentBL {
		return &SyncResponse{BlVersion: currentBL, NotModified: true}, nil
	}

	resp := &SyncResponse{BlVersion: currentBL}
	if entries, err := s.deps.Repo.GetBlocklistSince(blVersion); err == nil {
		resp.Blocklist = entries
	}
	if pks, err := s.deps.Repo.ListPublicKeys(); err == nil {
		resp.PublicKeys = pks
	}
	return resp, nil
}

// ─── Heartbeat ────────────────────────────────────────────────────────────

func (s *Service) Heartbeat(ctx context.Context, name, instanceID string) error {
	if name == "" || instanceID == "" {
		return fmt.Errorf("%w: name and instance_id are required", ErrInvalidInput)
	}
	return s.deps.Repo.Heartbeat(name, instanceID, time.Now())
}

func (s *Service) CountOnline(ctx context.Context, window time.Duration) (map[string]int, error) {
	if window <= 0 {
		window = 2 * time.Minute
	}
	return s.deps.Repo.CountOnlineByService(time.Now().Add(-window))
}

// ─── Report ───────────────────────────────────────────────────────────────

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

// ─── Caller / Dep queries (per-service scope) ─────────────────────────────

func (s *Service) CallersOf(ctx context.Context, name string, window time.Duration) ([]TopologyEdge, error) {
	if window <= 0 {
		window = 1 * time.Hour
	}
	return s.deps.Repo.CallersOf(name, time.Now().Add(-window))
}

func (s *Service) DepsOf(ctx context.Context, name string, window time.Duration) ([]TopologyEdge, error) {
	if window <= 0 {
		window = 1 * time.Hour
	}
	return s.deps.Repo.DepsOf(name, time.Now().Add(-window))
}
