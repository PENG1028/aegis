package action

import (
	"context"
	"fmt"
)

// DisableDomainInput is the input for disabling a domain binding.
type DisableDomainInput struct {
	Domain       string `json:"domain"`
	ResourceType string `json:"resource_type"` // "route" or "edge_rule"
}

// DisableDomain disables a route or edge rule in the current space.
func (s *ActionService) DisableDomain(ctx context.Context, input DisableDomainInput) (*ActionResult, error) {
	opID := newOperationID()

	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	switch input.ResourceType {
	case "route":
		return s.disableRouteDomain(ctx, ac, opID, input.Domain)
	case "edge_rule":
		return s.disableEdgeRuleDomain(ctx, ac, opID, input.Domain)
	default:
		// Try route first, then edge rule
		if result, err := s.disableRouteDomain(ctx, ac, opID, input.Domain); err == nil {
			return result, nil
		}
		return s.disableEdgeRuleDomain(ctx, ac, opID, input.Domain)
	}
}

func (s *ActionService) disableRouteDomain(ctx context.Context, ac *ActionContext, opID, domain string) (*ActionResult, error) {
	rt, err := s.routeSvc.GetRoute(ctx, domain)
	if err != nil {
		return nil, ErrResourceNotFound("route", domain)
	}

	if err := s.requireOwnership(ctx, rt.SpaceID, "route", rt.ID); err != nil {
		return nil, err
	}

	if rt.Status == "disabled" {
		return &ActionResult{
			OperationID: opID,
			Status:      "success",
			Message:     fmt.Sprintf("route for %s is already disabled", domain),
		}, nil
	}

	if err := s.routeSvc.DisableRoute(ctx, rt.ID); err != nil {
		return nil, fmt.Errorf("disable route: %w", err)
	}

	if err := s.safeApply(ctx); err != nil {
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "domain disabled but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.disable-domain", "route", rt.ID, "success",
		fmt.Sprintf("disabled route for domain %s", domain), ac.Actor)

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("disabled route for domain %s", domain),
	}, nil
}

func (s *ActionService) disableEdgeRuleDomain(ctx context.Context, ac *ActionContext, opID, sniHost string) (*ActionResult, error) {
	rule, err := s.edgeSvc.FindBySNIHost(ctx, sniHost)
	if err != nil || rule == nil {
		return nil, ErrResourceNotFound("edge_rule", sniHost)
	}

	if err := s.requireOwnership(ctx, rule.SpaceID, "edge_rule", rule.ID); err != nil {
		return nil, err
	}

	if rule.Status == "disabled" {
		return &ActionResult{
			OperationID: opID,
			Status:      "success",
			Message:     fmt.Sprintf("edge rule for %s is already disabled", sniHost),
		}, nil
	}

	if err := s.edgeSvc.DisableRule(ctx, rule.ID); err != nil {
		return nil, fmt.Errorf("disable edge rule: %w", err)
	}

	if err := s.safeApply(ctx); err != nil {
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "domain disabled but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.disable-domain", "edge_rule", rule.ID, "success",
		fmt.Sprintf("disabled edge rule for SNI %s", sniHost), ac.Actor)

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("disabled edge rule for SNI %s", sniHost),
	}, nil
}
