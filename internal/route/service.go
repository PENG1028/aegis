package route

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"aegis/internal/core"
	"aegis/internal/edgemux"
	"aegis/internal/logs"
)

// MutationHook is called after route mutations to trigger desired state regeneration.
type MutationHook interface {
	OnRouteChanged(ctx context.Context, routeID string) error
}

// AppService defines the route application service interface.
type AppService struct {
	repo    *Repository
	logSvc  logs.Logger
	edgeSvc *edgemux.AppService
	hook    MutationHook
}

// NewAppService creates a new route application service.
func NewAppService(repo *Repository, logSvc logs.Logger, edgeSvc *edgemux.AppService) *AppService {
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

	comp := input.Composition
	if comp == "" {
		comp = "https_route" // default: HTTPS with TLS termination
	}
	compDef := (&Route{Composition: comp}).CompDef()
	if compDef == nil {
		return nil, fmt.Errorf("unknown route composition %q", comp)
	}

	now := time.Now()
	rt := &Route{
		ID:                 core.NewID("rt"),
		Domain:             input.Domain,
		PathPrefix:         input.PathPrefix,
		StripPrefix:        input.StripPrefix,
		ServiceID:          input.ServiceID,
		TLSEnabled:         compDef.TLSMode == "terminate",
		Composition:        comp,
		Status:             "active",
		MaintenanceEnabled: false,
		MaintenanceMessage: "",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	normalizeRouteManagement(rt, "caddy")

	if err := s.repo.Create(rt); err != nil {
		s.logSvc.Log(ctx, "route.create", "route", rt.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("create route: %w", err)
	}

	s.logSvc.Log(ctx, "route.create", "route", rt.ID, "success",
		fmt.Sprintf("created route for domain %q", rt.Domain), "cli")

	// Auto-sync edge rule in EdgeMux mode
	if s.edgeSvc != nil {
		if _, err := s.edgeSvc.EnsureRuleForHTTPRoute(ctx, rt.Domain, rt.ID); err != nil {
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
	normalizeRouteManagement(rt, "caddy")
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

// UpsertSystemRoute ensures a route exists for the panel's own domain.
// If tlsAvailable is false (no email, no custom cert), the route is created
// as plain HTTP so Caddy does not attempt auto-TLS and block IP access.
func (s *AppService) UpsertSystemRoute(ctx context.Context, domain string, tlsAvailable bool) error {
	existing, err := s.repo.FindByDomain(domain)
	if err != nil {
		return fmt.Errorf("look up system route %q: %w", domain, err)
	}
	if existing != nil {
		if existing.TLSEnabled != tlsAvailable || existing.Composition != compositionForTLS(tlsAvailable) {
			existing.TLSEnabled = tlsAvailable
			existing.Composition = compositionForTLS(tlsAvailable)
			normalizeRouteManagement(existing, "caddy")
			existing.UpdatedAt = time.Now()
			return s.repo.Update(existing)
		}
		return nil
	}
	now := time.Now()
	rt := &Route{
		ID:          core.NewID("rt"),
		Domain:      domain,
		ServiceID:   "__panel",
		Composition: compositionForTLS(tlsAvailable),
		TLSEnabled:  tlsAvailable,
		Status:      "active",
		OwnerType:   "system",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	normalizeRouteManagement(rt, "caddy")
	return s.repo.Create(rt)
}

func normalizeRouteManagement(rt *Route, defaultProvider string) {
	rt.NormalizeManagement(defaultProvider)
	capsJSON, _ := json.Marshal(rt.CapabilityKeys())
	rt.SourceCapabilities = string(capsJSON)
}

func compositionForTLS(tlsAvailable bool) string {
	if tlsAvailable {
		return "https_route"
	}
	return "http_route"
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

// FindRoutesByCertID returns all routes that reference a given certificate.
func (s *AppService) FindRoutesByCertID(ctx context.Context, certID string) ([]Route, error) {
	return s.repo.FindByCertID(certID)
}

// FindRoutesByFlowBridgeID returns all routes that reference a flowbridge instance.
func (s *AppService) FindRoutesByFlowBridgeID(ctx context.Context, flowbridgeID string) ([]Route, error) {
	return s.repo.FindByFlowBridgeID(flowbridgeID)
}

// SetTLSBinding changes how a TLS-terminating route obtains its certificate.
func (s *AppService) SetTLSBinding(ctx context.Context, idOrDomain, mode, providerID, certID string) (*Route, error) {
	rt, err := s.GetRoute(ctx, idOrDomain)
	if err != nil {
		return nil, err
	}
	def := rt.CompDef()
	if def == nil || def.TLSMode != "terminate" {
		return nil, fmt.Errorf("route %q does not terminate TLS", rt.Domain)
	}

	switch mode {
	case TLSBindingProviderAuto:
		rt.CertID = nil
		rt.TLSBindingMode = TLSBindingProviderAuto
		rt.TLSProvider = providerID
	case TLSBindingCertificate:
		if certID == "" {
			return nil, fmt.Errorf("certificate ID is required")
		}
		rt.CertID = &certID
		rt.TLSBindingMode = TLSBindingCertificate
		rt.TLSProvider = ""
	default:
		return nil, fmt.Errorf("invalid TLS binding mode %q", mode)
	}

	normalizeRouteManagement(rt, rt.SourceProvider)
	rt.UpdatedAt = time.Now()
	if err := s.repo.Update(rt); err != nil {
		return nil, fmt.Errorf("update TLS binding: %w", err)
	}
	if s.hook != nil {
		if err := s.hook.OnRouteChanged(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "desired-state.regen", "route", rt.ID, "warning", "desired state regeneration failed: "+err.Error(), "system")
		}
	}
	return rt, nil
}

// SetCertificateBindings atomically binds one portable certificate asset to a
// validated set of routes. Validation belongs to tlslifecycle; this method owns
// persistence and desired-state notifications.
func (s *AppService) SetCertificateBindings(ctx context.Context, routeIDs []string, certID string) error {
	if err := s.repo.SetCertificateBindings(routeIDs, certID, time.Now()); err != nil {
		return err
	}
	if s.hook != nil {
		for _, routeID := range routeIDs {
			if err := s.hook.OnRouteChanged(ctx, routeID); err != nil {
				s.logSvc.Log(ctx, "desired-state.regen", "route", routeID, "warning", "desired state regeneration failed: "+err.Error(), "system")
			}
		}
	}
	return nil
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

// DeleteAllSystemRoutes removes all routes belonging to the __panel service.
// Used when the panel domain is cleared to prevent stale TLS routes from being re-applied.
func (s *AppService) DeleteAllSystemRoutes(ctx context.Context) error {
	routes, err := s.repo.FindByServiceID("__panel")
	if err != nil {
		return fmt.Errorf("find panel routes: %w", err)
	}
	for _, rt := range routes {
		if err := s.DeleteRoute(ctx, rt.ID); err != nil {
			s.logSvc.Log(ctx, "route.delete.system", "route", rt.ID, "warning",
				fmt.Sprintf("failed to delete panel route: %v", err), "system")
		}
	}
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
