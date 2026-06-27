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
		Name:         hostname,
		Role:         RoleWorker,
		Status:       StatusOnline,
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
	if err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []NodeRecord{}
	}
	return nodes, nil
}

// GetNode returns a node by node_id.
func (s *Service) GetNode(nodeID string) (*NodeRecord, error) {
	return s.repo.FindByNodeID(nodeID)
}

// CreateNode creates a new node record with the given parameters (v1.8C).
func (s *Service) CreateNode(name, role, hostname, publicIP, privateIP, osName, arch, agentVersion string) (*NodeRecord, error) {
	now := time.Now()
	nodeID := fmt.Sprintf("nd_%s", id.GenerateRandomHex(4))

	n := &NodeRecord{
		ID:           id.New("node"),
		NodeID:       nodeID,
		Name:         name,
		Role:         role,
		Status:       StatusUnknown,
		Hostname:     hostname,
		LocalIP:      "127.0.0.1",
		PrivateIP:    privateIP,
		PublicIP:     publicIP,
		OS:           osName,
		Arch:         arch,
		AgentVersion: agentVersion,
		Capabilities: DefaultCapabilities(),
		LastSeen:     now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.repo.Create(n); err != nil {
		return nil, fmt.Errorf("create node: %w", err)
	}
	return n, nil
}

// UpdateHeartbeat updates a node's heartbeat information.
func (s *Service) UpdateHeartbeat(nodeID, status, agentVersion, publicIP, privateIP, hostname, lastError string) error {
	now := time.Now()
	return s.repo.UpdateHeartbeat(nodeID, status, agentVersion, publicIP, privateIP, hostname, lastError, now)
}

// SetNodeStatus updates a node's operational status.
func (s *Service) SetNodeStatus(nodeID, status, lastError string) error {
	return s.repo.SetStatus(nodeID, status, lastError)
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
