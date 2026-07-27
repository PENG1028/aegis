package egress

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Dependencies holds external deps for Service.
type Dependencies struct {
	Repo  *Repository
	IDGen func() string
}

// Service provides egress rule business logic.
type Service struct {
	repo  *Repository
	idGen func() string
}

// NewService creates an egress service.
func NewService(deps Dependencies) *Service {
	if deps.IDGen == nil {
		deps.IDGen = func() string { return fmt.Sprintf("egr_%x", time.Now().UnixNano()) }
	}
	return &Service{
		repo:  deps.Repo,
		idGen: deps.IDGen,
	}
}

// CreateRule creates a new egress rule.
func (s *Service) CreateRule(ctx context.Context, rule *EgressRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	rule.ID = s.idGen()
	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now
	if rule.Status == "" {
		rule.Status = "active"
	}
	return s.repo.Create(rule)
}

// ListRules returns all rules.
func (s *Service) ListRules(ctx context.Context) ([]EgressRule, error) {
	return s.repo.FindAll()
}

// GetRule returns a single rule.
func (s *Service) GetRule(ctx context.Context, id string) (*EgressRule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, ErrNotFound
	}
	return rule, nil
}

// UpdateRule updates a rule.
func (s *Service) UpdateRule(ctx context.Context, rule *EgressRule) error {
	if err := rule.Validate(); err != nil {
		return err
	}
	rule.UpdatedAt = time.Now()
	return s.repo.Update(rule)
}

// DeleteRule removes a rule.
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	return s.repo.Delete(id)
}

// ─── DNS Integration: RuleChecker ───

// RuleChecker checks whether a domain should bypass internal resolution.
// Used by the DNS resolver to skip allowlisted domains.
type RuleChecker struct {
	svc   *Service
	mu    sync.RWMutex
	cache []EgressRule
}

// NewRuleChecker creates a rule checker with cached lookups.
func NewRuleChecker(svc *Service) *RuleChecker {
	return &RuleChecker{svc: svc}
}

// Refresh reloads the cached rules from the DB.
func (c *RuleChecker) Refresh() error {
	rules, err := c.svc.repo.FindActive()
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.cache = rules
	c.mu.Unlock()
	return nil
}

// IsAllowlisted returns true if domain matches an active allow rule.
// Allowlisted domains bypass internal DNS resolution → upstream only.
func (c *RuleChecker) IsAllowlisted(domain string) bool {
	c.mu.RLock()
	rules := c.cache
	c.mu.RUnlock()
	for _, r := range rules {
		if r.Type == TypeAllow && r.MatchesDomain(domain) {
			return true
		}
	}
	return false
}

// IsBlocked returns true if domain matches an active block rule.
func (c *RuleChecker) IsBlocked(domain string) bool {
	c.mu.RLock()
	rules := c.cache
	c.mu.RUnlock()
	for _, r := range rules {
		if r.Type == TypeBlock && r.MatchesDomain(domain) {
			return true
		}
	}
	return false
}
