package manageddomain

import (
	"context"
	"fmt"
	"time"

	aerrors "aegis/internal/core"
	"aegis/internal/id"
	"aegis/internal/logs"
)

// Allowed state transitions.
var allowedTransitions = map[string][]string{
	"pending_verification": {"verified", "failed"},
	"verified":             {"active"},
	"active":               {"disabled"},
	"disabled":             {"active"},
	"failed":               {"pending_verification"},
}

// AppService defines the managed domain application service.
type AppService struct {
	repo   *Repository
	logSvc logs.Logger
}

// NewAppService creates a new managed domain application service.
func NewAppService(repo *Repository, logSvc logs.Logger) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateManagedDomain creates a new managed domain in pending_verification status.
func (s *AppService) CreateManagedDomain(ctx context.Context, input CreateManagedDomainInput) (*ManagedDomain, error) {
	if input.Domain == "" {
		return nil, fmt.Errorf("domain is required")
	}
	if input.ServiceID == "" {
		return nil, fmt.Errorf("service is required")
	}
	if input.OwnerRef == "" {
		return nil, fmt.Errorf("owner reference is required")
	}

	existing, err := s.repo.FindByDomain(input.Domain)
	if err != nil {
		return nil, fmt.Errorf("check duplicate domain: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("managed domain %q already exists", input.Domain)
	}

	now := time.Now()
	verifyValue := fmt.Sprintf("aegis-verify-%s", id.New(""))
	verifyName := fmt.Sprintf("_aegis.%s", input.Domain)

	md := &ManagedDomain{
		ID:                id.New("md"),
		Domain:            input.Domain,
		ServiceID:         input.ServiceID,
		OwnerRef:          input.OwnerRef,
		TargetType:        input.TargetType,
		TargetRef:         input.TargetRef,
		VerificationType:  "dns_txt",
		VerificationName:  verifyName,
		VerificationValue: verifyValue,
		Status:            "pending_verification",
		TLSStatus:         "pending",
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.repo.Create(md); err != nil {
		s.logSvc.Log(ctx, "managed_domain.create", "managed_domain", md.ID, "failed", err.Error(), "cli")
		return nil, fmt.Errorf("create managed domain: %w", err)
	}

	s.logSvc.Log(ctx, "managed_domain.create", "managed_domain", md.ID, "success",
		fmt.Sprintf("created managed domain %q (owner: %s)", md.Domain, md.OwnerRef), "cli")
	return md, nil
}

// DNSVerificationResult holds structured DNS check results.
type DNSVerificationResult struct {
	TXT   DNSCheck `json:"txt"`
	CNAME DNSCheck `json:"cname,omitempty"`
	A     DNSCheck `json:"a,omitempty"`
	AAAA  DNSCheck `json:"aaaa,omitempty"`
}

// DNSCheck represents a single DNS record check.
type DNSCheck struct {
	Expected string   `json:"expected,omitempty"`
	Actual   []string `json:"actual"`
	OK       bool     `json:"ok"`
	Warning  bool     `json:"warning,omitempty"`
}

// VerifyDomain performs DNS verification and returns structured results.
func (s *AppService) VerifyDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, *DNSVerificationResult, error) {
	md, err := s.getDomain(ctx, idOrDomain)
	if err != nil {
		return nil, nil, err
	}

	if md.Status == "active" {
		return nil, nil, aerrors.StateTransitionInvalid(
			fmt.Sprintf("domain %q is already active; verification not needed", md.Domain))
	}

	result := &DNSVerificationResult{}

	// TXT verification
	txtVerified, txtMsg, txtRecords := checkDNSTXTWithRecords(md.VerificationName, md.VerificationValue)
	result.TXT = DNSCheck{
		Expected: md.VerificationValue,
		Actual:   txtRecords,
		OK:       txtVerified,
	}

	// CNAME check (optional)
	cnameTarget := md.Domain
	cnameValue, cnameErr := checkDNSRecordCNAME(cnameTarget)
	if cnameErr != nil {
		result.CNAME = DNSCheck{
			Expected: "(CNAME lookup)",
			Actual:   []string{},
			OK:       false,
			Warning:  true,
		}
	} else {
		result.CNAME = DNSCheck{
			Expected: "(CNAME lookup)",
			Actual:   []string{cnameValue},
			OK:       true,
			Warning:  false,
		}
	}

	// A record check
	aIPs, _ := lookupIP(md.Domain, "ip4")
	result.A = DNSCheck{Actual: aIPs, OK: len(aIPs) > 0}

	// AAAA record check
	aaaaIPs, _ := lookupIP(md.Domain, "ip6")
	result.AAAA = DNSCheck{Actual: aaaaIPs, OK: len(aaaaIPs) > 0}

	// Determine status
	newStatus := "failed"
	msg := fmt.Sprintf("TXT verification: %s", txtMsg)
	if txtVerified {
		newStatus = "verified"
		msg = "TXT record verified successfully"
	}

	// Enforce state transition
	if err := s.transitionStatus(md, newStatus); err != nil {
		return nil, nil, err
	}

	md.LastCheckMessage = msg
	md.UpdatedAt = time.Now()

	if err := s.repo.Update(md); err != nil {
		return nil, nil, fmt.Errorf("update managed domain: %w", err)
	}

	s.logSvc.Log(ctx, "managed_domain.verify", "managed_domain", md.ID, newStatus, msg, "cli")
	return md, result, nil
}

// EnableDomain activates a verified managed domain.
// Enforces state transition: verified → active.
func (s *AppService) EnableDomain(ctx context.Context, idOrDomain string, force bool) (*ManagedDomain, error) {
	md, err := s.getDomain(ctx, idOrDomain)
	if err != nil {
		return nil, err
	}

	if md.Status == "active" {
		return nil, fmt.Errorf("domain %q is already active", md.Domain)
	}

	if force {
		// Force enable: only allowed via admin:* scope (checked at API layer)
		md.Status = "active"
		md.UpdatedAt = time.Now()
		if err := s.repo.Update(md); err != nil {
			return nil, fmt.Errorf("force enable managed domain: %w", err)
		}
		s.logSvc.Log(ctx, "managed_domain.enable", "managed_domain", md.ID, "success",
			fmt.Sprintf("force-enabled managed domain %q", md.Domain), "api")
		return md, nil
	}

	if err := s.transitionStatus(md, "active"); err != nil {
		return nil, err
	}

	md.UpdatedAt = time.Now()
	if err := s.repo.Update(md); err != nil {
		return nil, fmt.Errorf("enable managed domain: %w", err)
	}

	s.logSvc.Log(ctx, "managed_domain.enable", "managed_domain", md.ID, "success",
		fmt.Sprintf("enabled managed domain %q", md.Domain), "cli")
	return md, nil
}

// DisableDomain disables a managed domain.
func (s *AppService) DisableDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, error) {
	md, err := s.getDomain(ctx, idOrDomain)
	if err != nil {
		return nil, err
	}

	if err := s.transitionStatus(md, "disabled"); err != nil {
		return nil, err
	}

	md.UpdatedAt = time.Now()
	if err := s.repo.Update(md); err != nil {
		return nil, fmt.Errorf("disable managed domain: %w", err)
	}

	s.logSvc.Log(ctx, "managed_domain.disable", "managed_domain", md.ID, "success",
		fmt.Sprintf("disabled managed domain %q", md.Domain), "cli")
	return md, nil
}

