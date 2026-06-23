package manageddomain

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
)

// AppService defines the managed domain application service.
type AppService struct {
	repo   *Repository
	logSvc *logs.AppService
}

// NewAppService creates a new managed domain application service.
func NewAppService(repo *Repository, logSvc *logs.AppService) *AppService {
	return &AppService{repo: repo, logSvc: logSvc}
}

// CreateManagedDomain creates a new managed domain.
// The domain starts in pending_verification status and requires DNS verification.
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

// VerifyDomain performs DNS verification for a managed domain.
// In v0.x, this is a basic check that marks the domain as verified
// (actual DNS lookup runs in dns_check.go).
func (s *AppService) VerifyDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, error) {
	md, err := s.getDomain(ctx, idOrDomain)
	if err != nil {
		return nil, err
	}

	if md.Status == "active" || md.Status == "verified" {
		return md, fmt.Errorf("domain %q is already %s", md.Domain, md.Status)
	}

	// Perform DNS TXT verification
	verified, msg := checkDNSTXT(md.VerificationName, md.VerificationValue)
	md.LastCheckMessage = msg
	md.UpdatedAt = time.Now()

	if verified {
		md.Status = "verified"
		s.logSvc.Log(ctx, "managed_domain.verify", "managed_domain", md.ID, "success",
			fmt.Sprintf("domain %q verified via DNS TXT", md.Domain), "cli")
	} else {
		md.Status = "failed"
		s.logSvc.Log(ctx, "managed_domain.verify", "managed_domain", md.ID, "failed",
			fmt.Sprintf("domain %q verification failed: %s", md.Domain, msg), "cli")
	}

	if err := s.repo.Update(md); err != nil {
		return nil, fmt.Errorf("update managed domain: %w", err)
	}
	return md, nil
}

// EnableDomain activates a verified managed domain.
func (s *AppService) EnableDomain(ctx context.Context, idOrDomain string) (*ManagedDomain, error) {
	md, err := s.getDomain(ctx, idOrDomain)
	if err != nil {
		return nil, err
	}

	if md.Status == "active" {
		return nil, fmt.Errorf("domain %q is already active", md.Domain)
	}

	if md.Status != "verified" {
		return nil, fmt.Errorf("domain %q must be verified before activation (current: %s)", md.Domain, md.Status)
	}

	md.Status = "active"
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

	if md.Status == "disabled" {
		return nil, fmt.Errorf("domain %q is already disabled", md.Domain)
	}

	md.Status = "disabled"
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
