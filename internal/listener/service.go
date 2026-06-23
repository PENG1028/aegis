package listener

import (
	"fmt"
)

// Service provides listener conflict detection and management.
type Service struct {
	repo *Repository
}

// NewService creates a new listener service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// CheckConflict checks if a bind would conflict with existing listeners.
// Caddy owns 80/443 by default. HAProxy TCP cannot bind to those ports
// unless it's in edge_mux mode (not implemented yet).
func (s *Service) CheckConflict(provider, protocol, bindIP string, port int) error {
	// Caddy 80/443 protection
	if provider != "caddy_http" && (port == 80 || port == 443) {
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
	}

	// General port conflict check
	existing, err := s.repo.FindByBind(bindIP, port)
	if err != nil {
		return err
	}
	if existing != nil && existing.ID != "" {
		return &ConflictError{
			ExistingListener: *existing,
			RequestedBind:    bindIP,
			RequestedPort:    port,
		}
	}

	return nil
}

// RegisterDefaultListeners ensures the standard Caddy listeners exist.
func (s *Service) RegisterDefaultListeners() error {
	defaults := DefaultListeners()
	for _, d := range defaults {
		existing, _ := s.repo.FindByBind(d.BindIP, d.Port)
		if existing == nil {
			if err := s.repo.Create(&d); err != nil {
				return fmt.Errorf("register default listener %s: %w", d.ID, err)
			}
		}
	}
	return nil
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
