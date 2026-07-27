package action

import (
	"context"
	"fmt"

	"aegis/internal/apply"
	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/core"
	"aegis/internal/listener"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/space"
)

// ActionResult represents the result of an action execution.
type ActionResult struct {
	OperationID string `json:"operation_id"`
	Status      string `json:"status"` // success | failed
	Message     string `json:"message"`
	Details     string `json:"details,omitempty"`
}

// CallReporter records an inter-service call log.
// Injected after construction to break circular init dependency.
type CallReporter func(ctx context.Context, callerService, targetService, targetAPI string, allowed bool, latencyMs int, errMsg string) error

// ActionService is the unified entry point for all v1.6 actions.
// Both CLI and HTTP API call this same service to ensure consistent behavior.
type ActionService struct {
	serviceSvc   *service.AppService
	routeSvc     *route.AppService
	edgeSvc      *edgemux.AppService
	endpointRepo *endpoint.Repository
	endpointSvc  *endpoint.AppService
	applySvc     *apply.AppService
	spaceRepo    *space.Repository
	logSvc       logs.Logger
	listenerSvc  *listener.Service
	callReporter CallReporter // optional, set via SetCallReporter
}

// NewActionService creates a new ActionService.
func NewActionService(
	serviceSvc *service.AppService,
	routeSvc *route.AppService,
	edgeSvc *edgemux.AppService,
	endpointRepo *endpoint.Repository,
	endpointSvc *endpoint.AppService,
	applySvc *apply.AppService,
	spaceRepo *space.Repository,
	logSvc logs.Logger,
	listenerSvc *listener.Service,
) *ActionService {
	return &ActionService{
		serviceSvc:   serviceSvc,
		routeSvc:     routeSvc,
		edgeSvc:      edgeSvc,
		endpointRepo: endpointRepo,
		endpointSvc:  endpointSvc,
		applySvc:     applySvc,
		spaceRepo:    spaceRepo,
		logSvc:       logSvc,
		listenerSvc:  listenerSvc,
	}
}

// SetCallReporter injects the call reporter after construction.
func (s *ActionService) SetCallReporter(r CallReporter) {
	s.callReporter = r
}

// reportCall writes a call log entry if the caller is a service and a reporter is set.
func (s *ActionService) reportCall(ctx context.Context, ac *ActionContext, api string) {
	if s.callReporter == nil || !ac.IsService() {
		return
	}
	_ = s.callReporter(ctx, ac.SpaceID, "aegis-gateway", api, true, 0, "")
}

// requireSpace validates that the action context has a valid space for space-scoped operations.
// Admin tokens bypass this check.
func (s *ActionService) requireSpace(ctx context.Context) (*ActionContext, error) {
	ac := GetActionContext(ctx)
	if ac == nil {
		return nil, ErrScopeDenied("no action context found")
	}
	if ac.IsAdmin() {
		return ac, nil
	}
	if ac.SpaceID == "" {
		return nil, ErrScopeDenied("space tokens must have a space_id")
	}
	return ac, nil
}

// requireOwnership checks that a resource belongs to the caller's space.
// Admin tokens bypass this check. Resources with empty space_id are system-owned (admin only).
func (s *ActionService) requireOwnership(ctx context.Context, resourceSpaceID, resourceType, resourceID string) error {
	ac := GetActionContext(ctx)
	if ac == nil {
		return ErrScopeDenied("no action context found")
	}
	if ac.IsAdmin() {
		return nil
	}
	// System-owned resources (empty space_id) are admin-only
	if resourceSpaceID == "" {
		return ErrScopeDenied(fmt.Sprintf("system-owned %s %s cannot be modified by space tokens", resourceType, resourceID))
	}
	if resourceSpaceID != ac.SpaceID {
		return ErrResourceNotOwned(resourceType, resourceID, ac.SpaceID)
	}
	return nil
}

// checkDomainOwnership verifies a domain is not already owned by another space.
// Returns the owning space_id if owned, empty string if free.
func (s *ActionService) checkDomainOwnership(domain string) (string, error) {
	// Check routes first
	routes, err := s.routeSvc.ListRoutes(context.Background())
	if err != nil {
		return "", fmt.Errorf("list routes: %w", err)
	}
	for _, rt := range routes {
		if rt.Domain == domain && rt.SpaceID != "" {
			return rt.SpaceID, nil
		}
	}

	// Check edge rules by SNI
	edgeRule, err := s.edgeSvc.FindBySNIHost(context.Background(), domain)
	if err != nil {
		return "", nil
	}
	if edgeRule != nil && edgeRule.SpaceID != "" {
		return edgeRule.SpaceID, nil
	}

	return "", nil
}

// safeApply triggers a safe apply and returns an action error on failure.
func (s *ActionService) safeApply(ctx context.Context) error {
	if s.applySvc == nil {
		// No apply service available (e.g., test mode) — skip apply
		return nil
	}
	_, err := s.applySvc.TryApply(ctx)
	if err != nil {
		if IsActionError(err, ErrCodeApplyLocked) {
			return err
		}
		return NewError(ErrCodeConfigValidateFailed, fmt.Sprintf("apply failed: %v", err))
	}
	return nil
}

// newOperationID generates a unique operation ID.
func newOperationID() string {
	opID := core.NewID("op")
	return fmt.Sprintf("op_%s", opID[3:])
}