// ListManagedDomains returns all managed domains.
func (s *AppService) ListManagedDomains(ctx context.Context) ([]ManagedDomain, error) {
	domains, err := s.repo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("list managed domains: %w", err)
	}
	if domains == nil {
		domains = []ManagedDomain{}
	}
	return domains, nil
}

// GetManagedDomain finds a managed domain by ID or domain name.
func (s *AppService) GetManagedDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, error) {
	return s.getDomain(ctx, idOrDomain)
}

// transitionStatus validates and applies a state transition.
func (s *AppService) transitionStatus(md *ManagedDomain, newStatus string) error {
	if md.Status == newStatus {
		return nil
	}

	allowed, ok := allowedTransitions[md.Status]
	if !ok {
		return aerrors.StateTransitionInvalid(
			fmt.Sprintf("unknown current status: %s", md.Status))
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == newStatus {
			md.Status = newStatus
			return nil
		}
	}

	return aerrors.StateTransitionInvalid(
		fmt.Sprintf("cannot transition %q from %s to %s (allowed: %v)",
			md.Domain, md.Status, newStatus, allowed))
}

func (s *AppService) getDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, error) {
	md, err := s.repo.FindByID(idOrDomain)
	if err != nil {
		return nil, fmt.Errorf("find managed domain: %w", err)
	}
	if md != nil {
		return md, nil
	}

	md, err = s.repo.FindByDomain(idOrDomain)
	if err != nil {
		return nil, fmt.Errorf("find managed domain: %w", err)
	}
	if md == nil {
		return nil, fmt.Errorf("managed domain %q not found", idOrDomain)
	}
	return md, nil
}
