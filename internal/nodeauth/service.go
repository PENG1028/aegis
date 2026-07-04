package nodeauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"time"

	"aegis/internal/core"
	"aegis/internal/node"
)

// Service manages node registration and authentication.
type Service struct {
	repo     *Repository
	nodeRepo *node.Repository
	nodeSvc  *node.Service
}

// NewService creates a new node auth service.
func NewService(repo *Repository, nodeRepo *node.Repository, nodeSvc *node.Service) *Service {
	return &Service{
		repo:     repo,
		nodeRepo: nodeRepo,
		nodeSvc:  nodeSvc,
	}
}

// DefaultJoinTokenTTL is the default TTL for join tokens (1 hour).
const DefaultJoinTokenTTL = 3600

// TokenByteLength is the number of random bytes for token generation.
const TokenByteLength = 32

// ============================================================================
// Join Token Operations
// ============================================================================

// CreateJoinToken creates a new join token and returns the raw token value (once).
func (s *Service) CreateJoinToken(input CreateJoinTokenInput) (*JoinToken, string, error) {
	if input.ExpiresInSeconds <= 0 {
		input.ExpiresInSeconds = DefaultJoinTokenTTL
	}
	if input.AllowedRoles == nil {
		input.AllowedRoles = []string{}
	}

	rawToken := core.GenerateRandomHex(TokenByteLength)
	tokenHash := hashToken(rawToken)

	now := time.Now()
	t := &JoinToken{
		ID:                core.NewID("jt"),
		TokenHash:         tokenHash,
		Name:              input.Name,
		AllowedRoles:      input.AllowedRoles,
		ExpectedNodeName:  input.ExpectedNodeName,
		AllowedSourceCIDR: input.AllowedSourceCIDR,
		ExpiresAt:         now.Add(time.Duration(input.ExpiresInSeconds) * time.Second),
		CreatedAt:         now,
	}

	if err := s.repo.CreateJoinToken(t); err != nil {
		return nil, "", err
	}

	return t, rawToken, nil
}

// ValidateJoinToken checks if a raw join token is valid for registration.
// Returns the token record if valid, or an error describing why it's invalid.
func (s *Service) ValidateJoinToken(rawToken string, req JoinRequest, sourceIP string) (*JoinToken, error) {
	tokenHash := hashToken(rawToken)
	t, err := s.repo.FindJoinTokenByHash(tokenHash)
	if err != nil {
		return nil, fmt.Errorf("join token lookup failed: %w", err)
	}
	if t == nil {
		return nil, fmt.Errorf("join token not found")
	}

	if !t.IsValid() {
		if t.IsExpired() {
			return nil, fmt.Errorf("join token expired at %s", t.ExpiresAt.Format(time.RFC3339))
		}
		if t.IsUsed() {
			return nil, fmt.Errorf("join token already used")
		}
		if t.IsRevoked() {
			return nil, fmt.Errorf("join token has been revoked")
		}
		return nil, fmt.Errorf("join token is not valid")
	}

	// Check expected node name
	if t.ExpectedNodeName != "" && t.ExpectedNodeName != req.NodeName {
		return nil, fmt.Errorf("join token requires node_name '%s', got '%s'", t.ExpectedNodeName, req.NodeName)
	}

	// Check allowed roles
	if len(t.AllowedRoles) > 0 {
		allowed := false
		for _, allowedRole := range t.AllowedRoles {
			for _, reqRole := range req.Roles {
				if allowedRole == reqRole {
					allowed = true
					break
				}
			}
			if allowed {
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("join token does not allow any of the requested roles %v", req.Roles)
		}
	}

	// Check source CIDR
	if t.AllowedSourceCIDR != "" {
		if sourceIP == "" {
			return nil, fmt.Errorf("join token requires source IP matching %s, but source IP is unknown", t.AllowedSourceCIDR)
		}
		_, cidrNet, err := net.ParseCIDR(t.AllowedSourceCIDR)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed_source_cidr in token: %s", t.AllowedSourceCIDR)
		}
		clientIP := net.ParseIP(sourceIP)
		if clientIP == nil {
			return nil, fmt.Errorf("cannot parse source IP: %s", sourceIP)
		}
		if !cidrNet.Contains(clientIP) {
			return nil, fmt.Errorf("source IP %s is not in allowed CIDR %s", sourceIP, t.AllowedSourceCIDR)
		}
	}

	return t, nil
}

// ListJoinTokens returns all join tokens.
func (s *Service) ListJoinTokens() ([]JoinToken, error) {
	return s.repo.ListJoinTokens()
}

