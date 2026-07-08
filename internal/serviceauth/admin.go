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

	// Layer 2: private IPv4/IPv6 ranges.
	if ip.IsPrivate() {
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

	keys, ok := allKeys[callerName]
	if !ok || len(keys) == 0 {
		return "", ErrServiceNotFound
	}
	var verifyErr error
	for _, pubKey := range keys {
		_, verifyErr = VerifyTicket(ticketStr, pubKey)
		if verifyErr == nil {
			return callerName, nil
		}
	}
	return "", fmt.Errorf("verify ticket: %w", verifyErr)
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
