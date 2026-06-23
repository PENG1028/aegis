package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/service"
)

// Checker performs health checks on services via their endpoints.
type Checker struct {
	httpClient  *http.Client
	repo        *Repository
	svcRepo     *service.Repository
	endpointRepo *endpoint.Repository
	logSvc      *logs.AppService
}

// NewChecker creates a new health checker.
func NewChecker(repo *Repository, svcRepo *service.Repository, endpointRepo *endpoint.Repository, logSvc *logs.AppService) *Checker {
	return &Checker{
		httpClient:   &http.Client{Timeout: 3 * time.Second},
		repo:         repo,
		svcRepo:      svcRepo,
		endpointRepo: endpointRepo,
		logSvc:       logSvc,
	}
}

// AppService is the health application service.
type AppService struct {
	checker      *Checker
	repo         *Repository
	svcRepo      *service.Repository
	endpointRepo *endpoint.Repository
	logSvc       *logs.AppService
}

// NewAppService creates a new health application service.
func NewAppService(repo *Repository, svcRepo *service.Repository, endpointRepo *endpoint.Repository, logSvc *logs.AppService) *AppService {
	return &AppService{
		checker:      NewChecker(repo, svcRepo, endpointRepo, logSvc),
		repo:         repo,
		svcRepo:      svcRepo,
		endpointRepo: endpointRepo,
		logSvc:       logSvc,
	}
}

// CheckService performs a health check on a specific service via its endpoints.
func (s *AppService) CheckService(ctx context.Context, svc *service.Service) ([]HealthCheck, error) {
	endpoints, err := s.endpointRepo.FindEnabledByServiceID(svc.ID)
	if err != nil {
		return nil, fmt.Errorf("find endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		h := s.recordCheck(svc.ID, "", StatusUnhealthy, 0, "no endpoints configured")
		return []HealthCheck{*h}, nil
	}

	var results []HealthCheck
	for _, ep := range endpoints {
		h := s.checkEndpoint(ctx, svc.ID, &ep)
		results = append(results, *h)
	}
	return results, nil
}

// CheckAll checks all active services.
func (s *AppService) CheckAll(ctx context.Context) ([]HealthCheck, error) {
	services, err := s.svcRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active services: %w", err)
	}

	var results []HealthCheck
	for _, svc := range services {
		checks, err := s.CheckService(ctx, &svc)
		if err != nil {
			continue
		}
		results = append(results, checks...)
	}
	return results, nil
}

// GetLatestForService returns the latest health check for a service.
func (s *AppService) GetLatestForService(ctx context.Context, serviceID string) (*HealthCheck, error) {
	return s.repo.FindLatestByServiceID(serviceID)
}

// GetLatestForAll returns the latest health check for all services.
func (s *AppService) GetLatestForAll(ctx context.Context) ([]HealthCheck, error) {
	checks, err := s.repo.FindLatestForAll()
	if err != nil {
		return nil, fmt.Errorf("get latest checks: %w", err)
	}
	if checks == nil {
		checks = []HealthCheck{}
	}
	return checks, nil
}

func (s *AppService) checkEndpoint(ctx context.Context, serviceID string, ep *endpoint.Endpoint) *HealthCheck {
	addr := ep.Address
	if addr == "" {
		return s.recordCheck(serviceID, ep.ID, StatusUnhealthy, 0, "empty address")
	}

	start := time.Now()

	// TCP connect check
	host, port, err := parseAddress(addr)
	if err != nil {
		return s.recordCheck(serviceID, ep.ID, StatusUnhealthy, 0, fmt.Sprintf("invalid address: %v", err))
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return s.recordCheck(serviceID, ep.ID, StatusUnhealthy, latency, fmt.Sprintf("TCP connect failed: %v", err))
	}
	conn.Close()

	return s.recordCheck(serviceID, ep.ID, StatusHealthy, latency, "TCP connect OK")
}

func parseAddress(addr string) (host string, port string, err error) {
	// Strip http:// or https:// prefix
	cleaned := addr
	if len(cleaned) > 7 && cleaned[:7] == "http://" {
		cleaned = cleaned[7:]
	} else if len(cleaned) > 8 && cleaned[:8] == "https://" {
		cleaned = cleaned[8:]
	}

	h, p, e := net.SplitHostPort(cleaned)
	if e != nil {
		// No port specified, try common defaults
		if addr[:5] == "https" {
			return cleaned, "443", nil
		}
		return cleaned, "80", nil
	}
	return h, p, nil
}

func (s *AppService) recordCheck(serviceID, endpointID, status string, latency int64, message string) *HealthCheck {
	h := &HealthCheck{
		ID:         id.New("hc"),
		ServiceID:  serviceID,
		EndpointID: endpointID,
		Status:     status,
		LatencyMS:  latency,
		Message:    message,
		CheckedAt:  time.Now(),
	}
	_ = s.repo.Create(h)
	return h
}
