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
	"sync"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
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
	user       *acmeUser
	legoClient *lego.Client
	http01     *HTTPChallengeProvider
	mu         chan struct{} // capacity 1 → single concurrent obtain
	emailMu    sync.RWMutex
}

// NewClient creates an ACME client. The account key is loaded or generated
// from dataDir/acme/account.key. email is optional for LE registration.
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
		http01:     newHTTPChallengeProvider(),
		mu:         make(chan struct{}, 1),
	}

	if err := c.initLego(); err != nil {
		return nil, err
	}

	return c, nil
}

// Available returns true if the client can obtain certificates.
func (c *Client) Available() bool {
	return c.legoClient != nil
}

// HasEmail returns true if a registration email was configured.
// Used by diagnostics to warn about missing expiry notifications.
func (c *Client) HasEmail() bool {
	c.emailMu.RLock()
	defer c.emailMu.RUnlock()
	return c.email != ""
}

// UpdateEmail updates the local lego user and, when registered, the remote
// ACME account contact without racing an obtain or renewal operation.
func (c *Client) UpdateEmail(ctx context.Context, email string) error {
	select {
	case c.mu <- struct{}{}:
		defer func() { <-c.mu }()
	case <-ctx.Done():
		return ctx.Err()
	}
	c.emailMu.Lock()
	c.email = strings.TrimSpace(email)
	if c.user == nil {
		c.emailMu.Unlock()
		return nil
	}
	c.user.email = c.email
	c.emailMu.Unlock()
	if c.user.registration == nil || c.legoClient == nil {
		return nil
	}
	reg, err := c.legoClient.Registration.UpdateRegistration(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return fmt.Errorf("update ACME account contact: %w", err)
	}
	c.user.registration = reg
	return nil
}

func (c *Client) initLego() error {
	user := &acmeUser{email: c.email, key: c.accountKey}
	config := lego.NewConfig(user)
	config.CADirURL = c.acmeServer
	if config.CADirURL == "" {
		config.CADirURL = lego.LEDirectoryProduction
	}
	config.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("create lego client: %w", err)
	}

	// WHY: the gateway owns port 80 in production. Tokens are served through
	// Aegis' existing listener and a Planner-injected Caddy route.
	if err := client.Challenge.SetHTTP01Provider(c.http01); err != nil {
		return fmt.Errorf("set HTTP-01 provider: %w", err)
	}

	c.legoClient = client
	c.user = user
	// Account registration is lazy — done on first Obtain call
	return nil
}

// HTTPChallengeResponse returns the active key authorization for a lego token.
func (c *Client) HTTPChallengeResponse(token string) (string, bool) {
	if c == nil || c.http01 == nil {
		return "", false
	}
	return c.http01.Response(token)
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

	// Lazy account registration — only try when actually obtaining
	if err := c.ensureRegistered(); err != nil {
		return nil, fmt.Errorf("ACME account: %w", err)
	}

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

// RenewCertificate obtains fresh material and promotes it into the existing
// CertStore asset, preserving route references.
func (c *Client) RenewCertificate(ctx context.Context, certID string, domains []string) (string, error) {
	result, err := c.Obtain(ctx, domains)
	if err != nil {
		return "", err
	}
	if err := c.certStore.ReplaceWith(certID, result.CertID); err != nil {
		_ = c.certStore.Delete(result.CertID)
		return "", err
	}
	return certID, nil
}

func sanitizeCertName(domain string) string {
	return strings.ReplaceAll(domain, "*", "wildcard")
}

// ensureRegistered registers or recovers the ACME account if not already done.
func (c *Client) ensureRegistered() error {
	if c.user == nil {
		return fmt.Errorf("lego client not initialized")
	}
	// WHY: lego switches its JWS signer from an embedded JWK to the account
	// KID after registration. Calling newAccount again with that KID is invalid
	// at some CAs, so the successful registration is the process-local guard.
	if c.user.registration != nil {
		return nil
	}
	if c.legoClient == nil || c.legoClient.Registration == nil {
		return fmt.Errorf("lego client not initialized")
	}
	reg, err := c.legoClient.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return err
	}
	if reg == nil || reg.URI == "" {
		return fmt.Errorf("ACME registration returned no account URI")
	}
	c.user.registration = reg
	return nil
}

// ─── lego user implementation ───

type acmeUser struct {
	email        string
	key          *ecdsa.PrivateKey
	registration *registration.Resource
}

func (u *acmeUser) GetEmail() string                        { return u.email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }
