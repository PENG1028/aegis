package acme

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"aegis/internal/certstore"
)

// Client wraps the lego ACME library to obtain and renew certificates.
// It replaces the external certbot CLI with an embedded ACME implementation.
type Client struct {
	email      string
	acmeServer string // empty = production LE, set for staging
	dataDir    string
	certStore  *certstore.Service
	accountKey *ecdsa.PrivateKey
	legoClient *lego.Client
	mu         chan struct{} // capacity 1 → single concurrent obtain
}

// NewClient creates an ACME client. The account key is loaded or generated
// from dataDir/acme/account.key. email is required for LE registration.
// acmeServer can be empty (production) or a staging URL for testing.
func NewClient(certStore *certstore.Service, email, acmeServer, dataDir string) (*Client, error) {
	key, err := LoadOrCreateAccountKey(dataDir)
	if err != nil {
		return nil, fmt.Errorf("acme account key: %w", err)
	}

	c := &Client{
		email:      email,
		acmeServer: acmeServer,
		dataDir:    dataDir,
		certStore:  certStore,
		accountKey: key,
		mu:         make(chan struct{}, 1),
	}

	if email != "" {
		if err := c.initLego(); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Available returns true if the client can obtain certificates (email configured).
func (c *Client) Available() bool {
	return c.legoClient != nil
}

func (c *Client) initLego() error {
	config := lego.NewConfig(&acmeUser{email: c.email, key: c.accountKey})
	config.CADirURL = c.acmeServer
	if config.CADirURL == "" {
		config.CADirURL = lego.LEDirectoryProduction
	}
	config.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("create lego client: %w", err)
	}

	// HTTP-01 challenge — opens a temporary listener on :80 during validation
	if err := client.Challenge.SetHTTP01Provider(http01.NewProviderServer("", "80")); err != nil {
		return fmt.Errorf("set HTTP-01 provider: %w", err)
	}

	// Register or recover account
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		// Try to recover existing account
		if strings.Contains(err.Error(), "already registered") {
			reg, err = client.Registration.ResolveAccountByKey()
		}
		if err != nil {
			return fmt.Errorf("register ACME account: %w", err)
		}
	}
	_ = reg

	c.legoClient = client
	return nil
}

// ObtainResult is returned by Obtain.
type ObtainResult struct {
	CertID  string   `json:"cert_id"`
	Domains []string `json:"domains"`
}

// Obtain gets a certificate for the given domains via ACME and stores it
// in CertStore. Only one Obtain call runs at a time.
func (c *Client) Obtain(ctx context.Context, domains []string) (*ObtainResult, error) {
	if !c.Available() {
		return nil, fmt.Errorf("ACME not available — configure proxy.email in settings")
	}

	select {
	case c.mu <- struct{}{}:
		defer func() { <-c.mu }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	if len(domains) == 0 {
		return nil, fmt.Errorf("at least one domain required")
	}

	primary := domains[0]
	log.Printf("[acme] obtaining certificate for %s...", primary)

	// Generate new ECDSA P-256 key pair for the certificate (in-memory only)
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate cert key: %w", err)
	}

	request := certificate.ObtainRequest{
		Domains:    domains,
		Bundle:     true,
		PrivateKey: certKey,
	}

	certRes, err := c.legoClient.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("obtain certificate: %w", err)
	}

	// Marshal the cert private key to PEM
	keyDER, _ := x509.MarshalECPrivateKey(certKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	})

	cert, err := c.certStore.Upload(certstore.UploadRequest{
		CertPEM: certRes.Certificate,
		KeyPEM:  keyPEM,
		Source:  certstore.SourceLocalACME,
		Note:    fmt.Sprintf("ACME via lego for %s", strings.Join(domains, ", ")),
	})
	if err != nil {
		return nil, fmt.Errorf("store certificate: %w", err)
	}

	// Save cert files to acme live directory for reference (compat with old certbot paths)
	certDir := filepath.Join(c.dataDir, "acme", "live", sanitizeCertName(primary))
	os.MkdirAll(certDir, 0700)
	os.WriteFile(filepath.Join(certDir, "fullchain.pem"), certRes.Certificate, 0600)
	os.WriteFile(filepath.Join(certDir, "privkey.pem"), keyPEM, 0600)

	log.Printf("[acme] certificate obtained: %s → certstore:%s", primary, cert.ID)
	return &ObtainResult{CertID: cert.ID, Domains: domains}, nil
}

// Renew renews a certificate — implements certstore.ACMERenewer.
func (c *Client) Renew(ctx context.Context, domains []string) (string, error) {
	result, err := c.Obtain(ctx, domains)
	if err != nil {
		return "", err
	}
	return result.CertID, nil
}

func sanitizeCertName(domain string) string {
	return strings.ReplaceAll(domain, "*", "wildcard")
}

// ─── lego user implementation ───

type acmeUser struct {
	email string
	key   *ecdsa.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return nil }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey         { return u.key }
