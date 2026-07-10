package certstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aegis/internal/core"
)

// Service handles certificate business logic: upload, validate, store, delete.
type Service struct {
	repo    *Repository
	certDir string // filesystem directory for PEM files
}

// NewService creates a certificate service.
func NewService(repo *Repository, certDir string) *Service {
	return &Service{repo: repo, certDir: certDir}
}

// UploadRequest is the input for uploading a new certificate.
type UploadRequest struct {
	CertPEM []byte `json:"cert_pem"` // raw PEM certificate
	KeyPEM  []byte `json:"key_pem"`  // raw PEM private key
	Source  string `json:"source"`   // override source; defaults to manual_upload
	Note    string `json:"note,omitempty"`
}

// Upload validates and stores a certificate.
func (s *Service) Upload(req UploadRequest) (*Certificate, error) {
	// Validate PEM
	cert, err := ValidatePEM(req.CertPEM, req.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Extract metadata
	domains := DomainsFromCert(cert)
	if len(domains) == 0 {
		return nil, fmt.Errorf("certificate has no DNS names")
	}
	domainsJSON, _ := json.Marshal(domains)
	issuer := cert.Issuer.String()

	// Expiry warning
	if time.Now().After(cert.NotAfter) {
		return nil, fmt.Errorf("certificate has already expired (not after: %s)", cert.NotAfter.Format(time.RFC3339))
	}

	// Ensure cert directory exists
	if err := os.MkdirAll(s.certDir, 0700); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	// Generate ID and write PEM files
	certID := core.NewID("cert")
	certPath := filepath.Join(s.certDir, certID+".crt")
	keyPath := filepath.Join(s.certDir, certID+".key")

	if err := os.WriteFile(certPath, req.CertPEM, 0600); err != nil {
		return nil, fmt.Errorf("write cert file: %w", err)
	}
	if err := os.WriteFile(keyPath, req.KeyPEM, 0600); err != nil {
		os.Remove(certPath) // clean up
		return nil, fmt.Errorf("write key file: %w", err)
	}

	source := req.Source
	if source == "" {
		source = SourceManualUpload
	}

	now := time.Now()
	c := &Certificate{
		ID:        certID,
		Domains:   string(domainsJSON),
		Issuer:    issuer,
		NotBefore: cert.NotBefore.Format(time.RFC3339),
		NotAfter:  cert.NotAfter.Format(time.RFC3339),
		CertPath:  certPath,
		KeyPath:   keyPath,
		Source:    source,
		Note:      req.Note,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(c); err != nil {
		os.Remove(certPath)
		os.Remove(keyPath)
		return nil, fmt.Errorf("save certificate: %w", err)
	}

	return c, nil
}

// List returns all stored certificates.
func (s *Service) List() ([]Certificate, error) {
	return s.repo.FindAll()
}

// Get returns a single certificate by ID.
func (s *Service) Get(id string) (*Certificate, error) {
	return s.repo.FindByID(id)
}

// Delete removes a certificate from filesystem and database.
func (s *Service) Delete(id string) error {
	cert, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("certificate %s not found", id)
	}
	// Remove files (best-effort)
	os.Remove(cert.CertPath)
	os.Remove(cert.KeyPath)
	return s.repo.Delete(id)
}

// GetPaths returns the cert and key filesystem paths for a certificate ID.
// Used by Provider renderers to emit cert directives.
func (s *Service) GetPaths(id string) (certPath, keyPath string, err error) {
	cert, err := s.repo.FindByID(id)
	if err != nil {
		return "", "", err
	}
	if cert == nil {
		return "", "", fmt.Errorf("certificate %s not found", id)
	}
	return cert.CertPath, cert.KeyPath, nil
}

// SyncAutoCerts scans Caddy's certificate directory and imports discovered
// certs into CertStore. Existing certs for the same domains are updated
// if the discovered cert is newer. Returns count and IDs of imported/updated.
func (s *Service) SyncAutoCerts(caddyDataDir string) (int, []string, error) {
	discovered, err := DiscoverCaddyCerts(caddyDataDir)
	if err != nil {
		return 0, nil, err
	}

	var certIDs []string
	count := 0
	for _, dc := range discovered {
		cert, err := s.upsertAutoCert(dc)
		if err != nil {
			continue
		}
		certIDs = append(certIDs, cert.ID)
		count++
	}
	return count, certIDs, nil
}

// upsertAutoCert imports one discovered auto-cert. If a cert for the same
// domains already exists, it's updated if newer; otherwise inserted.
func (s *Service) upsertAutoCert(dc DiscoveredCert) (*Certificate, error) {
	var domains []string
	json.Unmarshal([]byte(dc.Domains), &domains)
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains in discovered cert")
	}

	existing, _ := s.repo.FindByDomain(domains[0])
	for _, e := range existing {
		var ed []string
		json.Unmarshal([]byte(e.Domains), &ed)
		if stringSlicesEqual(domains, ed) {
			if dc.NotAfter > e.NotAfter {
				e.NotBefore = dc.NotBefore
				e.NotAfter = dc.NotAfter
				e.Issuer = dc.Issuer
				e.UpdatedAt = time.Now()
				if err := s.repo.Update(&e); err != nil {
					return nil, err
				}
			}
			return &e, nil
		}
	}

	// New cert — copy files from Caddy's directory
	certID := core.NewID("cert")
	certPath := filepath.Join(s.certDir, certID+".crt")
	keyPath := filepath.Join(s.certDir, certID+".key")

	caddyCert, caddyKey := findCaddyFiles(dc)
	if caddyCert == "" {
		return nil, fmt.Errorf("caddy cert file not found")
	}

	certPEM, _ := os.ReadFile(caddyCert)
	keyPEM, _ := os.ReadFile(caddyKey)
	os.WriteFile(certPath, certPEM, 0600)
	os.WriteFile(keyPath, keyPEM, 0600)

	now := time.Now()
	c := &Certificate{
		ID:        certID,
		Domains:   dc.Domains,
		Issuer:    dc.Issuer,
		NotBefore: dc.NotBefore,
		NotAfter:  dc.NotAfter,
		CertPath:  certPath,
		KeyPath:   keyPath,
		Source:    SourceGatewayAuto,
		Note:      "Caddy 自动签发",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.Create(c); err != nil {
		os.Remove(certPath)
		os.Remove(keyPath)
		return nil, err
	}
	return c, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findCaddyFiles(dc DiscoveredCert) (certPath, keyPath string) {
	base := "/var/lib/caddy/.local/share/caddy/certificates"
	var domains []string
	json.Unmarshal([]byte(dc.Domains), &domains)
	if len(domains) == 0 {
		return "", ""
	}
	domain := domains[0]
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		certFile := filepath.Join(base, e.Name(), domain, domain+".crt")
		if _, err := os.Stat(certFile); err == nil {
			return certFile, filepath.Join(base, e.Name(), domain, domain+".key")
		}
	}
	return "", ""
}
