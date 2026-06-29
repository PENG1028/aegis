package transparent

import (
	"fmt"
	"log"
	"net"
	"sync"
)

// Manager manages transparent interception rules + proxy lifecycle.
// Coordinates iptables DNAT rules with local transparent proxy instances.
type Manager struct {
	iptables   *iptablesManager
	proxies    map[string]*TransparentProxy // keyed by rule ID
	rulesByID  map[string]RedirectRule      // full rule state for ListStatus
	mu         sync.Mutex

	currentNodeID string // set via SetCurrentNodeID, used to decide local vs cross-node

	portStart int
	portEnd   int
	nextPort  int
}

// NewManager creates a transparent proxy manager.
// Call SetCurrentNodeID before StartRedirect to enable cross-node routing.
func NewManager() *Manager {
	return &Manager{
		iptables:  newIPTablesManager(),
		proxies:   make(map[string]*TransparentProxy),
		rulesByID: make(map[string]RedirectRule),
		portStart: 18100,
		portEnd:   18199,
		nextPort:  18100,
	}
}

// SetCurrentNodeID sets the local node identifier. Used to determine whether
// a redirect target is local (forward directly to backend) or remote
// (forward via Caddy :80 with gateway link routing).
func (m *Manager) SetCurrentNodeID(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentNodeID = nodeID
}

// StartRedirect begins transparent interception for the given rule:
//  1. Allocates a local port
//  2. Starts a transparent proxy on that port
//  3. Installs an iptables DNAT rule: original_ip:port → 127.0.0.1:local_port
//
// For same-node targets: proxy forwards directly to 127.0.0.1:original_port
// For cross-node targets: proxy forwards to 127.0.0.1:80 (Caddy), which
// routes via the matching domain-based route + gateway link.
func (m *Manager) StartRedirect(rule RedirectRule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.proxies[rule.ID]; ok {
		return fmt.Errorf("redirect rule %s already active", rule.ID)
	}

	// Allocate local port if not explicitly set
	if rule.LocalProxyPort == 0 {
		port, err := m.allocatePortLocked()
		if err != nil {
			return fmt.Errorf("allocate port: %w", err)
		}
		rule.LocalProxyPort = port
	}

	// Determine target based on whether the service is local or remote.
	// Same node: forward directly to 127.0.0.1:backend_port.
	// Cross node: forward to 127.0.0.1:80 (Caddy routes by domain via gateway link).
	targetHost := "127.0.0.1"
	targetPort := rule.OriginalPort // default: same port, local machine
	isCrossNode := m.currentNodeID != "" && rule.TargetNodeID != "" &&
		m.currentNodeID != rule.TargetNodeID

	if isCrossNode {
		// Cross-node: forward to Caddy on :80. Caddy must have a route
		// matching a domain that resolves to this service.
		targetPort = 80
		log.Printf("[transparent] %s: cross-node → forwarding via Caddy :80", rule.ID)
	}

	proxy := NewProxy(ProxyConfig{
		ID:         rule.ID,
		ListenAddr: fmt.Sprintf("127.0.0.1:%d", rule.LocalProxyPort),
		TargetHost: targetHost,
		TargetPort: targetPort,
	})
	if err := proxy.Start(); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}

	// Install iptables rule
	if err := m.iptables.addRule(rule); err != nil {
		proxy.Stop()
		return fmt.Errorf("iptables: %w", err)
	}

	m.proxies[rule.ID] = proxy
	m.rulesByID[rule.ID] = rule
	log.Printf("[transparent] redirect active: %s:%d → 127.0.0.1:%d → %s:%d (cross=%v)",
		rule.OriginalIP, rule.OriginalPort, rule.LocalProxyPort,
		targetHost, targetPort, isCrossNode)
	return nil
}

