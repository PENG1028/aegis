package node

import (
	"fmt"
	"net"
	"os"
	"time"

	"aegis/internal/id"
)

// Service manages node identity registration.
type Service struct {
	repo *Repository
}

// NewService creates a new node service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// RegisterCurrent registers the current machine as a node.
// If a node already exists, checks for IP migration.
func (s *Service) RegisterCurrent() (*NodeRecord, error) {
	hostname, _ := os.Hostname()
	localIP := "127.0.0.1"
	privateIP := detectPrivateIP()
	publicIP := "" // Set via config or external detection

	now := time.Now()
	nodeID := fmt.Sprintf("node_%s", hostname)

	// Check existing current node
	existing, err := s.repo.FindCurrent()
	if err != nil {
		return nil, fmt.Errorf("find current node: %w", err)
	}

	if existing != nil {
		// Check for IP migration
		ipChanged := existing.LocalIP != localIP || existing.PrivateIP != privateIP || existing.PublicIP != publicIP
		existing.Hostname = hostname
		existing.LocalIP = localIP
		existing.PrivateIP = privateIP
		existing.PublicIP = publicIP
		existing.LastSeen = now
		existing.UpdatedAt = now
		existing.Capabilities = DetectCapabilities()
		if ipChanged {
			existing.IPMigrated = true
		}
		if err := s.repo.Update(existing); err != nil {
			return nil, err
		}
		return existing, nil
	}

	// Create new node record
	n := &NodeRecord{
		ID:           id.New("node"),
		NodeID:       nodeID,
		Hostname:     hostname,
		LocalIP:      localIP,
		PrivateIP:    privateIP,
		PublicIP:     publicIP,
		IsCurrent:    true,
		Capabilities: DetectCapabilities(),
		LastSeen:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(n); err != nil {
		return nil, err
	}
	return n, nil
}

// GetCurrent returns the current node record.
func (s *Service) GetCurrent() (*NodeRecord, error) {
	return s.repo.FindCurrent()
}

// ListAll returns all known nodes.
func (s *Service) ListAll() ([]NodeRecord, error) {
	nodes, err := s.repo.FindAll()
	if err != nil { return nil, err }
	if nodes == nil { nodes = []NodeRecord{} }
	return nodes, nil
}

// detectPrivateIP finds the first non-loopback private IPv4 address.
func detectPrivateIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			if ipnet.IP.IsPrivate() {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}
