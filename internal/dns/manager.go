package dns

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager coordinates the DNS resolver, reachability checker, and server lifecycle.
type Manager struct {
	Resolver      *Resolver
	Server        *Server
	Reachability  *Reachability

	cfgListen   string
	cfgUpstream string
	refreshSec  int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu     sync.Mutex
	active bool
}

// NewManager creates a DNS manager.
// Does not start anything until Enable() is called.
func NewManager(
	routeRepo RouteRepo,
	serviceRepo ServiceRepo,
	endpointRepo EndpointRepo,
	nodeRepo NodeRepo,
	listenAddr, upstream string,
	refreshSec int,
) *Manager {
	// Create reachability checker
	reach := NewReachability(nodeRepo, time.Duration(refreshSec)*time.Second)

	// Create resolver
	resolver := NewResolver(routeRepo, serviceRepo, endpointRepo, nodeRepo, reach)

	// Create server (not started yet)
	server := NewServer(resolver, listenAddr, upstream)

	// Set current node ID on reachability
	if curr, err := nodeRepo.FindCurrent(); err == nil && curr != nil {
		reach.SetCurrentNodeID(curr.NodeID)
	}

	return &Manager{
		Resolver:     resolver,
		Server:       server,
		Reachability: reach,
		cfgListen:    listenAddr,
		cfgUpstream:  upstream,
		refreshSec:   refreshSec,
	}
}

// Enable starts the DNS server, reachability checker, and table refresh loop.
func (m *Manager) Enable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.active {
		return nil // already running
	}

	m.ctx, m.cancel = context.WithCancel(context.Background())

	// 1. Initial resolver table build
	if err := m.Resolver.Refresh(); err != nil {
		log.Printf("[dns] initial refresh failed: %v", err)
	}

	// 2. Start reachability checker (background)
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.Reachability.Start(m.ctx)
	}()

	// 3. Start periodic table refresh
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		ticker := time.NewTicker(time.Duration(m.refreshSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				// Re-read current node
				// (nodeRepo reachability checker already refreshes itself)
				if err := m.Resolver.Refresh(); err != nil {
					log.Printf("[dns] periodic refresh failed: %v", err)
				}
			}
		}
	}()

	// 4. Start DNS server
	if err := m.Server.Start(); err != nil {
		m.cancel()
		m.wg.Wait()
		return fmt.Errorf("start dns server: %w", err)
	}

	m.active = true
	log.Printf("[dns] manager: enabled (listen=%s, upstream=%s)", m.cfgListen, m.cfgUpstream)
	return nil
}

// Disable stops the DNS server and background goroutines.
func (m *Manager) Disable() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return nil
	}

	// Stop DNS server first
	if err := m.Server.Stop(); err != nil {
		log.Printf("[dns] server stop error: %v", err)
	}

	// Cancel context to stop background goroutines
	m.cancel()
	m.wg.Wait()

	m.active = false
	log.Printf("[dns] manager: disabled")
	return nil
}

// IsActive returns whether the DNS server is currently running.
func (m *Manager) IsActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}
