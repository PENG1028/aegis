package credential

import (
	"context"
	"fmt"
	"time"

	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/secrets"
	"aegis/internal/addr"
)

// Service manages encrypted connection string credentials.
type Service struct {
	repo     *Repository
	masterKey *secrets.MasterKey
	logSvc   logs.Logger
}

// NewService creates a new credential service.
func NewService(repo *Repository, masterKey *secrets.MasterKey, logSvc logs.Logger) *Service {
	return &Service{repo: repo, masterKey: masterKey, logSvc: logSvc}
}

// CreateResult is returned by Create — includes the raw connection string once.
type CreateResult struct {
	Credential Credential `json:"credential"`
}

// Create encrypts and stores a new connection string.
// The raw connection string is returned ONCE in the result — caller should
// not log or store it further.
func (s *Service) Create(ctx context.Context, alias, rawConnString, description string) (*CreateResult, error) {
	if alias == "" {
		return nil, fmt.Errorf("alias is required")
	}
	if rawConnString == "" {
		return nil, fmt.Errorf("connection string is required")
	}

	// Check alias uniqueness
	existing, err := s.repo.FindByAlias(alias)
	if err != nil {
		return nil, fmt.Errorf("check alias: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("alias %q already exists", alias)
	}

	// Parse the URI to detect scheme and generate masked version
	info, err := addr.ParseConnString(rawConnString)
	if err != nil {
		return nil, fmt.Errorf("invalid connection URI: %w", err)
	}

	// Encrypt the connection string
	encrypted, err := secrets.EncryptToStorage(s.masterKey, rawConnString)
	if err != nil {
		return nil, fmt.Errorf("encrypt connection string: %w", err)
	}

	now := time.Now()
	c := &Credential{
		ID:                  id.New("cred"),
		Alias:               alias,
		EncryptedConnString: encrypted,
		SecretVersion:       0,
		SecretCreatedAt:     now.Format(time.RFC3339),
		Scheme:              info.Scheme,
		MaskedURI:           info.MaskPassword(),
		Description:         description,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	if err := s.repo.Create(c); err != nil {
		s.logSvc.Log(ctx, "credential.create", "credential", c.ID, "failed", err.Error(), "api")
		return nil, err
	}

	s.logSvc.Log(ctx, "credential.create", "credential", c.ID, "success",
		fmt.Sprintf("created credential alias=%q scheme=%s version=%d", alias, info.Scheme, 0), "api")
	return &CreateResult{Credential: *c}, nil
}

// DecryptAndResolve decrypts a credential and returns the parsed connection info.
// This is the runtime path — called when starting a TCP proxy, generating config, etc.
func (s *Service) DecryptAndResolve(ctx context.Context, alias string) (*addr.ConnInfo, error) {
	c, err := s.repo.FindByAlias(alias)
	if err != nil {
		return nil, fmt.Errorf("lookup credential %q: %w", alias, err)
	}
	if c == nil {
		return nil, fmt.Errorf("credential %q not found", alias)
	}

	raw, err := secrets.DecryptFromStorage(s.masterKey, c.EncryptedConnString)
	if err != nil {
		return nil, fmt.Errorf("decrypt credential %q: %w", alias, err)
	}

	info, err := addr.ParseConnString(raw)
	if err != nil {
		return nil, fmt.Errorf("parse decrypted connection string for %q: %w", alias, err)
	}

	return info, nil
}

// GetByID returns a credential by ID (encrypted fields included for display).
func (s *Service) GetByID(ctx context.Context, id string) (*Credential, error) {
	return s.repo.FindByID(id)
}

// GetByAlias returns a credential by alias.
func (s *Service) GetByAlias(ctx context.Context, alias string) (*Credential, error) {
	return s.repo.FindByAlias(alias)
}

// List returns all credentials. Never includes raw connection strings.
func (s *Service) List(ctx context.Context) ([]Credential, error) {
	list, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	// Sanitize: clear encrypted fields from list response
	for i := range list {
		list[i].EncryptedConnString = ""
	}
	return list, nil
}

// Rotate re-encrypts a credential with a new connection string and increments version.
// The new raw connection string is returned ONCE.
func (s *Service) Rotate(ctx context.Context, idOrAlias string, newRawConnString string) (*Credential, string, error) {
	var c *Credential
	var err error

	// Try ID first, then alias
	c, err = s.repo.FindByID(idOrAlias)
	if err != nil {
		return nil, "", fmt.Errorf("lookup credential: %w", err)
	}
	if c == nil {
		c, err = s.repo.FindByAlias(idOrAlias)
		if err != nil {
			return nil, "", fmt.Errorf("lookup credential: %w", err)
		}
		if c == nil {
			return nil, "", fmt.Errorf("credential %q not found", idOrAlias)
		}
	}

	// Parse and validate new URI
	info, err := addr.ParseConnString(newRawConnString)
	if err != nil {
		return nil, "", fmt.Errorf("invalid new connection URI: %w", err)
	}

	// Encrypt new connection string
	encrypted, err := secrets.EncryptToStorage(s.masterKey, newRawConnString)
	if err != nil {
		return nil, "", fmt.Errorf("encrypt new connection string: %w", err)
	}

	c.EncryptedConnString = encrypted
	c.SecretVersion++
	c.SecretRotatedAt = time.Now().Format(time.RFC3339)
	c.Scheme = info.Scheme
	c.MaskedURI = info.MaskPassword()
	c.UpdatedAt = time.Now()

	if err := s.repo.Update(c); err != nil {
		return nil, "", fmt.Errorf("rotate credential: %w", err)
	}

	s.logSvc.Log(ctx, "credential.rotate", "credential", c.ID, "success",
		fmt.Sprintf("rotated credential alias=%q version=%d", c.Alias, c.SecretVersion), "api")
	return c, newRawConnString, nil
}

// RevealRaw decrypts and returns the raw connection string once.
// Audit-logged — caller must be authenticated.
func (s *Service) RevealRaw(ctx context.Context, id string) (string, error) {
	c, err := s.repo.FindByID(id)
	if err != nil {
		return "", fmt.Errorf("lookup credential: %w", err)
	}
	if c == nil {
		return "", fmt.Errorf("credential %q not found", id)
	}

	raw, err := secrets.DecryptFromStorage(s.masterKey, c.EncryptedConnString)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	s.logSvc.Log(ctx, "credential.reveal", "credential", c.ID, "warning",
		fmt.Sprintf("raw credential revealed for alias=%q version=%d", c.Alias, c.SecretVersion), "api")
	return raw, nil
}

// Delete removes a credential.
func (s *Service) Delete(ctx context.Context, id string) error {
	existing, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("credential %q not found", id)
	}

	if err := s.repo.Delete(id); err != nil {
		return err
	}

	s.logSvc.Log(ctx, "credential.delete", "credential", id, "success",
		fmt.Sprintf("deleted credential alias=%q", existing.Alias), "api")
	return nil
}
