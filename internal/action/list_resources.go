package action

import (
	"context"
	"fmt"

	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/service"
	"aegis/internal/edgemux"
)

// ListMyRoutes returns all routes owned by the caller's space.
func (s *ActionService) ListMyRoutes(ctx context.Context) ([]route.Route, error) {
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	if ac.IsAdmin() {
		return s.routeSvc.ListRoutes(ctx)
	}

	routes, err := s.routeSvc.ListRoutesBySpaceID(ctx, ac.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("list my routes: %w", err)
	}
	return routes, nil
}

// ListMyServices returns all services owned by the caller's space.
func (s *ActionService) ListMyServices(ctx context.Context) ([]service.Service, error) {
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	if ac.IsAdmin() {
		return s.serviceSvc.ListServices(ctx)
	}

	services, err := s.serviceSvc.ListServicesBySpaceID(ctx, ac.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("list my services: %w", err)
	}
	return services, nil
}

// ListMyEdgeRules returns all edge rules owned by the caller's space.
func (s *ActionService) ListMyEdgeRules(ctx context.Context) ([]edgemux.Rule, error) {
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	if ac.IsAdmin() {
		return s.edgeSvc.ListRules(ctx)
	}

	rules, err := s.edgeSvc.ListRulesBySpaceID(ctx, ac.SpaceID)
	if err != nil {
		return nil, fmt.Errorf("list my edge rules: %w", err)
	}
	return rules, nil
}

// ListMyOperations returns recent operation logs for the caller's space.
func (s *ActionService) ListMyOperations(ctx context.Context, limit int) ([]logs.OperationLog, error) {
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 50
	}

	if ac.IsAdmin() {
		ops, err := s.logSvc.ListLogs(ctx, "", "")
		if err != nil {
			return nil, fmt.Errorf("list operations: %w", err)
		}
		if len(ops) > limit {
			ops = ops[:limit]
		}
		return ops, nil
	}

	// Space tokens must only see their own space's operations — the old
	// "action." prefix filter returned every space's action logs (info leak
	// across tenants).
	ops, err := s.logSvc.ListLogsBySpace(ctx, ac.SpaceID, limit)
	if err != nil {
		return nil, fmt.Errorf("list my operations: %w", err)
	}
	return ops, nil
}
