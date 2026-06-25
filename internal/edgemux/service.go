package edgemux

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// AppService manages edge mux SNI rules.
type AppService struct {
	repo   *Repository
	logSvc *logs.AppService
}

// NewAppService creates a new edge mux application service.
func NewAppService(repo *Repository, logSvc *logs.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateRule creates a new edge mux SNI rule.
func (s *AppService) CreateRule(ctx context.Context, input CreateRuleInput) (*Rule, error) {
	if err := ValidateSNIHost(input.SNIHost); err != nil {
		return nil, err
	}

	existing, err := s.repo.FindBySNIHost(input.SNIHost)
	if err != nil {
		return nil, fmt.Errorf("check duplicate sni_host: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("SNI host %q already exists (rule: %s)", input.SNIHost, existing.ID)
	}

	ok, msg := ValidateTarget(input.TargetHost)
	if !ok {
		return nil, fmt.Errorf("target safety check failed: %s", msg)
	}

	if input.DeclaredKind == "" {
		input.DeclaredKind = KindUnknownTLSBackend
	}

	now := time.Now()
	rule := &Rule{
		ID:           id.New("edge"),
		SNIHost:      input.SNIHost,
		DeclaredKind: input.DeclaredKind,
		TargetHost:   input.TargetHost,
		TargetPort:   input.TargetPort,
		ServiceID:    input.ServiceID,
		ManagedBy:    "manual",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(rule); err != nil {
		s.logSvc.Log(ctx, "edgemux.create", "edge_mux_rule", rule.ID, "failed", err.Error(), "api")
		return nil, err
	}

	s.logSvc.Log(ctx, "edgemux.create", "edge_mux_rule", rule.ID, "success",
		fmt.Sprintf("created edge rule: SNI %s -> %s:%d (%s)", rule.SNIHost, rule.TargetHost, rule.TargetPort, rule.DeclaredKind), "api")
	return rule, nil
}

// EnsureRuleForHTTPRoute creates or updates an edge rule for an HTTP route.
// Called automatically when an HTTP route is created. The edge rule is managed_by=http_route.
func (s *AppService) EnsureRuleForHTTPRoute(ctx context.Context, domain, routeID string) (*Rule, error) {
	if err := ValidateSNIHost(domain); err != nil {
		return nil, fmt.Errorf("invalid HTTP route domain for edge rule: %w", err)
	}

	existing, _ := s.repo.FindBySNIHost(domain)
	if existing != nil {
		// Check ownership — only http_route-managed rules are auto-updated
		if existing.ManagedBy != "http_route" && existing.ManagedBy != "" {
			// manual rule exists — don't auto-override
			return existing, nil
		}
		existing.TargetHost = "127.0.0.1"
		existing.TargetPort = 8443
		existing.DeclaredKind = KindHTTPSApp
		existing.ManagedBy = "http_route"
		existing.SourceRef = routeID
		existing.UpdatedAt = time.Now()
		if err := s.repo.Update(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	now := time.Now()
	rule := &Rule{
		ID:           id.New("edge"),
		SNIHost:      domain,
		DeclaredKind: KindHTTPSApp,
		TargetHost:   "127.0.0.1",
		TargetPort:   8443,
		ManagedBy:    "http_route",
		SourceRef:    routeID,
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(rule); err != nil {
		return nil, err
	}

	s.logSvc.Log(ctx, "edgemux.sync", "edge_mux_rule", rule.ID, "success",
		fmt.Sprintf("auto-created edge rule for HTTP route %s: SNI %s -> 127.0.0.1:8443", routeID, domain), "system")
	return rule, nil
}

// RemoveRuleForHTTPRoute removes the auto-managed edge rule for an HTTP route.
func (s *AppService) RemoveRuleForHTTPRoute(ctx context.Context, routeID string) error {
	rules, err := s.repo.FindBySourceRef(routeID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.ManagedBy == "http_route" {
			if err := s.repo.Delete(rule.ID); err != nil {
				return err
			}
			s.logSvc.Log(ctx, "edgemux.sync", "edge_mux_rule", rule.ID, "success",
				fmt.Sprintf("auto-removed edge rule for HTTP route %s (SNI %s)", routeID, rule.SNIHost), "system")
		}
	}
	return nil
}

// SyncRouteStatus syncs edge rule status with route status.
func (s *AppService) SyncRouteStatus(ctx context.Context, routeID string, routeActive bool) error {
	rules, err := s.repo.FindBySourceRef(routeID)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if rule.ManagedBy == "http_route" {
			if routeActive {
				rule.Status = "active"
			} else {
				rule.Status = "disabled"
			}
			rule.UpdatedAt = time.Now()
			if err := s.repo.Update(&rule); err != nil {
				return err
			}
		}
	}
	return nil
}

// DeleteRule removes an edge rule. Blocks deletion of http_route-managed rules unless force=true.
func (s *AppService) DeleteRule(ctx context.Context, id string, force bool) error {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if rule == nil {
		return fmt.Errorf("edge rule %q not found", id)
	}
	if rule.ManagedBy == "http_route" && !force {
		return fmt.Errorf("edge rule %q is managed by HTTP route (source: %s). Use --force to override, or manage via the route instead.", id, rule.SourceRef)
	}
	return s.repo.Delete(id)
}

func (s *AppService) ListRules(ctx context.Context) ([]Rule, error) {
	rules, err := s.repo.FindAll()
	if err != nil { return nil, err }
	if rules == nil { rules = []Rule{} }
	return rules, nil
}

func (s *AppService) GetRule(ctx context.Context, id string) (*Rule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil { return nil, err }
	if rule == nil { return nil, fmt.Errorf("edge rule %q not found", id) }
	return rule, nil
}

// CreateRuleDirect creates a pre-built edge rule directly via the repository.
// Used by the action service to create rules with ownership fields set.
func (s *AppService) CreateRuleDirect(rule *Rule) error {
	return s.repo.Create(rule)
}

// UpdateRuleDirect updates an edge rule directly via the repository.
// Used by the action service to update rules with ownership fields preserved.
func (s *AppService) UpdateRuleDirect(rule *Rule) error {
	return s.repo.Update(rule)
}

// ListRulesBySpaceID returns all edge rules for a specific space.
func (s *AppService) ListRulesBySpaceID(ctx context.Context, spaceID string) ([]Rule, error) {
	rules, err := s.repo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("list edge rules by space: %w", err)
	}
	if rules == nil {
		rules = []Rule{}
	}
	return rules, nil
}

// FindBySNIHost looks up an edge rule by SNI hostname.
func (s *AppService) FindBySNIHost(ctx context.Context, sniHost string) (*Rule, error) {
	rule, err := s.repo.FindBySNIHost(sniHost)
	if err != nil {
		return nil, err
	}
	return rule, nil
}

func (s *AppService) EnableRule(ctx context.Context, id string) error {
	rule, err := s.repo.FindByID(id)
	if err != nil { return err }
	if rule == nil { return fmt.Errorf("edge rule %q not found", id) }
	rule.Status = "active"
	rule.UpdatedAt = time.Now()
	return s.repo.Update(rule)
}

func (s *AppService) DisableRule(ctx context.Context, id string) error {
	rule, err := s.repo.FindByID(id)
	if err != nil { return err }
	if rule == nil { return fmt.Errorf("edge rule %q not found", id) }
	rule.Status = "disabled"
	rule.UpdatedAt = time.Now()
	return s.repo.Update(rule)
}
