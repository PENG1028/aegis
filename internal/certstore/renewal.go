package certstore

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ─── Certificate Renewal Checker ─────────────────────────────────────────
// CertRenewalChecker provides a provider-agnostic interface for checking
// certificate expiry and triggering renewal. It doesn't care which provider
// issued the cert — it routes to the right renewal method based on source.

// ExpiringCert wraps a Certificate with computed fields about its expiry
// and whether auto-renewal is possible.
type ExpiringCert struct {
	Certificate
	DaysLeft    int    `json:"days_left"`    // days until expiry (negative = expired)
	CanRenew    bool   `json:"can_renew"`    // whether we can auto-renew this cert
	RenewMethod string `json:"renew_method"` // "caddy_auto" | "certbot" | "manual"
	RenewNote   string `json:"renew_note"`   // human-readable instructions
}

// RenewalResult is returned after a renewal attempt.
type RenewalResult struct {
	CertID    string `json:"cert_id"`    // new or updated cert ID
	Renewed   bool   `json:"renewed"`    // whether a new cert was obtained
	Message   string `json:"message"`    // human-readable result
	NotAfter  string `json:"not_after"`  // new expiry date
}

// CertRenewalChecker implements certificate expiry checking and renewal.
// It uses the CertStore repository and knows how to renew certs by source.
type CertRenewalChecker struct {
	svc       *Service
	acmeEmail string // from proxy.email, needed for certbot
}

// NewCertRenewalChecker creates a renewal checker.
func NewCertRenewalChecker(svc *Service, acmeEmail string) *CertRenewalChecker {
	return &CertRenewalChecker{svc: svc, acmeEmail: acmeEmail}
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
			continue // not expiring soon enough
		}

		ec := ExpiringCert{
			Certificate: cert,
			DaysLeft:    daysLeft,
		}

		switch cert.Source {
		case SourceGatewayAuto:
			ec.RenewMethod = "caddy_auto"
			ec.CanRenew = true
			ec.RenewNote = "Caddy 自动续期，无需手动干预。如未续期，请检查 Caddy 服务状态和 DNS 解析。"
		case SourceLocalACME:
			ec.RenewMethod = "certbot"
			ec.CanRenew = c.acmeEmail != ""
			if c.acmeEmail == "" {
				ec.RenewNote = "未配置 ACME email（proxy.email），无法自动续期。请在设置中配置后重试。"
			} else {
				ec.RenewNote = "将通过 certbot 续期。需确保域名 DNS 仍指向本机且 80 端口可用。"
			}
		default: // manual_upload, external
			ec.RenewMethod = "manual"
			ec.CanRenew = false
			ec.RenewNote = "此证书为手动导入，无法自动续期。请在证书过期前重新上传或通过 ACME 申请新证书。"
		}

		expiring = append(expiring, ec)
	}
	return expiring, nil
}

// Renew attempts to renew a certificate by ID. Returns the result.
func (c *CertRenewalChecker) Renew(ctx context.Context, certID string) (*RenewalResult, error) {
	cert, err := c.svc.Get(certID)
	if err != nil {
		return nil, err
	}

	switch cert.Source {
	case SourceGatewayAuto:
		// Caddy handles renewal automatically. Trigger a Caddy reload
		// which will cause Caddy to re-check cert expiry and renew if needed.
		output, err := exec.CommandContext(ctx, "systemctl", "reload", "caddy").CombinedOutput()
		if err != nil {
			return &RenewalResult{
				CertID:  certID,
				Renewed: false,
				Message: fmt.Sprintf("Caddy reload failed: %v — %s", err, string(output)),
			}, nil
		}
		// Re-sync to pick up potentially renewed cert
		c.svc.SyncAutoCerts("")
		return &RenewalResult{
			CertID:  certID,
			Renewed: true,
			Message: "Caddy reloaded, auto-renewal triggered if cert was expiring. CertStore re-synced.",
		}, nil

	case SourceLocalACME:
		if c.acmeEmail == "" {
			return nil, fmt.Errorf("ACME email not configured")
		}
		output, err := exec.CommandContext(ctx, "certbot", "renew",
			"--non-interactive", "--agree-tos").CombinedOutput()
		if err != nil {
			return &RenewalResult{
				CertID:  certID,
				Renewed: false,
				Message: fmt.Sprintf("certbot renew failed: %v — %s", err, string(output)),
			}, nil
		}
		// After renewal, re-import the renewed cert
		c.svc.SyncAutoCerts("")
		return &RenewalResult{
			CertID:  certID,
			Renewed: true,
			Message: fmt.Sprintf("certbot renew succeeded: %s", string(output)),
		}, nil

	default:
		return &RenewalResult{
			CertID:  certID,
			Renewed: false,
			Message: "此证书为手动导入，无法自动续期。请重新上传新证书或通过 ACME 申请。",
		}, nil
	}
}