// StopRedirect stops interception and removes the iptables rule.
func (m *Manager) StopRedirect(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, ok := m.proxies[ruleID]
	if !ok {
		return fmt.Errorf("redirect rule %s not found", ruleID)
	}

	// Remove iptables rule first — stop redirecting new connections
	if err := m.iptables.removeRule(ruleID); err != nil {
		log.Printf("[transparent] iptables remove %s: %v (continuing)", ruleID, err)
	}

	proxy.Stop()
	delete(m.proxies, ruleID)
	delete(m.rulesByID, ruleID)
	return nil
}

// ListStatus returns full status for all active redirect rules.
func (m *Manager) ListStatus() []RuleStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]RuleStatus, 0, len(m.proxies))
	for id, proxy := range m.proxies {
		rule, ok := m.rulesByID[id]
		if !ok {
			rule = RedirectRule{ID: id}
		}
		_, bytesIn, bytesOut := proxy.Stats()
		result = append(result, RuleStatus{
			Rule:      rule,
			Active:    true,
			ProxyPort: rule.LocalProxyPort,
			BytesIn:   bytesIn,
			BytesOut:  bytesOut,
		})
	}
	return result
}

// RuleCount returns the number of active redirect rules.
func (m *Manager) RuleCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.proxies)
}

// CleanupStaleRules scans for iptables rules from previous Aegis instances
// (e.g., after a crash) and removes them. Call this during startup before
// adding new rules.
func (m *Manager) CleanupStaleRules() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rules, err := m.iptables.listRules()
	if err != nil {
		return fmt.Errorf("list iptables rules: %w", err)
	}

	if len(rules) == 0 {
		return nil
	}

	log.Printf("[transparent] found %d stale iptables rules from previous run, cleaning up...", len(rules))

	for _, ruleLine := range rules {
		log.Printf("[transparent] stale rule: %s", ruleLine)
	}
	m.iptables.cleanupAll()
	return nil
}

// Shutdown stops all redirects and cleans up iptables rules.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, proxy := range m.proxies {
		proxy.Stop()
		delete(m.proxies, id)
	}
	m.rulesByID = make(map[string]RedirectRule)
	m.iptables.cleanupAll()
	log.Printf("[transparent] manager shutdown complete")
}

// ─── Endpoint discovery ───────────────────────────────────────────────

// DiscoverTargets converts endpoint info into redirect rules for all
// non-loopback endpoints. The caller filters by node to decide which
// endpoints to intercept.
func (m *Manager) DiscoverTargets(endpoints []EndpointInfo) []RedirectRule {
	var rules []RedirectRule
	for _, ep := range endpoints {
		if ep.Host == "" || ep.Port == 0 {
			continue
		}
		// Skip loopback — no need to intercept localhost traffic
		if ep.Host == "127.0.0.1" || ep.Host == "localhost" || ep.Host == "::1" {
			continue
		}
		rules = append(rules, RedirectRule{
			ID:              fmt.Sprintf("ep-%s", ep.EndpointID),
			OriginalIP:      ep.Host,
			OriginalPort:    ep.Port,
			TargetServiceID: ep.ServiceID,
			TargetNodeID:    ep.NodeID,
			Description:     fmt.Sprintf("endpoint %s (%s:%d)", ep.EndpointID, ep.Host, ep.Port),
		})
	}
	return rules
}

// EndpointInfo is a simplified endpoint for discovery.
type EndpointInfo struct {
	EndpointID string
	ServiceID  string
	Host       string
	Port       int
	NodeID     string
}

// ─── Port allocation ───────────────────────────────────────────────────

func (m *Manager) allocatePortLocked() (int, error) {
	for range m.portEnd - m.portStart + 1 {
		port := m.nextPort
		m.nextPort++
		if m.nextPort > m.portEnd {
			m.nextPort = m.portStart
		}

		addr := fmt.Sprintf("127.0.0.1:%d", port)
		inUse := false
		for _, p := range m.proxies {
			if p.ListenAddr() == addr {
				inUse = true
				break
			}
		}
		if inUse {
			continue
		}

		ln, err := net.Listen("tcp", addr)
		if err == nil {
			ln.Close()
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free ports in %d-%d", m.portStart, m.portEnd)
}