// GetJoinToken returns a join token by ID.
func (s *Service) GetJoinToken(id string) (*JoinToken, error) {
	return s.repo.FindJoinTokenByID(id)
}

// RevokeJoinToken revokes a join token.
func (s *Service) RevokeJoinToken(id string) error {
	t, err := s.repo.FindJoinTokenByID(id)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("join token not found")
	}
	if t.IsUsed() {
		return fmt.Errorf("cannot revoke a join token that has already been used")
	}
	return s.repo.RevokeJoinToken(id)
}

// ============================================================================
// Node Registration
// ============================================================================

// RegisterNode registers a new node using a valid join token.
// Returns the join response containing node_id and raw node token (once).
func (s *Service) RegisterNode(req JoinRequest, sourceIP string) (*JoinResponse, error) {
	// Validate join token
	jt, err := s.ValidateJoinToken(req.JoinToken, req, sourceIP)
	if err != nil {
		return nil, fmt.Errorf("join validation failed: %w", err)
	}

	// Create node record
	role := node.RoleWorker
	if len(req.Roles) > 0 {
		role = req.Roles[0]
	}
	// Store additional roles as part of the node in future (role_json). For now, use primary role.
	_ = req.Roles // Keep for future multi-role support

	n, err := s.nodeSvc.CreateNode(req.NodeName, role, req.Hostname, req.PublicIP, req.PrivateIP, req.OS, req.Arch, req.AgentVersion)
	if err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}

	// Generate node credential
	rawNodeToken := core.GenerateRandomHex(TokenByteLength)
	nodeTokenHash := hashToken(rawNodeToken)

	cred := &NodeCredential{
		ID:        core.NewID("nc"),
		NodeID:    n.NodeID,
		TokenHash: nodeTokenHash,
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateNodeCredential(cred); err != nil {
		return nil, fmt.Errorf("create node credential: %w", err)
	}

	// Mark join token as used
	if err := s.repo.MarkJoinTokenUsed(jt.ID, n.NodeID, time.Now()); err != nil {
		return nil, fmt.Errorf("mark join token used: %w", err)
	}

	return &JoinResponse{
		NodeID:           n.NodeID,
		NodeToken:        rawNodeToken,
		NodeTokenRedacted: false,
		Status:           "registered",
		HeartbeatAfter:   30,
	}, nil
}

// ============================================================================
// Node Authentication
// ============================================================================

// AuthenticateNode validates a node credential for node API access.
// Returns the node_id if authentication succeeds.
func (s *Service) AuthenticateNode(rawNodeToken string) (string, error) {
	tokenHash := hashToken(rawNodeToken)
	cred, err := s.repo.FindNodeCredentialByTokenHash(tokenHash)
	if err != nil {
		return "", fmt.Errorf("credential lookup failed: %w", err)
	}
	if cred == nil {
		return "", fmt.Errorf("invalid node credential")
	}
	if cred.IsRevoked() {
		return "", fmt.Errorf("node credential has been revoked")
	}

	// Update last used
	_ = s.repo.UpdateNodeCredentialLastUsed(cred.ID, time.Now())

	return cred.NodeID, nil
}

// ============================================================================
// Credential Management
// ============================================================================

// GetCredentialByNodeID returns the active credential for a node.
func (s *Service) GetCredentialByNodeID(nodeID string) (*NodeCredential, error) {
	return s.repo.FindNodeCredentialByNodeID(nodeID)
}

// RevokeNodeCredential revokes a specific credential by ID.
func (s *Service) RevokeNodeCredential(credID string) error {
	return s.repo.RevokeNodeCredential(credID)
}

// RevokeAllNodeCredentials revokes all credentials for a node.
func (s *Service) RevokeAllNodeCredentials(nodeID string) error {
	return s.repo.RevokeNodeCredentialsByNodeID(nodeID)
}

// ============================================================================
// Token Hashing
// ============================================================================

// hashToken returns the SHA-256 hex hash of a token.
// Uses HMAC-SHA256 with a static key for consistency.
// The HMAC key is a domain separator, not a secret — security relies on
// the 256-bit random token entropy.
func hashToken(token string) string {
	// Use HMAC-SHA256 with a static key for domain separation
	// This is NOT a pepper — the key is fixed. The security comes from
	// token entropy (256-bit random).
	h := hmac.New(sha256.New, []byte("aegis-node-token-v1"))
	h.Write([]byte(token))
	return hex.EncodeToString(h.Sum(nil))
}

// GetHashTokenForTesting exposes the hash function for test verification.
// Tests use this to verify that DB stores hash(raw) rather than raw.
func GetHashTokenForTesting(token string) string {
	return hashToken(token)
}
