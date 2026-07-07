package serviceauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

// ============================================================================
// Block / Unblock — admin actions
// ============================================================================

// BlockService blocks an entire service by its record ID.
func (s *Service) BlockService(ctx context.Context, id, reason string) error {
	rec, err := s.deps.Repo.FindByID(id)
	if err != nil || rec == nil {
		return ErrServiceNotFound
	}

	if err := s.deps.Repo.UpdateStatus(id, "blocked"); err != nil {
		return fmt.Errorf("block service: %w", err)
	}

	ver := s.blVersion.Add(1)
	entry := &BlocklistEntry{
		ID:        s.deps.IDGen(),
		ServiceID: rec.Name, // store service name so SDK's isBlocked can match by name
		APIName:   "*",
		Reason:    reason,
		Version:   ver,
	}
	if err := s.deps.Repo.AddBlock(entry); err != nil {
		return fmt.Errorf("block service: add entry: %w", err)
	}

	return nil
}

// BlockAPI blocks a specific API of a service.
func (s *Service) BlockAPI(ctx context.Context, serviceID, apiName, reason string) error {
	rec, err := s.deps.Repo.FindByID(serviceID)
	if err != nil || rec == nil {
		return ErrServiceNotFound
	}

	ver := s.blVersion.Add(1)
	entry := &BlocklistEntry{
		ID:        s.deps.IDGen(),
		ServiceID: rec.Name, // store service name so SDK's isBlocked can match
		APIName:   apiName,
		Reason:    reason,
		Version:   ver,
	}
	if err := s.deps.Repo.AddBlock(entry); err != nil {
		return fmt.Errorf("block api: %w", err)
	}

	return nil
}

// Unblock removes a blocklist entry.
func (s *Service) Unblock(ctx context.Context, blockID string) error {
	if err := s.deps.Repo.RemoveBlock(blockID); err != nil {
		return fmt.Errorf("unblock: %w", err)
	}
	s.blVersion.Add(1)
	s.catVersion.Add(1)
	return nil
}

// ============================================================================
// Queries — admin UI data
// ============================================================================

// ListServices returns all registered services.
func (s *Service) ListServices(ctx context.Context) ([]ServiceRecord, error) {
	return s.deps.Repo.ListAll()
}

// GetService returns a single service by ID.
func (s *Service) GetService(ctx context.Context, id string) (*ServiceRecord, error) {
	rec, err := s.deps.Repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, ErrServiceNotFound
	}
	return rec, nil
}

// GetTopology returns the service call topology.
func (s *Service) GetTopology(ctx context.Context, window time.Duration) (*TopologyData, error) {
	if window <= 0 {
		window = 1 * time.Hour
	}
	since := time.Now().Add(-window)

	edges, err := s.deps.Repo.TopologyEdges(since)
	if err != nil {
		return nil, err
	}

	allActive, _ := s.deps.Repo.ListActive()
	nodes := make([]TopologyNode, 0, len(allActive))
	for _, svc := range allActive {
		nodes = append(nodes, TopologyNode{
			Name:     svc.Name,
			Host:     svc.Host,
			Port:     svc.Port,
			NodeHost: svc.NodeHost,
			Status:   svc.Status,
		})
	}

	return &TopologyData{Nodes: nodes, Edges: edges}, nil
}

// GetCallLogs returns recent call records.
func (s *Service) GetCallLogs(ctx context.Context, since time.Time, limit int) ([]CallLog, error) {
	return s.deps.Repo.QueryCallLogs(since, limit)
}

// ============================================================================
// Cluster membership
// ============================================================================

