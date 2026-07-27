package dns

import (
	"fmt"
	"log"
	"sync"

	"aegis/internal/endpoint"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/service"
)

// ─── Repository interfaces ───

// RouteRepo defines route queries needed by the DNS resolver.
type RouteRepo interface {
	FindActive() ([]route.Route, error)
}

// ServiceRepo defines service queries needed.
type ServiceRepo interface {
	FindByID(id string) (*service.Service, error)
}

// EndpointRepo defines endpoint queries needed.
type EndpointRepo interface {
	FindEnabledByServiceID(serviceID string) ([]endpoint.Endpoint, error)
}

// NodeRepo defines node queries needed.
type NodeRepo interface {
	FindCurrent() (*node.NodeRecord, error)
	FindAll() ([]node.NodeRecord, error)
	FindByNodeID(nodeID string) (*node.NodeRecord, error)
}

// ─── Peer reachability checker interface (injectable) ───

type ReachabilityChecker interface {
	IsReachable(nodeID string) bool
}

// AllowlistChecker is an optional interface for the egress gateway.
// When set, allowlisted domains are skipped from internal resolution,
// so they resolve via upstream DNS instead.
type AllowlistChecker interface {
	IsAllowlisted(domain string) bool
	Refresh() error
}

// ─── ResolvedEntry ───

// ResolvedEntry holds the DNS resolution result for one domain.
type ResolvedEntry struct {
	Domain     string `json:"domain"`
	TargetIP   string `json:"target_ip"`   // the IP to return
	TargetNode string `json:"target_node"` // target node_id
	NodeIP     string `json:"node_ip"`     // node's private IP (if available)
	PublicIP   string `json:"public_ip"`   // node's public IP
	IsLocal    bool   `json:"is_local"`    // target is this machine
	RouteID    string `json:"route_id"`
	ServiceID  string `json:"service_id"`
	EndpointID string `json:"endpoint_id"`
}

// ─── Resolver ───

// Resolver builds and serves the domain → IP lookup table.
type Resolver struct {
	routeRepo    RouteRepo
	serviceRepo  ServiceRepo
	endpointRepo EndpointRepo
	nodeRepo     NodeRepo
	reachability ReachabilityChecker

	// AllowlistChecker is used to check if a domain should bypass
	// internal resolution (egress gateway allow list).
	allowlistChecker AllowlistChecker

	currentNodeID string

	mu    sync.RWMutex
	table map[string]ResolvedEntry // domain → entry
}

// NewResolver creates a DNS resolver.
func NewResolver(
	routeRepo RouteRepo,
	serviceRepo ServiceRepo,
	endpointRepo EndpointRepo,
	nodeRepo NodeRepo,
	reachability ReachabilityChecker,
) *Resolver {
	return &Resolver{
		routeRepo:      routeRepo,
		serviceRepo:    serviceRepo,
		endpointRepo:   endpointRepo,
		nodeRepo:       nodeRepo,
		reachability:   reachability,
		table:          make(map[string]ResolvedEntry),
	}
}

// SetAllowlistChecker injects an egress allowlist checker.
// Call before Refresh().
func (r *Resolver) SetAllowlistChecker(ac AllowlistChecker) {
	r.allowlistChecker = ac
}

// Lookup returns the resolved entry for a domain, or nil if not managed.
func (r *Resolver) Lookup(domain string) *ResolvedEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.table[domain]
	if !ok {
		return nil
	}
	return &entry
}

// Table returns a copy of the current resolution table.
func (r *Resolver) Table() map[string]ResolvedEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cp := make(map[string]ResolvedEntry, len(r.table))
	for k, v := range r.table {
		cp[k] = v
	}
	return cp
}

