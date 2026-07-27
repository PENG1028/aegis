package action

import (
	"context"
	"fmt"
)

// DeleteDomainInput is the input for deleting a domain binding.
type DeleteDomainInput struct {
	Domain string `json:"domain"`
}

// DeleteDomain deletes a domain binding and cleans up managed edge rules.
func (s *ActionService) DeleteDomain(ctx context.Context, input DeleteDomainInput) (*ActionResult, error) {
	opID := newOperationID()

	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	// Try route first
	rt, err := s.routeSvc.GetRoute(ctx, input.Domain)
	if err == nil && rt != nil {
		if err := s.requireOwnership(ctx, rt.SpaceID, "route", rt.ID); err != nil {
			return nil, err
		}

		if err := s.routeSvc.DeleteRoute(ctx, rt.ID); err != nil {
			return nil, fmt.Errorf("delete route: %w", err)
		}

		if err := s.safeApply(ctx); err != nil {
			return &ActionResult{
				OperationID: opID,
				Status:      "failed",
				Message:     "domain deleted but apply failed",
				Details:     err.Error(),
			}, nil
		}

		s.logSvc.Log(ctx, "action.delete-domain", "route", rt.ID, "success",
			fmt.Sprintf("deleted route for domain %s", input.Domain), ac.Actor)
		s.reportCall(ctx, ac, "delete-domain")

		return &ActionResult{
			OperationID: opID,
			Status:      "success",
			Message:     fmt.Sprintf("deleted route for domain %s", input.Domain),
		}, nil
	}

	// Try edge rule
	rule, err := s.edgeSvc.FindBySNIHost(ctx, input.Domain)
	if err == nil && rule != nil {
		if err := s.requireOwnership(ctx, rule.SpaceID, "edge_rule", rule.ID); err != nil {
			return nil, err
		}

		// Force delete managed rules too
		if err := s.edgeSvc.DeleteRule(ctx, rule.ID, true); err != nil {
			return nil, fmt.Errorf("delete edge rule: %w", err)
		}

		if err := s.safeApply(ctx); err != nil {
			return &ActionResult{
				OperationID: opID,
				Status:      "failed",
				Message:     "domain deleted but apply failed",
				Details:     err.Error(),
			}, nil
		}

		s.logSvc.Log(ctx, "action.delete-domain", "edge_rule", rule.ID, "success",
			fmt.Sprintf("deleted edge rule for SNI %s", input.Domain), ac.Actor)
		s.reportCall(ctx, ac, "delete-domain")

		return &ActionResult{
			OperationID: opID,
			Status:      "success",
			Message:     fmt.Sprintf("deleted edge rule for SNI %s", input.Domain),
		}, nil
	}

	return nil, ErrResourceNotFound("domain", input.Domain)
}
