package route

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// AppService defines the route application service interface.
type AppService struct {
	repo   *Repository
	logSvc *logs.AppService
}

// NewAppService creates a new route application service.
func NewAppService(repo *Repository, logSvc *logs.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateRoute creates a new route.
func (s *AppService) CreateRoute(ctx context.Context, input CreateRouteInput) (*Route, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if input.ServiceID == "" {
		return nil, fmt.Errorf("service is required")
	}

	// Check for duplicate domain
	existing, err := s.repo.FindByDomain(input.Domain)
	if err != nil {
		return nil, fmt.Errorf("check duplicate domain: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("route for domain %q already exists", input.Domain)
	}

	now := time.Now()
	rt := &Route{
		ID:                 id.New("rt"),
		Domain:             input.Domain,
		ServiceID:          input.ServiceID,
		TLSEnabled:          true,
		Status:              "active",
		MaintenanceEnabled:  false,
		MaintenanceMessage:  "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	if err := s.repo.Create(rt); err != nil {
		s.logSvc.Log(ctx, "route.create", "route", rt.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("create route: %w", err)
	}

	s.logSvc.Log(ctx, "route.create", "route", rt.ID, "success",
		fmt.Sprintf("created route for domain %q", rt.Domain), "cli")
	return rt, nil
}

// ListRoutes returns all routes.
func (s *AppService) ListRoutes(ctx context.Context) ([]Route, error) {
	routes, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	if routes == nil {
		routes = []Route{}
	}
	return routes, nil
}

// GetRoute finds a route by ID or domain.
func (s *AppService) GetRoute(ctx context.Context, idOrDomain string) (*Route, error) {
	rt, err := s.repo.FindByID(idOrDomain)
	if err != nil {
		return nil, fmt.Errorf("find route: %w", err)
	}
	if rt != nil {
		return rt, nil
	}

	rt, err = s.repo.FindByDomain(idOrDomain)
	if err != nil {
		return nil, fmt.Errorf("find route: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("route %q not found", idOrDomain)
	}
	return rt, nil
}

// EnableRoute enables a route.
func (s *AppService) EnableRoute(ctx context.Context, idOrDomain string) error {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return err
	}

	if rt.Status == "active" {
		return fmt.Errorf("route for %q is already active", rt.Domain)
	}

	rt.Status = "active"
	rt.UpdatedAt = time.Now()

	if err := s.repo.Update(rt); err != nil {
		return fmt.Errorf("enable route: %w", err)
	}

	s.logSvc.Log(ctx, "route.enable", "route", rt.ID, "success",
		fmt.Sprintf("enabled route for %q", rt.Domain), "cli")
	return nil
}

// DisableRoute disables a route.
func (s *AppService) DisableRoute(ctx context.Context, idOrDomain string) error {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return err
	}

	if rt.Status == "disabled" {
		return fmt.Errorf("route for %q is already disabled", rt.Domain)
	}

	rt.Status = "disabled"
	rt.UpdatedAt = time.Now()

	if err := s.repo.Update(rt); err != nil {
		return fmt.Errorf("disable route: %w", err)
	}

	s.logSvc.Log(ctx, "route.disable", "route", rt.ID, "success",
		fmt.Sprintf("disabled route for %q", rt.Domain), "cli")
	return nil
}

// SwitchRoute switches a route to a different service.
func (s *AppService) SwitchRoute(ctx context.Context, idOrDomain string, serviceID string) error {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return err
	}

	oldServiceID := rt.ServiceID
	rt.ServiceID = serviceID
	rt.UpdatedAt = time.Now()

	if err := s.repo.Update(rt); err != nil {
		return fmt.Errorf("switch route: %w", err)
	}

	s.logSvc.Log(ctx, "route.switch", "route", rt.ID, "success",
		fmt.Sprintf("switched route %q from service %q to %q", rt.Domain, oldServiceID, serviceID), "cli")
	return nil
}

// SetMaintenance enables or disables maintenance mode for a route.
func (s *AppService) SetMaintenance(ctx context.Context, idOrDomain string, enabled bool, message string) error {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return err
	}

	rt.MaintenanceEnabled = enabled
	rt.MaintenanceMessage = message
	rt.UpdatedAt = time.Now()

	if err := s.repo.Update(rt); err != nil {
		return fmt.Errorf("set maintenance: %w", err)
	}

	action := "maintenance.off"
	msg := fmt.Sprintf("disabled maintenance for %q", rt.Domain)
	if enabled {
		action = "maintenance.on"
		msg = fmt.Sprintf("enabled maintenance for %q", rt.Domain)
	}

	s.logSvc.Log(ctx, action, "route", rt.ID, "success", msg, "cli")
	return nil
}

// ListMaintenanceStatus returns all routes with their maintenance status.
func (s *AppService) ListMaintenanceStatus(ctx context.Context) ([]Route, error) {
	routes, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list maintenance status: %w", err)
	}
	if routes == nil {
		routes = []Route{}
	}
	return routes, nil
}
