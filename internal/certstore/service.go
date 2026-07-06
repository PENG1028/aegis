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

	now := time.Now()
	c := &Certificate{
		ID:        certID,
		Domains:   string(domainsJSON),
		Issuer:    issuer,
		NotBefore: cert.NotBefore.Format(time.RFC3339),
		NotAfter:  cert.NotAfter.Format(time.RFC3339),
		CertPath:  certPath,
		KeyPath:   keyPath,
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
