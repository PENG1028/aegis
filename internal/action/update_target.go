package action

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/edgemux"
)

// UpdateTargetInput is the input for updating a service or edge rule target.
type UpdateTargetInput struct {
	ResourceType string `json:"resource_type"` // "service" or "edge_rule"
	ResourceID   string `json:"resource_id"`
	TargetHost   string `json:"target_host"`
	TargetPort   int    `json:"target_port"`
}

// UpdateTarget updates the target_host/target_port of a service endpoint or edge rule.
func (s *ActionService) UpdateTarget(ctx context.Context, input UpdateTargetInput) (*ActionResult, error) {
	opID := newOperationID()

	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	switch input.ResourceType {
	case "service":
		return s.updateServiceTarget(ctx, ac, opID, input)
	case "edge_rule":
		return s.updateEdgeRuleTarget(ctx, ac, opID, input)
	default:
		return nil, NewError(ErrCodeResourceNotFound, fmt.Sprintf("unknown resource_type: %s (must be 'service' or 'edge_rule')", input.ResourceType))
	}
}

func (s *ActionService) updateServiceTarget(ctx context.Context, ac *ActionContext, opID string, input UpdateTargetInput) (*ActionResult, error) {
	svc, err := s.serviceSvc.GetService(ctx, input.ResourceID)
	if err != nil {
		return nil, ErrResourceNotFound("service", input.ResourceID)
	}

	// Validate ownership
	if err := s.requireOwnership(ctx, svc.SpaceID, "service", svc.ID); err != nil {
		return nil, err
	}

	// Find the service's endpoints and update the first one
	endpoints, err := s.endpointRepo.FindByServiceID(svc.ID)
	if err != nil {
		return nil, fmt.Errorf("find endpoints: %w", err)
	}

	if len(endpoints) == 0 {
		return nil, NewError(ErrCodeResourceNotFound, fmt.Sprintf("no endpoints found for service %s", svc.ID))
	}

	ep := &endpoints[0]
	ep.Address = fmt.Sprintf("%s:%d", input.TargetHost, input.TargetPort)
	ep.UpdatedAt = time.Now()
	if err := s.endpointSvc.UpdateEndpoint(ctx, ep); err != nil {
		return nil, fmt.Errorf("update endpoint: %w", err)
	}

	// Trigger safe apply
	if err := s.safeApply(ctx); err != nil {
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "target updated but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.update-target", "service", svc.ID, "success",
		fmt.Sprintf("updated service target to %s:%d", input.TargetHost, input.TargetPort), ac.Actor)
	s.reportCall(ctx, ac, "update-target")

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("updated service %s target to %s:%d", svc.Name, input.TargetHost, input.TargetPort),
	}, nil
}

func (s *ActionService) updateEdgeRuleTarget(ctx context.Context, ac *ActionContext, opID string, input UpdateTargetInput) (*ActionResult, error) {
	rule, err := s.edgeSvc.GetRule(ctx, input.ResourceID)
	if err != nil {
		return nil, ErrResourceNotFound("edge_rule", input.ResourceID)
	}

	// Validate ownership
	if err := s.requireOwnership(ctx, rule.SpaceID, "edge_rule", rule.ID); err != nil {
		return nil, err
	}

	rule.TargetHost = input.TargetHost
	rule.TargetPort = input.TargetPort
	rule.UpdatedAt = time.Now()
	if err := updateEdgeRuleDirect(ctx, s.edgeSvc, rule); err != nil {
		return nil, fmt.Errorf("update edge rule: %w", err)
	}

	// Trigger safe apply
	if err := s.safeApply(ctx); err != nil {
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "target updated but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.update-target", "edge_rule", rule.ID, "success",
		fmt.Sprintf("updated edge rule target to %s:%d", input.TargetHost, input.TargetPort), ac.Actor)
	s.reportCall(ctx, ac, "update-target")

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("updated edge rule %s target to %s:%d", rule.SNIHost, input.TargetHost, input.TargetPort),
	}, nil
}

// updateEdgeRuleDirect updates an edge rule via the edgemux service.
func updateEdgeRuleDirect(ctx context.Context, edgeSvc *edgemux.AppService, rule *edgemux.Rule) error {
	return edgeSvc.UpdateRuleDirect(rule)
}
