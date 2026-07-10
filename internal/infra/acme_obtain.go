package infra

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"aegis/internal/certstore"
)

// ACMEManager handles ACME certificate lifecycle via certbot.
type ACMEManager struct {
	CertStore  *certstore.Service
	Email      string
	AcmeServer string // staging URL for testing, empty = production
	DataDir    string // certbot working directory
}

// NewACMEManager creates an ACME manager.
func NewACMEManager(certStore *certstore.Service, email, dataDir, acmeServer string) *ACMEManager {
	if dataDir == "" {
		dataDir = "/var/lib/aegis/acme"
	}
	return &ACMEManager{CertStore: certStore, Email: email, AcmeServer: acmeServer, DataDir: dataDir}
}

// ObtainRequest contains the parameters for obtaining a certificate.
type ObtainRequest struct {
	Domains []string
}

// Obtain gets a certificate via certbot and imports it into certstore.
func (m *ACMEManager) Obtain(ctx context.Context, req ObtainRequest) (certID string, err error) {
	if len(req.Domains) == 0 {
		return "", fmt.Errorf("at least one domain required")
	}
	if m.Email == "" {
		return "", fmt.Errorf("ACME email not configured (set proxy.email in config)")
	}

	primaryDomain := req.Domains[0]
	if err := os.MkdirAll(m.DataDir, 0700); err != nil {
		return "", fmt.Errorf("create acme data dir: %w", err)
	}

	args := []string{"certonly", "--standalone", "--non-interactive", "--agree-tos",
		"--email", m.Email, "--cert-name", sanitizeCertName(primaryDomain), "-d", primaryDomain,
		"--work-dir", m.DataDir, "--config-dir", m.DataDir, "--logs-dir", filepath.Join(m.DataDir, "logs")}
	for _, d := range req.Domains[1:] {
		args = append(args, "-d", d)
	}
	if m.AcmeServer != "" {
		args = append(args, "--server", m.AcmeServer)
	}

	log.Printf("[acme] running certbot for %s...", primaryDomain)
	cmd := exec.CommandContext(ctx, "certbot", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("certbot failed: %w\n%s", err, string(output))
	}

	certPath := filepath.Join(m.DataDir, "live", sanitizeCertName(primaryDomain), "fullchain.pem")
	keyPath := filepath.Join(m.DataDir, "live", sanitizeCertName(primaryDomain), "privkey.pem")
	certPEM, _ := os.ReadFile(certPath)
	keyPEM, _ := os.ReadFile(keyPath)

	cert, err := m.CertStore.Upload(certstore.UploadRequest{
		CertPEM: certPEM, KeyPEM: keyPEM,
		Source: certstore.SourceLocalACME,
		Note:   fmt.Sprintf("ACME auto-issued via certbot for %s", strings.Join(req.Domains, ", ")),
	})
	if err != nil {
		return "", fmt.Errorf("store certificate: %w", err)
	}
	log.Printf("[acme] certificate obtained: %s → certstore:%s", primaryDomain, cert.ID)
	return cert.ID, nil
}

// IsAvailable returns true if certbot is installed and email is configured.
func (m *ACMEManager) IsAvailable() bool {
	return DetectCertbot(m.Email).Available
}

func sanitizeCertName(domain string) string {
	return strings.ReplaceAll(domain, "*", "wildcard")
}
