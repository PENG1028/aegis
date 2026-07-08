package action

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/core"
	"aegis/internal/provider"
	"aegis/internal/route"
	"aegis/internal/service"
)

// BindHTTPDomainInput is the input for binding an HTTP domain.
type BindHTTPDomainInput struct {
	Domain        string `json:"domain"`
	TargetHost    string `json:"target_host"`
	TargetPort    int    `json:"target_port"`
	GatewayLinkID string `json:"gateway_link_id,omitempty"`
	CertID        string `json:"cert_id,omitempty"` // certstore ID for custom TLS cert
}

// BindHTTPDomain binds an HTTP domain to a backend target.
// Creates service, endpoint, route, and auto-managed edge rule.
func (s *ActionService) BindHTTPDomain(ctx context.Context, input BindHTTPDomainInput) (*ActionResult, error) {
	opID := newOperationID()

	// 1. Validate space permission
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Check domain not already owned by another space
	ownerSpaceID, err := s.checkDomainOwnership(input.Domain)
	if err != nil {
		return nil, err
	}
	if ownerSpaceID != "" && ownerSpaceID != ac.SpaceID {
		return nil, ErrDomainAlreadyOwned(input.Domain, ownerSpaceID)
	}

	// 3. Validate target
	if input.TargetHost == "" {
		return nil, NewError(ErrCodeTargetNotAllowed, "target_host is required")
	}
	if input.TargetPort <= 0 || input.TargetPort > 65535 {
		return nil, NewError(ErrCodeTargetNotAllowed, fmt.Sprintf("invalid target_port: %d", input.TargetPort))
	}

	spaceID := ac.SpaceID
	ownerType := "admin"
	ownerID := ""
	tokenID := ac.TokenID
	if !ac.IsAdmin() {
		ownerType = "space"
		ownerID = ac.SpaceID
	}

	// 4. Derive settings from composition registry
	compDef := provider.LookupComp(provider.CompHTTPSRoute) // default: HTTPS
	if compDef == nil {
		return nil, fmt.Errorf("composition not found")
	}

	svcName := fmt.Sprintf("%s-%s", compDef.AppProtocol, input.Domain)
	svc := &service.Service{
		ID:               core.NewID("svc"),
		ProjectID:        "",
		Name:             svcName,
		Kind:             compDef.AppProtocol,
		Env:              "prod",
		Status:           "active",
		SpaceID:          spaceID,
		OwnerType:        ownerType,
		OwnerID:          ownerID,
		CreatedByTokenID: tokenID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	// Need to use repo directly since CreateService validates project
	if err := createServiceDirect(ctx, s.serviceSvc, svc); err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}

	// 5. Create endpoint
	_, err = s.endpointSvc.CreateEndpoint(ctx, endpoint.CreateEndpointInput{
		ServiceID: svc.ID,
		Type:      "private",
		Address:   net.JoinHostPort(input.TargetHost, strconv.Itoa(input.TargetPort)),
	})
	if err != nil {
		return nil, fmt.Errorf("create endpoint: %w", err)
	}

	// 6. Create route — fields derived from composition registry
	rt := &route.Route{
		ID:                 core.NewID("rt"),
		Domain:             input.Domain,
		PathPrefix:         "",
		StripPrefix:        false,
		ServiceID:          svc.ID,
		Composition:        string(compDef.Key),
		TLSEnabled:         compDef.TLSMode != "none",
		GatewayLinkID:      input.GatewayLinkID,
		CertID:             certIDPtr(input.CertID),
		Status:             "active",
		SpaceID:            spaceID,
		OwnerType:          ownerType,
		OwnerID:            ownerID,
		CreatedByTokenID:   tokenID,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := createRouteDirect(ctx, s.routeSvc, rt); err != nil {
		return nil, fmt.Errorf("create route: %w", err)
	}

	// 7. Auto-create managed edge rule: SNI domain -> 127.0.0.1:8443
	if _, err := s.edgeSvc.EnsureRuleForHTTPRoute(ctx, rt.Domain, rt.ID); err != nil {
		s.logSvc.Log(ctx, "action.bind-http-domain.edge", "edge_mux_rule", "", "warning",
			fmt.Sprintf("edge rule auto-create warning: %v", err), "system")
	}

	// 8. Set ownership on the auto-created edge rule
	edgeRule, _ := s.edgeSvc.FindBySNIHost(ctx, rt.Domain)
	if edgeRule != nil {
		edgeRule.SpaceID = spaceID
		edgeRule.OwnerType = ownerType
		edgeRule.OwnerID = ownerID
		edgeRule.CreatedByTokenID = tokenID
		// Update ownership via repo — need the edge repo directly.
		// For now, ownership is set via route lifecycle sync.
		_ = edgeRule
	}

	// 9. Trigger safe apply
	if err := s.safeApply(ctx); err != nil {
		s.logSvc.Log(ctx, "action.bind-http-domain", "action", opID, "failed",
			fmt.Sprintf("apply failed: %v", err), ac.Actor)
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "domain bound but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.bind-http-domain", "action", opID, "success",
		fmt.Sprintf("bound HTTP domain %s -> %s:%d", input.Domain, input.TargetHost, input.TargetPort), ac.Actor)
	s.reportCall(ctx, ac, "bind-http-domain")

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("bound HTTP domain %s -> %s:%d", input.Domain, input.TargetHost, input.TargetPort),
		Details:     fmt.Sprintf("service_id=%s route_id=%s", svc.ID, rt.ID),
	}, nil
}

// createServiceDirect creates a service directly via repo (bypasses project validation).
func createServiceDirect(ctx context.Context, svcSvc *service.AppService, s *service.Service) error {
	// Use reflection-like approach: we need low-level repo access.
	// For now, we export this helper for the action service.
	return svcSvc.CreateServiceDirect(s)
}

// createRouteDirect creates a route directly via repo (bypasses duplicate path check with ownership).
func createRouteDirect(ctx context.Context, rtSvc *route.AppService, rt *route.Route) error {
	return rtSvc.CreateRouteDirect(rt)
}

func certIDPtr(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
