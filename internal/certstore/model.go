// Package certstore — certificate storage and lifecycle management.
//
// CertStore is a provider-agnostic certificate repository. It stores PEM-encoded
// TLS certificates and private keys on the filesystem, with metadata (domains,
// issuer, expiry) tracked in the database. Providers consume certificates by
// reading the filesystem paths returned by CertStore — they never interact with
// the database directly.
//
// The store does NOT implement ACME. Auto-cert (CapAutoCert) is delegated to
// the provider (Caddy's built-in client). CertStore holds ONLY user-provided
// certificates — purchased, Cloudflare Origin CA, self-signed, etc.
package certstore

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// Certificate represents a stored TLS certificate with metadata.
type Certificate struct {
	ID        string    `json:"id"`
	Domains   string    `json:"domains"`    // JSON array of DNS names from cert SAN/CN
	Issuer    string    `json:"issuer"`     // e.g. "CN=Let's Encrypt R3", "CN=Cloudflare Origin CA"
	NotBefore string    `json:"not_before"` // RFC3339
	NotAfter  string    `json:"not_after"`  // RFC3339
	CertPath  string    `json:"cert_path"`  // filesystem path to PEM certificate
	KeyPath   string    `json:"key_path"`   // filesystem path to PEM private key
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ValidatePEM checks that the provided bytes are valid PEM-encoded cert + key.
// Returns the parsed x509 certificate (for domain/expiry extraction) or an error.
func ValidatePEM(certPEM, keyPEM []byte) (*x509.Certificate, error) {
	// Decode cert
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return nil, fmt.Errorf("invalid PEM: no certificate block found")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate: %w", err)
	}

	// Decode key
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, fmt.Errorf("invalid PEM: no private key block found")
	}
	// Try parsing as various key types
	if _, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes); err != nil {
		if _, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes); err != nil {
			if _, err := x509.ParseECPrivateKey(keyBlock.Bytes); err != nil {
				return nil, fmt.Errorf("invalid private key: unsupported format (need PKCS#1, PKCS#8, or EC)")
			}
		}
	}

	return cert, nil
}

// DomainsFromCert extracts all DNS names from the certificate (SAN + CN fallback).
func DomainsFromCert(cert *x509.Certificate) []string {
	domains := cert.DNSNames
	if len(domains) == 0 && cert.Subject.CommonName != "" {
		domains = []string{cert.Subject.CommonName}
	}
	return domains
}