// Refresh rebuilds the resolution table from current database state.
func (r *Resolver) Refresh() error {
	// 1. Get current node
	currentNode, err := r.nodeRepo.FindCurrent()
	if err != nil {
		return fmt.Errorf("dns: find current node: %w", err)
	}
	if currentNode != nil {
		r.currentNodeID = currentNode.NodeID
	}

	// 2. Get all nodes for lookups
	allNodes, err := r.nodeRepo.FindAll()
	if err != nil {
		return fmt.Errorf("dns: find all nodes: %w", err)
	}
	nodeByID := make(map[string]*node.NodeRecord, len(allNodes))
	for i := range allNodes {
		nodeByID[allNodes[i].NodeID] = &allNodes[i]
	}

	// 3. Get all active routes
	routes, err := r.routeRepo.FindActive()
	if err != nil {
		return fmt.Errorf("dns: find active routes: %w", err)
	}

	newTable := make(map[string]ResolvedEntry, len(routes))

	for _, rt := range routes {
		// Skip routes with path_prefix — DNS only covers domain-level routes
		// Path-based routing is handled at the proxy level (Caddy)
		if rt.PathPrefix != "" {
			continue
		}

		// 3.5 Skip allowlisted domains (egress gateway 重名保护)
		// These domains resolve via upstream DNS instead of internally.
		if r.allowlistChecker != nil {
			if r.allowlistChecker.IsAllowlisted(rt.Domain) {
				continue
			}
		}

		// 4. Get service for this route
		svc, err := r.serviceRepo.FindByID(rt.ServiceID)
		if err != nil || svc == nil {
			continue
		}

		// 5. Get enabled endpoints
		eps, err := r.endpointRepo.FindEnabledByServiceID(rt.ServiceID)
		if err != nil || len(eps) == 0 {
			continue
		}

		// 6. Pick the best endpoint (prefer local type, then matching node_id)
		ep := pickBestEndpoint(eps, r.currentNodeID)
		if ep == nil {
			continue
		}

		targetHost, _ := ep.HostPort()

		// 7. Determine target node
		var targetNode *node.NodeRecord
		if ep.NodeID != "" {
			targetNode = nodeByID[ep.NodeID]
		}
		if targetNode == nil {
			targetNode = findNodeByHost(targetHost, allNodes)
		}
		if targetNode == nil {
			continue
		}

		// 8. Determine the best IP to return
		targetIP := resolveBestIP(targetNode, r.currentNodeID, r.reachability)

		newTable[rt.Domain] = ResolvedEntry{
			Domain:     rt.Domain,
			TargetIP:   targetIP,
			TargetNode: targetNode.NodeID,
			NodeIP:     targetNode.PrivateIP,
			PublicIP:   targetNode.PublicIP,
			IsLocal:    targetNode.NodeID == r.currentNodeID,
			RouteID:    rt.ID,
			ServiceID:  svc.ID,
			EndpointID: ep.ID,
		}
	}

	r.mu.Lock()
	r.table = newTable
	r.mu.Unlock()

	log.Printf("[dns] resolver: rebuilt table with %d entries", len(newTable))
	return nil
}

// resolveBestIP picks the most appropriate IP for a target node.
func resolveBestIP(targetNode *node.NodeRecord, currentNodeID string, reachability ReachabilityChecker) string {
	// Same node → loopback
	if currentNodeID != "" && targetNode.NodeID == currentNodeID {
		if targetNode.LocalIP != "" {
			return targetNode.LocalIP
		}
		return "127.0.0.1"
	}

	// Different node: try private IP first if reachable
	if targetNode.PrivateIP != "" {
		if reachability == nil || reachability.IsReachable(targetNode.NodeID) {
			return targetNode.PrivateIP
		}
	}

	// Fallback to public IP
	if targetNode.PublicIP != "" {
		return targetNode.PublicIP
	}

	// Last resort
	return targetNode.PrivateIP
}

// pickBestEndpoint returns the best endpoint for DNS resolution
// (prefer local type, then one matching node_id).
func pickBestEndpoint(eps []endpoint.Endpoint, currentNodeID string) *endpoint.Endpoint {
	if len(eps) == 0 {
		return nil
	}

	// Prefer endpoint matching current node
	for i := range eps {
		if eps[i].NodeID != "" && eps[i].NodeID == currentNodeID {
			return &eps[i]
		}
	}

	// Then prefer local type
	for i := range eps {
		if eps[i].Type == "local" {
			return &eps[i]
		}
	}

	// Fallback to first enabled
	return &eps[0]
}

// findNodeByHost searches all nodes for one whose IP matches.
func findNodeByHost(host string, allNodes []node.NodeRecord) *node.NodeRecord {
	for i := range allNodes {
		n := &allNodes[i]
		if n.LocalIP == host || n.PrivateIP == host || n.PublicIP == host {
			return n
		}
	}
	return nil
}
