package action

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/edgemux"
	"aegis/internal/id"
)

// BindTLSBackendInput is the input for binding a TLS backend.
type BindTLSBackendInput struct {
	SNIHost    string `json:"sni_host"`
	TargetHost string `json:"target_host"`
	TargetPort int    `json:"target_port"`
	Kind       string `json:"kind,omitempty"` // optional, defaults to unknown_tls_backend
}

// BindTLSBackend binds a TLS SNI host to a backend target.
// Creates a manual edge mux rule only — no Caddy route.
func (s *ActionService) BindTLSBackend(ctx context.Context, input BindTLSBackendInput) (*ActionResult, error) {
	opID := newOperationID()

	// 1. Validate space permission
	ac, err := s.requireSpace(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Validate SNI host
	if err := edgemux.ValidateSNIHost(input.SNIHost); err != nil {
		return nil, NewError(ErrCodeTargetNotAllowed, fmt.Sprintf("invalid sni_host: %v", err))
	}

	// 3. Check sni_host not already owned by another space
	ownerSpaceID, err := s.checkDomainOwnership(input.SNIHost)
	if err != nil {
		return nil, err
	}
	if ownerSpaceID != "" && ownerSpaceID != ac.SpaceID {
		return nil, ErrDomainAlreadyOwned(input.SNIHost, ownerSpaceID)
	}

	// 4. Validate target
	ok, msg := edgemux.ValidateTarget(input.TargetHost)
	if !ok {
		return nil, NewError(ErrCodeTargetNotAllowed, msg)
	}
	if input.TargetPort <= 0 || input.TargetPort > 65535 {
		return nil, NewError(ErrCodeTargetNotAllowed, fmt.Sprintf("invalid target_port: %d", input.TargetPort))
	}

	kind := input.Kind
	if kind == "" {
		kind = edgemux.KindUnknownTLSBackend
	}

	spaceID := ac.SpaceID
	ownerType := "admin"
	ownerID := ""
	tokenID := ac.TokenID
	if !ac.IsAdmin() {
		ownerType = "space"
		ownerID = ac.SpaceID
	}

	// 5. Create edge mux rule (managed_by=manual)
	now := time.Now()
	rule := &edgemux.Rule{
		ID:               id.New("edge"),
		SNIHost:          input.SNIHost,
		DeclaredKind:     kind,
		TargetHost:       input.TargetHost,
		TargetPort:       input.TargetPort,
		ManagedBy:        "manual",
		Status:           "active",
		SpaceID:          spaceID,
		OwnerType:        ownerType,
		OwnerID:          ownerID,
		CreatedByTokenID: tokenID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := createEdgeRuleDirect(ctx, s.edgeSvc, rule); err != nil {
		return nil, fmt.Errorf("create edge rule: %w", err)
	}

	// 6. Trigger safe apply
	if err := s.safeApply(ctx); err != nil {
		s.logSvc.Log(ctx, "action.bind-tls-backend", "action", opID, "failed",
			fmt.Sprintf("apply failed: %v", err), ac.Actor)
		return &ActionResult{
			OperationID: opID,
			Status:      "failed",
			Message:     "TLS backend bound but apply failed",
			Details:     err.Error(),
		}, nil
	}

	s.logSvc.Log(ctx, "action.bind-tls-backend", "action", opID, "success",
		fmt.Sprintf("bound TLS backend %s -> %s:%d", input.SNIHost, input.TargetHost, input.TargetPort), ac.Actor)

	return &ActionResult{
		OperationID: opID,
		Status:      "success",
		Message:     fmt.Sprintf("bound TLS backend %s -> %s:%d", input.SNIHost, input.TargetHost, input.TargetPort),
		Details:     fmt.Sprintf("edge_rule_id=%s", rule.ID),
	}, nil
}

// createEdgeRuleDirect creates an edge rule directly via repo.
func createEdgeRuleDirect(ctx context.Context, edgeSvc *edgemux.AppService, rule *edgemux.Rule) error {
	return edgeSvc.CreateRuleDirect(rule)
}
