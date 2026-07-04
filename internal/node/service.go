package node

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"aegis/internal/core"
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
	publicIP := detectPublicIP()

	now := time.Now()
	nodeID := fmt.Sprintf("node_%s", hostname)

	// Check existing current node
	existing, err := s.repo.FindCurrent()
	if err != nil {
		return nil, fmt.Errorf("find current node: %w", err)
	}

	if existing != nil {
		// Preserve existing public IP if detection fails.
		// On cloud VPS (e.g. Tencent), the public IP is NAT'd and
		// not visible on any local interface; external detection
		// services may be unreachable.  An operator may have set
		// public_ip manually via the API or SQL.
		if publicIP == "" && existing.PublicIP != "" {
			publicIP = existing.PublicIP
		}
		// Preserve existing network_id — it is set manually and
		// cannot be auto-detected.
		networkID := existing.NetworkID

		// Check for IP migration
		ipChanged := existing.LocalIP != localIP || existing.PrivateIP != privateIP || existing.PublicIP != publicIP
		existing.Hostname = hostname
		existing.LocalIP = localIP
		existing.PrivateIP = privateIP
		existing.PublicIP = publicIP
		existing.NetworkID = networkID
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
		ID:           core.NewID("node"),
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
	nodeID := fmt.Sprintf("nd_%s", core.GenerateRandomHex(4))

	n := &NodeRecord{
		ID:           core.NewID("node"),
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

// detectPublicIP tries to discover the machine's public IPv4 address by
// querying external IP-detection services.  Each service gets a short
// timeout; the first successful response is returned.
//
// Returns "" if all services are unreachable.  In that case the caller
// should fall back to the value already stored in the database (set
// manually by the operator) or leave it empty.
func detectPublicIP() string {
	services := []string{
		"http://myip.ipip.net",        // Chinese CDN, fast in Asia
		"http://ip.sb",                // anycast
		"http://checkip.amazonaws.com", // AWS global
		"http://ident.me",             // global
		"http://ifconfig.me",          // global
		"http://icanhazip.com",        // global
	}

	client := &http.Client{Timeout: 3 * time.Second}

	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
		resp.Body.Close()
		if err != nil {
			continue
		}
		// Some services return extra text (e.g. "当前 IP：x.x.x.x"),
		// so extract the first plausible IPv4 address.
		ip := extractIPv4(string(body))
		if ip != "" {
			return ip
		}
	}
	return ""
}

// extractIPv4 scans a string and returns the first IPv4 address found.
func extractIPv4(s string) string {
	// Split on common delimiters and look for an IPv4 pattern.
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "：", ":") // fullwidth colon (Chinese)
	for _, field := range strings.Fields(s) {
		field = strings.TrimSpace(field)
		field = strings.TrimRight(field, ".,;:：")
		if ip := net.ParseIP(field); ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}
	// Last resort: walk through the raw string character by character
	// looking for an IPv4-like pattern.
	return firstIPv4InString(s)
}

// firstIPv4InString does a naive scan for a dotted-quad IPv4 address.
func firstIPv4InString(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) < 4 {
		return ""
	}
	for i := 0; i <= len(parts)-4; i++ {
		candidate := strings.TrimSpace(parts[i]) + "." +
			strings.TrimSpace(parts[i+1]) + "." +
			strings.TrimSpace(parts[i+2]) + "." +
			strings.TrimSpace(parts[i+3])
		// Strip leading/trailing non-digit characters.
		candidate = strings.TrimLeft(candidate, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ:： \t")
		candidate = strings.TrimRight(candidate, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ:： \t")
		if ip := net.ParseIP(candidate); ip != nil && ip.To4() != nil {
			return ip.String()
		}
	}
	return ""
}