// isInCluster returns true when clientIP is considered part of the trusted
// cluster. The check is layered:
//
//  1. localhost — always trusted
//  2. Private IP ranges (10.x, 172.16-31.x, 192.168.x) — trusted
//  3. NodeChecker — delegates to the injected implementation
//     (CIDR whitelist in standalone, node table in Aegis)
func (s *Service) isInCluster(clientIP string) bool {
	if clientIP == "" {
		return false
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// Layer 1: localhost.
	if ip.IsLoopback() {
		return true
	}

	// Layer 2: private IPv4 ranges.
	if isPrivateIP(ip) {
		return true
	}

	// Layer 3: injected checker (CIDR whitelist or node table).
	if s.deps.NodeChecker != nil {
		if _, err := s.deps.NodeChecker.FindByIP(clientIP); err == nil {
			return true
		}
	}

	return false
}

// isPrivateIP returns true for RFC 1918 private IPv4 and RFC 4193 private IPv6.
func isPrivateIP(ip net.IP) bool {
	// IPv4 private ranges.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}
	// IPv6 private ranges: fd00::/8 (unique local) and fe80::/10 (link-local).
	if ip.IsPrivate() {
		return true
	}
	return false
}

// ============================================================================
// Input validation
// ============================================================================

const (
	maxServiceNameLen = 128
)

var reservedChars = []byte{':', '\n', '\r', '\x00'}

// validateRegisterRequest checks that all fields in a registration request are
// well-formed. Rejects empty names, forbidden characters (colons break ticket
// parsing), and excessive lengths.
func validateRegisterRequest(req RegisterRequest) error {
	if req.ServiceName == "" {
		return fmt.Errorf("%w: service_name is required", ErrInvalidInput)
	}
	if len(req.ServiceName) > maxServiceNameLen {
		return fmt.Errorf("%w: service_name too long (max %d)", ErrInvalidInput, maxServiceNameLen)
	}
	if containsReserved(req.ServiceName) {
		return fmt.Errorf("%w: service_name contains reserved characters", ErrInvalidInput)
	}
	if req.PublicKey == "" {
		return fmt.Errorf("%w: public_key is required", ErrInvalidInput)
	}
	return nil
}

func containsReserved(s string) bool {
	for i := 0; i < len(s); i++ {
		for _, c := range reservedChars {
			if s[i] == c {
				return true
			}
		}
	}
	return false
}


// ============================================================================
// Bridge: Ticket → ActionContext (for Aegis integration)
// ============================================================================

// VerifyTicketAndGetSpace validates a service ticket using Ed25519 and the
// caller's public key from the repository. Returns the caller's service name.
func (s *Service) VerifyTicketAndGetSpace(ticketStr string) (serviceName string, err error) {
	allKeys, keyErr := s.deps.Repo.ListPublicKeys()
	if keyErr != nil {
		return "", fmt.Errorf("verify ticket: lookup public keys: %w", keyErr)
	}
	ticketDecoded, decodeErr := base64.StdEncoding.DecodeString(ticketStr)
	if decodeErr != nil {
		return "", fmt.Errorf("verify ticket: %w", ErrTicketInvalid)
	}
	parts := strings.SplitN(string(ticketDecoded), ":", 3)
	if len(parts) < 1 {
		return "", fmt.Errorf("verify ticket: %w", ErrTicketInvalid)
	}
	callerName := parts[0]

	pubKey, ok := allKeys[callerName]
	if !ok {
		return "", ErrServiceNotFound
	}

	claims, verifyErr := VerifyTicket(ticketStr, pubKey)
	if verifyErr != nil {
		return "", fmt.Errorf("verify ticket: %w", verifyErr)
	}

	instances, _ := s.deps.Repo.FindByName(callerName)
	for _, inst := range instances {
		if inst.Status == "blocked" {
			return "", ErrServiceBlocked
		}
	}
	_ = claims
	return callerName, nil
}

// Rebind migrates a service identity to a new name with a fresh keypair.
// Admin-only operation. The old name immediately becomes invalid.
func (s *Service) Rebind(ctx context.Context, oldName, newName string) (*KeyPair, error) {
	instances, err := s.deps.Repo.FindByName(oldName)
	if err != nil || len(instances) == 0 {
		return nil, ErrServiceNotFound
	}

	// Generate new keypair for the new name.
	pubKey, privKey, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("rebind: generate keypair: %w", err)
	}

	// Update the existing record with new name and new public key.
	// Old public key is invalidated immediately.
	rec := &instances[0]
	rec.Name = newName
	rec.PublicKey = pubKey
	rec.UpdatedAt = time.Now()

	if err := s.deps.Repo.DeleteService(rec.ID); err != nil {
		return nil, fmt.Errorf("rebind: remove old: %w", err)
	}
	// Re-insert with new name.
	if err := s.deps.Repo.UpsertService(rec); err != nil {
		return nil, fmt.Errorf("rebind: insert new: %w", err)
	}

	s.catVersion.Add(1)

	return &KeyPair{
		PublicKey:  pubKey,
		PrivateKey: privKey,
	}, nil
}

