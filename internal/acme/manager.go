// Package acme — Aegis built-in ACME client for automatic TLS certificate issuance.
//
// Uses certbot (or acme.sh) under the hood for battle-tested ACME handling.
// Obtained certificates are imported into certstore and usable by any provider
// with CapLoadCert. Providers with CapAutoCert (Caddy) manage their own certs
// and never touch this package.
package acme

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aegis/internal/infra"
	"aegis/internal/certstore"
)

// Manager handles ACME certificate lifecycle via certbot.
type Manager struct {
	certStore  *certstore.Service
	email      string
	acmeServer string // Let's Encrypt prod or staging
	dataDir    string // certbot working directory
}

// NewManager creates an ACME manager.
// dataDir: certbot working directory (default: /var/lib/aegis/acme).
// acmeServer: staging URL for testing, empty for production Let's Encrypt.
func NewManager(certStore *certstore.Service, email, dataDir, acmeServer string) *Manager {
	if dataDir == "" {
		dataDir = "/var/lib/aegis/acme"
	}
	return &Manager{
		certStore:  certStore,
		email:      email,
		acmeServer: acmeServer,
		dataDir:    dataDir,
	}
}

// ObtainRequest contains the parameters for obtaining a certificate.
type ObtainRequest struct {
	Domains []string // primary domain first, SANs follow
}

// Obtain gets a certificate from the ACME CA and stores it in certstore.
// Uses certbot in standalone HTTP-01 mode — requires port 80 to be free
// during issuance. Returns the certificate ID on success.
func (m *Manager) Obtain(ctx context.Context, req ObtainRequest) (certID string, err error) {
	if len(req.Domains) == 0 {
		return "", fmt.Errorf("at least one domain required")
	}
	if m.email == "" {
		return "", fmt.Errorf("ACME email not configured (set proxy.email in config)")
	}

	primaryDomain := req.Domains[0]

	// Ensure data directory exists
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return "", fmt.Errorf("create acme data dir: %w", err)
	}

	// Build certbot command
	args := []string{
		"certonly",
		"--standalone",
		"--non-interactive",
		"--agree-tos",
		"--email", m.email,
		"--cert-name", sanitizeCertName(primaryDomain),
		"-d", primaryDomain,
		"--work-dir", m.dataDir,
		"--config-dir", m.dataDir,
		"--logs-dir", filepath.Join(m.dataDir, "logs"),
	}

	// Add SAN domains
	for _, d := range req.Domains[1:] {
		args = append(args, "-d", d)
	}

	// Use staging server for testing
	if m.acmeServer != "" {
		args = append(args, "--server", m.acmeServer)
	}

	// Run certbot
	log.Printf("[acme] running certbot for %s...", primaryDomain)
	cmd := exec.CommandContext(ctx, "certbot", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("certbot failed: %w\n%s", err, string(output))
	}

	// Read the resulting certificate files
	certPath := filepath.Join(m.dataDir, "live", sanitizeCertName(primaryDomain), "fullchain.pem")
	keyPath := filepath.Join(m.dataDir, "live", sanitizeCertName(primaryDomain), "privkey.pem")

	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return "", fmt.Errorf("read cert file: %w", err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("read key file: %w", err)
	}

	// Import into certstore
	cert, err := m.certStore.Upload(certstore.UploadRequest{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		Note:    fmt.Sprintf("ACME auto-issued via certbot for %s", strings.Join(req.Domains, ", ")),
	})
	if err != nil {
		return "", fmt.Errorf("store certificate: %w", err)
	}

	log.Printf("[acme] certificate obtained: %s → certstore:%s", primaryDomain, cert.ID)
	return cert.ID, nil
}

// Status returns the infrastructure dependency status for the ACME client.
func (m *Manager) Status() infra.Status {
	return infra.DetectCertbot(m.email)
}

// IsAvailable returns true if certbot is installed and email is configured.
func (m *Manager) IsAvailable() bool {
	return m.Status().Available
}

// IsAvailableMessage returns a human-readable availability status.
func (m *Manager) IsAvailableMessage() string {
	return m.Status().Message
}

// sanitizeCertName converts a domain to a certbot-safe cert name.
func sanitizeCertName(domain string) string {
	return strings.ReplaceAll(domain, "*", "wildcard")
}
