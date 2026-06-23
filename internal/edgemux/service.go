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

	// Check duplicate
	existing, err := s.repo.FindBySNIHost(input.SNIHost)
	if err != nil {
		return nil, fmt.Errorf("check duplicate sni_host: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("SNI host %q already exists (rule: %s)", input.SNIHost, existing.ID)
	}

	// Validate target safety
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
// In EdgeMux mode, every HTTP route needs a corresponding SNI rule pointing to Caddy internal 8443.
func (s *AppService) EnsureRuleForHTTPRoute(ctx context.Context, domain string) (*Rule, error) {
	if err := ValidateSNIHost(domain); err != nil {
		return nil, fmt.Errorf("invalid HTTP route domain for edge rule: %w", err)
	}

	existing, _ := s.repo.FindBySNIHost(domain)
	if existing != nil {
		// Already exists — ensure it points to Caddy internal 8443
		if existing.TargetHost != "127.0.0.1" || existing.TargetPort != 8443 {
			existing.TargetHost = "127.0.0.1"
			existing.TargetPort = 8443
			existing.DeclaredKind = KindHTTPSApp
			existing.UpdatedAt = time.Now()
			if err := s.repo.Update(existing); err != nil {
				return nil, err
			}
		}
		return existing, nil
	}

	return s.CreateRule(ctx, CreateRuleInput{
		SNIHost:      domain,
		DeclaredKind: KindHTTPSApp,
		TargetHost:   "127.0.0.1",
		TargetPort:   8443,
	})
}

// ListRules returns all edge mux rules.
func (s *AppService) ListRules(ctx context.Context) ([]Rule, error) {
	rules, err := s.repo.FindAll()
	if err != nil { return nil, err }
	if rules == nil { rules = []Rule{} }
	return rules, nil
}

// GetRule returns a rule by ID.
func (s *AppService) GetRule(ctx context.Context, id string) (*Rule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil { return nil, err }
	if rule == nil { return nil, fmt.Errorf("edge rule %q not found", id) }
	return rule, nil
}

// EnableRule enables a disabled rule.
func (s *AppService) EnableRule(ctx context.Context, id string) error {
	rule, err := s.repo.FindByID(id)
	if err != nil { return err }
	if rule == nil { return fmt.Errorf("edge rule %q not found", id) }
	rule.Status = "active"
	rule.UpdatedAt = time.Now()
	return s.repo.Update(rule)
}

// DisableRule disables a rule.
func (s *AppService) DisableRule(ctx context.Context, id string) error {
	rule, err := s.repo.FindByID(id)
	if err != nil { return err }
	if rule == nil { return fmt.Errorf("edge rule %q not found", id) }
	rule.Status = "disabled"
	rule.UpdatedAt = time.Now()
	return s.repo.Update(rule)
}

// DeleteRule removes a rule.
func (s *AppService) DeleteRule(ctx context.Context, id string) error {
	return s.repo.Delete(id)
}