// ─── Groups ───

func (s *Service) ListGroups(ctx context.Context) ([]ServiceGroup, error) {
	return s.deps.Repo.ListGroups()
}

func (s *Service) UpsertGroup(ctx context.Context, g *ServiceGroup) error {
	if g.ID == "" { g.ID = s.deps.IDGen() }
	if err := s.deps.Repo.UpsertGroup(g); err != nil { return err }
	s.catVersion.Add(1)
	return nil
}

func (s *Service) DeleteGroup(ctx context.Context, id string) error {
	if err := s.deps.Repo.DeleteGroup(id); err != nil { return err }
	s.catVersion.Add(1)
	return nil
}

// ─── Policies ───

func (s *Service) ListPolicies(ctx context.Context) ([]Policy, error) {
	return s.deps.Repo.ListPolicies()
}

func (s *Service) UpsertPolicy(ctx context.Context, p *Policy) error {
	if p.ID == "" { p.ID = s.deps.IDGen() }
	if err := s.deps.Repo.UpsertPolicy(p); err != nil { return err }
	s.catVersion.Add(1)
	return nil
}

func (s *Service) DeletePolicy(ctx context.Context, id string) error {
	if err := s.deps.Repo.DeletePolicy(id); err != nil { return err }
	s.catVersion.Add(1)
	return nil
}

// ─── Policy Engine ───

// EvaluatePolicy checks whether a caller is allowed to perform an action on a target.
// Returns true if allowed, false if denied. Unmatched → defaultPolicy applies.
func EvaluatePolicy(callerName string, groups []ServiceGroup, policies []Policy, targetService, action, defaultPolicy string) bool {
	// Check if caller matches any policy subject.
	for _, p := range policies {
		if !matchSubject(callerName, p.Subject, groups) { continue }
		if p.TargetService != "*" && p.TargetService != targetService { continue }
		if p.Action != "*" && p.Action != action { continue }
		if p.Effect == "deny" { return false }
		return true
	}
	// No policy matched — apply default.
	return defaultPolicy == "allow"
}

func matchSubject(callerName, subject string, groups []ServiceGroup) bool {
	if subject == "*" { return true }
	if subject == callerName { return true }
	for _, g := range groups {
		if g.Name == subject {
			for _, m := range g.Members {
				if m == callerName { return true }
			}
		}
	}
	return false
}

// InGroup returns true if the service is a member of the named group.
func InGroup(serviceName string, groups []ServiceGroup, groupName string) bool {
	for _, g := range groups {
		if g.Name != groupName { continue }
		for _, m := range g.Members {
			if m == serviceName { return true }
		}
	}
	return false
}

// ListGroupMembers returns all members of the named group.
func ListGroupMembers(groups []ServiceGroup, groupName string) []string {
	for _, g := range groups {
		if g.Name == groupName { return g.Members }
	}
	return nil
}

// LookupServiceByName returns the first active instance of a named service.
func (s *Service) LookupServiceByName(ctx context.Context, name string) (*ServiceRecord, error) {
	instances, err := s.deps.Repo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, ErrServiceNotFound
	}
	return &instances[0], nil
}
