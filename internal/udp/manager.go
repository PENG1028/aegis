package udp

import (
	"fmt"
	"sync"
)

// Manager manages all active UDP proxies.
type Manager struct {
	proxies map[string]*Proxy
	mu      sync.RWMutex
}

// NewManager creates a new UDP proxy manager.
func NewManager() *Manager {
	return &Manager{
		proxies: make(map[string]*Proxy),
	}
}

// StartProxy starts a UDP proxy forwarding from entryHost:entryPort to targetHost:targetPort.
func (m *Manager) StartProxy(id, entryHost string, entryPort int, targetHost string, targetPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for existing proxy with same ID
	if existing, ok := m.proxies[id]; ok {
		if existing.IsRunning() {
			return fmt.Errorf("UDP proxy %s is already running", id)
		}
		// Remove dead proxy
		delete(m.proxies, id)
	}

	// Check port conflict
	for _, existing := range m.proxies {
		if existing.EntryHost == entryHost && existing.EntryPort == entryPort && existing.IsRunning() {
			return fmt.Errorf("UDP port conflict: %s:%d already in use by proxy %s", entryHost, entryPort, existing.ID)
		}
	}

	// Safety: prevent binding to 0.0.0.0
	if entryHost == "0.0.0.0" || entryHost == "::" {
		return fmt.Errorf("binding to %s is not allowed for safety", entryHost)
	}

	proxy := NewProxy(id, entryHost, entryPort, targetHost, targetPort)
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("start UDP proxy %s: %w", id, err)
	}

	m.proxies[id] = proxy
	return nil
}

// StopProxy stops and removes a UDP proxy.
func (m *Manager) StopProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, ok := m.proxies[id]
	if !ok {
		return fmt.Errorf("UDP proxy %s not found", id)
	}

	if err := proxy.Stop(); err != nil {
		return err
	}

	delete(m.proxies, id)
	return nil
}

// ListProxies returns all active UDP proxies.
func (m *Manager) ListProxies() []*Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Proxy
	for _, p := range m.proxies {
		result = append(result, p)
	}
	return result
}

// GetProxy returns a UDP proxy by ID.
func (m *Manager) GetProxy(id string) *Proxy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.proxies[id]
}

// Count returns the number of active UDP proxies.
func (m *Manager) Count() int {
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

// ProxyStatus holds a proxy's current status for API responses.
type ProxyStatus struct {
	ID         string `json:"id"`
	EntryHost  string `json:"entry_host"`
	EntryPort  int    `json:"entry_port"`
	Target     string `json:"target"`
	Running    bool   `json:"running"`
	Sessions   int    `json:"sessions"`
	PacketsIn  int64  `json:"packets_in"`
	PacketsOut int64  `json:"packets_out"`
	BytesIn    int64  `json:"bytes_in"`
	BytesOut   int64  `json:"bytes_out"`
}

func proxyStatus(p *Proxy) ProxyStatus {
	pi, po, bi, bo := p.Stats()
	return ProxyStatus{
		ID:         p.ID,
		EntryHost:  p.EntryHost,
		EntryPort:  p.EntryPort,
		Target:     p.Target.String(),
		Running:    p.IsRunning(),
		Sessions:   p.SessionCount(),
		PacketsIn:  pi,
		PacketsOut: po,
		BytesIn:    bi,
		BytesOut:   bo,
	}
}

// Status returns the status of a single proxy.
func (m *Manager) Status(id string) *ProxyStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p, ok := m.proxies[id]; ok {
		s := proxyStatus(p)
		return &s
	}
	return nil
}

// ListStatus returns status for all active UDP proxies.
func (m *Manager) ListStatus() []ProxyStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ProxyStatus, 0, len(m.proxies))
	for _, p := range m.proxies {
		result = append(result, proxyStatus(p))
	}
	return result
}

// ValidateEntryBind is in aegis/internal/safety package.
