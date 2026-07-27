package tcp

import (
	"aegis/internal/safety"

	"fmt"
	"strings"
	"sync"
)

// Manager manages all active TCP proxies.
type Manager struct {
	proxies map[string]*Proxy
	mu      sync.RWMutex
}

// NewManager creates a new TCP proxy manager.
func NewManager() *Manager {
	return &Manager{
		proxies: make(map[string]*Proxy),
	}
}

// StartProxy starts a TCP proxy for the given exposure.
func (m *Manager) StartProxy(id, entryHost string, entryPort int, targetHost string, targetPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check port conflict
	for _, existing := range m.proxies {
		if existing.EntryHost == entryHost && existing.EntryPort == entryPort && existing.IsRunning() {
			return fmt.Errorf("port conflict: %s:%d already in use by proxy %s", entryHost, entryPort, existing.ID)
		}
	}

	// Safety: prevent binding to 0.0.0.0 unless explicitly allowed
	if entryHost == "0.0.0.0" || entryHost == "::" {
		return fmt.Errorf("binding to %s is not allowed for safety reasons", entryHost)
	}

	proxy := NewProxy(id, entryHost, entryPort, targetHost, targetPort)
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("start tcp proxy %s: %w", id, err)
	}

	m.proxies[id] = proxy
	return nil
}

// StopProxy stops and removes a TCP proxy.
func (m *Manager) StopProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, ok := m.proxies[id]
	if !ok {
		return fmt.Errorf("proxy %s not found", id)
	}

	if err := proxy.Stop(); err != nil {
		return err
	}

	delete(m.proxies, id)
	return nil
}

// ReloadProxies synchronizes the manager's proxies with a desired set of active TCP exposures.
// Starts new proxies not already running, stops proxies no longer wanted.
func (m *Manager) ReloadProxies(desired []ProxyConfig) ([]string, []string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var started, stopped []string
	desiredMap := make(map[string]ProxyConfig)

	for _, d := range desired {
		desiredMap[d.ID] = d
	}

	// Stop proxies no longer in desired set
	for id, proxy := range m.proxies {
		if _, ok := desiredMap[id]; !ok {
			proxy.Stop()
			delete(m.proxies, id)
			stopped = append(stopped, id)
		}
	}

	// Start new / update existing proxies
	for _, d := range desired {
		existing, ok := m.proxies[d.ID]
		if ok {
			// Check if config changed
			if existing.EntryHost != d.EntryHost || existing.EntryPort != d.EntryPort ||
				existing.Target != nil && (existing.Target.Host != d.TargetHost || existing.Target.Port != d.TargetPort) {
				existing.Stop()
				delete(m.proxies, d.ID)
				ok = false
			}
		}

		if !ok {
			if err := m.startProxyLocked(d.ID, d.EntryHost, d.EntryPort, d.TargetHost, d.TargetPort); err != nil {
				return started, stopped, fmt.Errorf("start proxy %s: %w", d.ID, err)
			}
			started = append(started, d.ID)
		}
	}

	return started, stopped, nil
}

func (m *Manager) startProxyLocked(id, entryHost string, entryPort int, targetHost string, targetPort int) error {
	// Port conflict check
	for _, existing := range m.proxies {
		if existing.EntryHost == entryHost && existing.EntryPort == entryPort && existing.IsRunning() {
			return fmt.Errorf("port conflict: %s:%d", entryHost, entryPort)
		}
	}

	if entryHost == "0.0.0.0" || entryHost == "::" {
		return fmt.Errorf("0.0.0.0/:: binding not allowed")
	}

	proxy := NewProxy(id, entryHost, entryPort, targetHost, targetPort)
	if err := proxy.Start(); err != nil {
		return err
	}
	m.proxies[id] = proxy
	return nil
}

// ListProxies returns all active proxies.
func (m *Manager) ListProxies() []*Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Proxy
	for _, p := range m.proxies {
		result = append(result, p)
	}
	return result
}

// GetProxy returns a proxy by ID.
func (m *Manager) GetProxy(id string) *Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxies[id]
}

// ProxyCount returns the number of active proxies.
func (m *Manager) ProxyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.proxies)
}

// Shutdown stops all proxies gracefully.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.proxies {
		p.Stop()
	}
	m.proxies = make(map[string]*Proxy)
}

// ProxyConfig is the desired state for a TCP proxy from the apply planner.
type ProxyConfig struct {
	ID         string
	EntryHost  string
	EntryPort  int
	TargetHost string
	TargetPort int
}

// ValidateEntryBind validates that an entry bind address is safe.
// Returns true if the bind is allowed without admin override.
func ValidateEntryBind(host string) (bool, string) {
	if host == "" {
		return false, "entry host is required"
	}
	if host == "0.0.0.0" || host == "::" {
		return false, "binding to 0.0.0.0/:: is not allowed"
	}
	if strings.HasPrefix(host, "127.") || host == "localhost" {
		return true, "loopback allowed"
	}
	if safety.IsPrivateOrLinkLocal(host) {
		return true, "private address allowed"
	}
	return false, "public IP binding requires admin authorization"
}
