package listener

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// Service provides listener conflict detection and management.
type Service struct {
	repo         *Repository
	edgeMuxMode  bool
}

// NewService creates a new listener service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// SetEdgeMuxMode enables/disables EdgeMux mode (changes default listeners).
func (s *Service) SetEdgeMuxMode(enabled bool) {
	s.edgeMuxMode = enabled
}

// IsEdgeMuxMode returns whether EdgeMux mode is active.
func (s *Service) IsEdgeMuxMode() bool {
	return s.edgeMuxMode
}

// CheckConflict checks if a bind would conflict with existing listeners.
func (s *Service) CheckConflict(provider, protocol, bindIP string, port int) error {
	// In EdgeMux mode, port 443 belongs to the SNI pre-read provider (HAProxy).
	// Only that provider may bind it. Read the provider ID + port from listener
	// defaults rather than hardcoding.
	if s.edgeMuxMode {
		for _, l := range EdgeMuxDefaults() {
			if l.Port == port && l.Purpose == "public_tls_mux" && provider != l.Provider {
				return &ConflictError{
					ExistingListener: l,
					RequestedBind:    bindIP,
					RequestedPort:    port,
				}
			}
		}
	}

	existing, err := s.repo.FindByBind(bindIP, port)
	if err != nil {
		return err
	}
	if existing != nil {
		return &ConflictError{
			ExistingListener: *existing,
			RequestedBind:    bindIP,
			RequestedPort:    port,
		}
	}
	return nil
}

// RegisterDefaults registers the appropriate default listeners.
func (s *Service) RegisterDefaults() error {
	var defaults []Listener
	if s.edgeMuxMode {
		defaults = EdgeMuxDefaults()
	} else {
		defaults = DefaultListeners()
	}
	for _, d := range defaults {
		existing, _ := s.repo.FindByBind(d.BindIP, d.Port)
		if existing == nil {
			if err := s.repo.Create(&d); err != nil {
				return fmt.Errorf("register listener %s: %w", d.ID, err)
			}
		}
	}
	return nil
}

// RegisterDefaultListeners is the legacy API (non-EdgeMux).
func (s *Service) RegisterDefaultListeners() error {
	if s.edgeMuxMode {
		return s.RegisterDefaults()
	}
	return s.RegisterDefaults()
}

// ListAll returns all registered listeners.
func (s *Service) ListAll() ([]Listener, error) {
	listeners, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	if listeners == nil {
		listeners = []Listener{}
	}
	return listeners, nil
}

// CheckOSPorts checks actual OS port usage and returns unmanaged listeners.
func (s *Service) CheckOSPorts() ([]OSPortUsage, error) {
	var results []OSPortUsage

	// Use netstat or ss to check actual port usage
	out, err := exec.Command("ss", "-tlnp").Output()
	if err != nil {
		out, err = exec.Command("netstat", "-tlnp").Output()
		if err != nil {
			return nil, nil // OS check not available — skip
		}
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		addr := fields[3]
		host, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			continue
		}

		// Check if aegis knows about this port
		existing, _ := s.repo.FindByBind(host, port)
		if existing == nil {
			results = append(results, OSPortUsage{
				BindIP:  host,
				Port:    port,
				Status:  "unmanaged",
				Message: fmt.Sprintf("port in use by unknown process: %s", line),
			})
		}
	}
	return results, nil
}

// OSPortUsage represents an actual OS port binding.
type OSPortUsage struct {
	BindIP  string
	Port    int
	Status  string
	Message string
}
