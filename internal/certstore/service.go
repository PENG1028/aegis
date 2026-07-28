package certstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"aegis/internal/core"
)

// Service handles certificate business logic: upload, validate, store, delete.
type Service struct {
	repo        *Repository
	certDir     string // filesystem directory for PEM files
	consumerGID int    // -1 keeps assets private to the Aegis process owner
	mu          sync.Mutex
}

const (
	privateCertDirMode  = os.FileMode(0700)
	sharedCertDirMode   = os.FileMode(0750)
	privateCertFileMode = os.FileMode(0600)
	sharedCertFileMode  = os.FileMode(0640)
)

// NewService creates a certificate service.
func NewService(repo *Repository, certDir string) *Service {
	return &Service{repo: repo, certDir: certDir, consumerGID: -1}
}

// ConfigureConsumerGroup grants one gateway service group read-only access to
// Aegis-managed PEM assets and repairs files created by older releases.
func (s *Service) ConfigureConsumerGroup(groupName string) error {
	group, err := user.LookupGroup(groupName)
	if err != nil {
		return fmt.Errorf("lookup certificate consumer group %q: %w", groupName, err)
	}
	gid, err := strconv.Atoi(group.Gid)
	if err != nil {
		return fmt.Errorf("parse certificate consumer group %q: %w", groupName, err)
	}
	return s.configureConsumerGID(gid)
}

func (s *Service) configureConsumerGID(gid int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	previousGID := s.consumerGID
	s.consumerGID = gid
	if err := s.ensureCertDir(); err != nil {
		s.consumerGID = previousGID
		return err
	}
	entries, err := os.ReadDir(s.certDir)
	if err != nil {
		s.consumerGID = previousGID
		return fmt.Errorf("read cert dir: %w", err)
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() && (filepath.Ext(entry.Name()) == ".crt" || filepath.Ext(entry.Name()) == ".key") {
			if err := s.applyFileAccess(filepath.Join(s.certDir, entry.Name())); err != nil {
				s.consumerGID = previousGID
				return fmt.Errorf("repair certificate access for %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
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
	s.mu.Lock()
	defer s.mu.Unlock()
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
	if time.Now().Before(cert.NotBefore) {
		return nil, fmt.Errorf("certificate is not valid yet (not before: %s)", cert.NotBefore.Format(time.RFC3339))
	}
	source := req.Source
	if source == "" {
		source = SourceManualUpload
	}
	if source != SourceManualUpload && source != SourceExternal && source != SourceLocalACME {
		return nil, fmt.Errorf("invalid certificate source %q", source)
	}

	// Ensure cert directory exists with the configured consumer access policy.
	if err := s.ensureCertDir(); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	// Generate ID and write PEM files
	certID := core.NewID("cert")
	certPath := filepath.Join(s.certDir, certID+".crt")
	keyPath := filepath.Join(s.certDir, certID+".key")

	if err := s.writeAssetFile(certPath, req.CertPEM); err != nil {
		return nil, fmt.Errorf("write cert file: %w", err)
	}
	if err := s.writeAssetFile(keyPath, req.KeyPEM); err != nil {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	cert, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if cert == nil {
		return fmt.Errorf("certificate %s not found", id)
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	// Files are removed after metadata so a DB failure never leaves a record
	// pointing at deleted certificate material.
	_ = os.Remove(cert.CertPath)
	_ = os.Remove(cert.KeyPath)
	return nil
}

// ReplaceWith promotes newly issued material into an existing certificate
// asset so route references keep a stable ID across renewal.
func (s *Service) ReplaceWith(existingID, replacementID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existingID == replacementID {
		return nil
	}
	existing, err := s.repo.FindByID(existingID)
	if err != nil || existing == nil {
		return fmt.Errorf("existing certificate %s not found", existingID)
	}
	replacement, err := s.repo.FindByID(replacementID)
	if err != nil || replacement == nil {
		return fmt.Errorf("replacement certificate %s not found", replacementID)
	}
	if existing.Source != SourceLocalACME || replacement.Source != SourceLocalACME {
		return fmt.Errorf("only local ACME certificates can be renewed in place")
	}

	oldCertPEM, err := os.ReadFile(existing.CertPath)
	if err != nil {
		return fmt.Errorf("read existing certificate: %w", err)
	}
	oldKeyPEM, err := os.ReadFile(existing.KeyPath)
	if err != nil {
		return fmt.Errorf("read existing key: %w", err)
	}
	newCertPEM, err := os.ReadFile(replacement.CertPath)
	if err != nil {
		return fmt.Errorf("read replacement certificate: %w", err)
	}
	newKeyPEM, err := os.ReadFile(replacement.KeyPath)
	if err != nil {
		return fmt.Errorf("read replacement key: %w", err)
	}

	if err := s.replaceFile(existing.CertPath, newCertPEM); err != nil {
		return fmt.Errorf("replace certificate file: %w", err)
	}
	if err := s.replaceFile(existing.KeyPath, newKeyPEM); err != nil {
		_ = s.replaceFile(existing.CertPath, oldCertPEM)
		return fmt.Errorf("replace key file: %w", err)
	}

	existing.Domains = replacement.Domains
	existing.Issuer = replacement.Issuer
	existing.NotBefore = replacement.NotBefore
	existing.NotAfter = replacement.NotAfter
	existing.Note = replacement.Note
	existing.UpdatedAt = time.Now()
	if err := s.repo.Update(existing); err != nil {
		_ = s.replaceFile(existing.CertPath, oldCertPEM)
		_ = s.replaceFile(existing.KeyPath, oldKeyPEM)
		return fmt.Errorf("update renewed certificate: %w", err)
	}

	if err := s.repo.Delete(replacementID); err != nil {
		return err
	}
	_ = os.Remove(replacement.CertPath)
	_ = os.Remove(replacement.KeyPath)
	return nil
}

func (s *Service) ensureCertDir() error {
	mode := privateCertDirMode
	if s.consumerGID >= 0 {
		mode = sharedCertDirMode
	}
	if err := os.MkdirAll(s.certDir, mode); err != nil {
		return err
	}
	if s.consumerGID >= 0 {
		if err := os.Chown(s.certDir, -1, s.consumerGID); err != nil {
			return err
		}
	}
	return os.Chmod(s.certDir, mode)
}

func (s *Service) writeAssetFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, privateCertFileMode); err != nil {
		return err
	}
	if err := s.applyFileAccess(path); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func (s *Service) applyFileAccess(path string) error {
	mode := privateCertFileMode
	if s.consumerGID >= 0 {
		if err := os.Chown(path, -1, s.consumerGID); err != nil {
			return err
		}
		mode = sharedCertFileMode
	}
	return os.Chmod(path, mode)
}

func (s *Service) replaceFile(path string, content []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".aegis-cert-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if s.consumerGID >= 0 {
		if err := tmp.Chown(-1, s.consumerGID); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	mode := privateCertFileMode
	if s.consumerGID >= 0 {
		mode = sharedCertFileMode
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	// Windows cannot atomically replace an existing file. The fallback retains
	// the old bytes unless the final write itself succeeds.
	if err := os.WriteFile(path, content, privateCertFileMode); err != nil {
		return err
	}
	return s.applyFileAccess(path)
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
