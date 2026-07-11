package certstore

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ACMERenewer is an optional interface for ACME-based certificate renewal.
// The acme package implements this; certstore does not depend on acme.
type ACMERenewer interface {
	Renew(ctx context.Context, domains []string) (certID string, err error)
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
	CertID   string `json:"cert_id"`
	Renewed  bool   `json:"renewed"`
	Message  string `json:"message"`
	NotAfter string `json:"not_after"`
}

// CertRenewalChecker checks certificate expiry and triggers renewal.
type CertRenewalChecker struct {
	svc  *Service
	acme ACMERenewer // nil if no ACME capability
}

func NewCertRenewalChecker(svc *Service, acme ACMERenewer) *CertRenewalChecker {
	return &CertRenewalChecker{svc: svc, acme: acme}
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
			ec.RenewMethod = "caddy_auto"
			ec.CanRenew = true
			ec.RenewNote = "Caddy 自动续期，无需手动干预。"
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
	switch cert.Source {
	case SourceGatewayAuto:
		out, err := exec.CommandContext(ctx, "systemctl", "reload", "caddy").CombinedOutput()
		if err != nil {
			return &RenewalResult{CertID: certID, Message: fmt.Sprintf("Caddy reload failed: %v — %s", err, string(out))}, nil
		}
		c.svc.SyncAutoCerts("")
		return &RenewalResult{CertID: certID, Renewed: true, Message: "Caddy reloaded, auto-renewal triggered."}, nil

	case SourceLocalACME:
		if c.acme == nil {
			return nil, fmt.Errorf("ACME not configured")
		}
		var domains []string
		fmt.Sscanf(cert.Domains, "%q", &domains)
		newID, err := c.acme.Renew(ctx, domains)
		if err != nil {
			return &RenewalResult{CertID: certID, Message: fmt.Sprintf("ACME renewal failed: %v", err)}, nil
		}
		return &RenewalResult{CertID: newID, Renewed: true, Message: "ACME renewal succeeded."}, nil

	default:
		return &RenewalResult{CertID: certID, Message: "此证书为手动导入，无法自动续期。"}, nil
	}
}
