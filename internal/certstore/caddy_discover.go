package certstore

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiscoveredCert is a lightweight cert found in a provider's cert store.
// Unlike Certificate, it has no DB ID — it's discovered from filesystem.
type DiscoveredCert struct {
	Domains   string `json:"domains"`    // JSON array
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	Source    string `json:"source"` // gateway_auto
	ACMEPath  string `json:"acme_path,omitempty"`
}

// DiscoverCaddyCerts scans the Caddy certificate storage directory and returns
// all discovered auto-issued certificates. Returns empty slice if the directory
// doesn't exist or is inaccessible.
//
// Caddy stores certs at:
//
//	/var/lib/caddy/.local/share/caddy/certificates/{acme_dir}/{domain}/{domain}.crt
//	/var/lib/caddy/.local/share/caddy/certificates/{acme_dir}/{domain}/{domain}.json
func DiscoverCaddyCerts(caddyDataDir string) ([]DiscoveredCert, error) {
	if caddyDataDir == "" {
		caddyDataDir = "/var/lib/caddy/.local/share/caddy/certificates"
	}

	certDir := filepath.Join(caddyDataDir, "certificates")
	if _, err := os.Stat(certDir); os.IsNotExist(err) {
		// Also try the direct path (old Caddy versions or custom config)
		if _, err2 := os.Stat(caddyDataDir); os.IsNotExist(err2) {
			return nil, nil // no certs yet
		}
		certDir = caddyDataDir
	}

	var certs []DiscoveredCert

	// Walk {acme_dir}/{domain}/ directory structure
	entries, err := os.ReadDir(certDir)
	if err != nil {
		return nil, nil // permission denied or doesn't exist — not an error
	}

	for _, acmeEntry := range entries {
		if !acmeEntry.IsDir() {
			continue
		}
		acmePath := filepath.Join(certDir, acmeEntry.Name())

		domainEntries, err := os.ReadDir(acmePath)
		if err != nil {
			continue
		}

		for _, domainEntry := range domainEntries {
			if !domainEntry.IsDir() {
				continue
			}
			domainPath := filepath.Join(acmePath, domainEntry.Name())
			certFile := filepath.Join(domainPath, domainEntry.Name()+".crt")
			jsonFile := filepath.Join(domainPath, domainEntry.Name()+".json")

			dc, err := parseCaddyCert(certFile, jsonFile)
			if err != nil {
				continue // skip unreadable certs
			}
			certs = append(certs, dc)
		}
	}

	return certs, nil
}

// parseCaddyCert reads a single Caddy-managed certificate.
func parseCaddyCert(certFile, jsonFile string) (DiscoveredCert, error) {
	dc := DiscoveredCert{Source: SourceGatewayAuto}

	// Read PEM to extract cert metadata
	pemData, err := os.ReadFile(certFile)
	if err != nil {
		return dc, err
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		return dc, fmt.Errorf("no PEM block in %s", certFile)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return dc, fmt.Errorf("parse cert: %w", err)
	}

	// Extract domains
	domains := DomainsFromCert(cert)
	domainsJSON, _ := json.Marshal(domains)
	dc.Domains = string(domainsJSON)
	dc.Issuer = cert.Issuer.String()
	dc.NotBefore = cert.NotBefore.Format(time.RFC3339)
	dc.NotAfter = cert.NotAfter.Format(time.RFC3339)

	// Extract ACME directory path from the file path for reference
	// e.g. "acme-v02.api.letsencrypt.org-directory"
	if parts := strings.Split(certFile, string(filepath.Separator)); len(parts) >= 4 {
		dc.ACMEPath = parts[len(parts)-4] // the ACME directory name
	}

	// Read JSON metadata if available (for SANs, issuer URL)
	if jsonData, err := os.ReadFile(jsonFile); err == nil {
		var meta struct {
			SANs       []string `json:"sans"`
			IssuerData struct {
				URL string `json:"url"`
			} `json:"issuer_data"`
		}
		if json.Unmarshal(jsonData, &meta) == nil && len(meta.SANs) > 0 {
			sansJSON, _ := json.Marshal(meta.SANs)
			dc.Domains = string(sansJSON)
		}
	}

	return dc, nil
}
