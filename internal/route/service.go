package route

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/edgemux"
	"aegis/internal/id"
	"aegis/internal/logs"
)

// MutationHook is called after route mutations to trigger desired state regeneration.
type MutationHook interface {
	OnRouteChanged(ctx context.Context, routeID string) error
}

// AppService defines the route application service interface.
type AppService struct {
	repo    *Repository
	logSvc  *logs.AppService
	edgeSvc *edgemux.AppService
	hook    MutationHook
}

// NewAppService creates a new route application service.
func NewAppService(repo *Repository, logSvc *logs.AppService, edgeSvc *edgemux.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc, edgeSvc: edgeSvc}
}

// SetMutationHook sets the mutation hook for desired state regeneration.
func (s *AppService) SetMutationHook(hook MutationHook) {
	s.hook = hook
}

// CreateRoute creates a new route.
func (s *AppService) CreateRoute(ctx context.Context, input CreateRouteInput) (*Route, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if input.ServiceID == "" {
		return nil, fmt.Errorf("service is required")
	}

	// Validate path_prefix
	if err := ValidatePathPrefix(input.PathPrefix); err != nil {
		return nil, fmt.Errorf("invalid path_prefix: %w", err)
	}

	// Check for duplicate domain+path
	if err := s.repo.CheckDuplicatePath(input.Domain, input.PathPrefix, ""); err != nil {
		return nil, err
	}

	now := time.Now()
	rt := &Route{
		ID:                 id.New("rt"),
		Domain:             input.Domain,
		PathPrefix:         input.PathPrefix,
		StripPrefix:        input.StripPrefix,
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

	// Auto-sync edge rule in EdgeMux mode
	if s.edgeSvc != nil {
		if _, err := s.edgeSvc.EnsureRuleForHTTPRoute(ctx, rt.Domain, rt.ID); err != nil {
			// Log but don't fail — edge sync is best-effort on create
			s.logSvc.Log(ctx, "route.edge-sync", "route", rt.ID, "failed",
				fmt.Sprintf("edge rule sync failed: %v", err), "system")
		}
	}

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	return rt, nil
}

// CreateRouteDirect creates a pre-built route directly via the repository.
// Used by the action service to create routes with ownership fields set.
func (s *AppService) CreateRouteDirect(rt *Route) error {
	if err := s.repo.Create(rt); err != nil {
		return err
	}
	if s.hook != nil {
		if err := s.hook.OnRouteChanged(context.Background(), rt.ID); err != nil {
			s.logSvc.Log(context.Background(), "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}
	return nil
}

// ListRoutesBySpaceID returns all routes for a specific space.
func (s *AppService) ListRoutesBySpaceID(ctx context.Context, spaceID string) ([]Route, error) {
	routes, err := s.repo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("list routes by space: %w", err)
	}
	if routes == nil {
		routes = []Route{}
	}
	return routes, nil
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

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "route.enable", "route", rt.ID, "success",
		fmt.Sprintf("enabled route for %q", rt.Domain), "cli")
	if s.edgeSvc != nil {
		s.edgeSvc.SyncRouteStatus(ctx, rt.ID, true)
	}
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

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "route.disable", "route", rt.ID, "success",
		fmt.Sprintf("disabled route for %q", rt.Domain), "cli")
	if s.edgeSvc != nil {
		s.edgeSvc.SyncRouteStatus(ctx, rt.ID, false)
	}
	return nil
}

// DeleteRoute deletes a route and cleans up managed edge rules.
func (s *AppService) DeleteRoute(ctx context.Context, idOrDomain string) error {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return err
	}

	// Clean up managed edge rule
	if s.edgeSvc != nil {
		if err := s.edgeSvc.RemoveRuleForHTTPRoute(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "route.delete.edge-cleanup", "route", rt.ID, "failed",
				fmt.Sprintf("failed to remove edge rule: %v", err), "system")
		}
	}

	if err := s.repo.Delete(rt.ID); err != nil {
		s.logSvc.Log(ctx, "route.delete", "route", rt.ID, "failed", err.Error(), "cli")
		return fmt.Errorf("delete route: %w", err)
	}

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}

	s.logSvc.Log(ctx, "route.delete", "route", rt.ID, "success",
		fmt.Sprintf("deleted route for domain %q", rt.Domain), "cli")
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

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
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

	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
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
