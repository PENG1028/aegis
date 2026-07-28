package certstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ACMERenewer is an optional interface for ACME-based certificate renewal.
// The acme package implements this; certstore does not depend on acme.
type ACMERenewer interface {
	RenewCertificate(ctx context.Context, certID string, domains []string) (renewedID string, err error)
}

// ProviderReloader is implemented by the provider registry. The small
// interface keeps certificate policy independent from gateway packages.
type ProviderReloader interface {
	ReloadCertificateConsumers() error
}

// RenewalCoordinator holds the canonical apply lock across PEM replacement and
// provider reload, preventing config validation from observing a mixed pair.
type RenewalCoordinator interface {
	RenewCertificateAndApply(ctx context.Context, mutate func() error) error
}

// ExpiringCert wraps a Certificate with computed fields about its expiry.
type ExpiringCert struct {
	Certificate
	DaysLeft    int    `json:"days_left"`
	CanRenew    bool   `json:"can_renew"`
	RenewMethod string `json:"renew_method"`
	RenewNote   string `json:"renew_note"`
}

// RenewalResult is returned after a renewal attempt.
type RenewalResult struct {
	CertID       string `json:"cert_id"`
	Renewed      bool   `json:"renewed"`
	PendingApply bool   `json:"pending_apply,omitempty"`
	Message      string `json:"message"`
	NotAfter     string `json:"not_after"`
}

// CertRenewalChecker checks certificate expiry and triggers renewal.
type CertRenewalChecker struct {
	svc         *Service
	acme        ACMERenewer // nil if no ACME capability
	reloader    ProviderReloader
	coordinator RenewalCoordinator
}

func NewCertRenewalChecker(svc *Service, acme ACMERenewer) *CertRenewalChecker {
	return &CertRenewalChecker{svc: svc, acme: acme}
}

func (c *CertRenewalChecker) SetProviderReloader(reloader ProviderReloader) {
	c.reloader = reloader
}

func (c *CertRenewalChecker) SetRenewalCoordinator(coordinator RenewalCoordinator) {
	c.coordinator = coordinator
}

// Check returns all certificates expiring within the given number of days.
func (c *CertRenewalChecker) Check(ctx context.Context, withinDays int) ([]ExpiringCert, error) {
	all, err := c.svc.List()
	if err != nil {
		return nil, err
	}
	threshold := time.Now().Add(time.Duration(withinDays) * 24 * time.Hour)
	var expiring []ExpiringCert

	for _, cert := range all {
		notAfter, err := time.Parse(time.RFC3339, cert.NotAfter)
		if err != nil {
			continue
		}
		daysLeft := int(time.Until(notAfter).Hours() / 24)
		if notAfter.After(threshold) && daysLeft > 0 {
			continue
		}
		ec := ExpiringCert{Certificate: cert, DaysLeft: daysLeft}
		switch cert.Source {
		case SourceGatewayAuto:
			ec.RenewMethod = "gateway_auto"
			ec.CanRenew = false
			ec.RenewNote = "网关自动续期，重载网关提供者即可触发。"
		case SourceLocalACME:
			ec.RenewMethod = "acme"
			ec.CanRenew = c.acme != nil
			if c.acme == nil {
				ec.RenewNote = "ACME 未配置。请在设置中配置 proxy.email。"
			} else {
				ec.RenewNote = "可通过 ACME 续期。"
			}
		default:
			ec.RenewMethod = "manual"
			ec.CanRenew = false
			ec.RenewNote = "此证书为手动导入，无法自动续期。"
		}
		expiring = append(expiring, ec)
	}
	return expiring, nil
}

// Renew attempts to renew a certificate by ID.
func (c *CertRenewalChecker) Renew(ctx context.Context, certID string) (*RenewalResult, error) {
	cert, err := c.svc.Get(certID)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("certificate %s not found", certID)
	}
	switch cert.Source {
	case SourceGatewayAuto:
		return &RenewalResult{CertID: certID, Message: "Renewal is managed internally by the gateway provider."}, nil

	case SourceLocalACME:
		if c.acme == nil {
			return nil, fmt.Errorf("ACME not configured")
		}
		var domains []string
		if err := json.Unmarshal([]byte(cert.Domains), &domains); err != nil || len(domains) == 0 {
			return nil, fmt.Errorf("certificate domains are invalid")
		}
		newID := ""
		renew := func() error {
			var renewErr error
			newID, renewErr = c.acme.RenewCertificate(ctx, certID, domains)
			return renewErr
		}
		if c.coordinator != nil {
			err = c.coordinator.RenewCertificateAndApply(ctx, renew)
		} else {
			err = renew()
		}
		if err != nil {
			if newID != "" {
				return &RenewalResult{CertID: newID, Renewed: true, PendingApply: true, Message: fmt.Sprintf("ACME renewed, but provider apply failed: %v", err)}, nil
			}
			return &RenewalResult{CertID: certID, Message: fmt.Sprintf("ACME renewal failed: %v", err)}, nil
		}
		if c.coordinator != nil {
			return &RenewalResult{CertID: newID, Renewed: true, Message: "ACME renewal succeeded."}, nil
		}
		if c.reloader == nil {
			return &RenewalResult{CertID: newID, Renewed: true, PendingApply: true, Message: "ACME renewed, but provider reload is unavailable."}, nil
		}
		if err := c.reloader.ReloadCertificateConsumers(); err != nil {
			return &RenewalResult{CertID: newID, Renewed: true, PendingApply: true, Message: fmt.Sprintf("ACME renewed, but provider reload failed: %v", err)}, nil
		}
		return &RenewalResult{CertID: newID, Renewed: true, Message: "ACME renewal succeeded."}, nil

	default:
		return &RenewalResult{CertID: certID, Message: "此证书为手动导入，无法自动续期。"}, nil
	}
}

// RenewExpiring renews eligible local ACME certificates inside the threshold.
func (c *CertRenewalChecker) RenewExpiring(ctx context.Context, withinDays int) []RenewalResult {
	expiring, err := c.Check(ctx, withinDays)
	if err != nil {
		return []RenewalResult{{Message: err.Error()}}
	}
	results := make([]RenewalResult, 0)
	for _, cert := range expiring {
		if !cert.CanRenew || cert.Source != SourceLocalACME {
			continue
		}
		result, err := c.Renew(ctx, cert.ID)
		if err != nil {
			results = append(results, RenewalResult{CertID: cert.ID, Message: err.Error()})
			continue
		}
		results = append(results, *result)
	}
	return results
}
