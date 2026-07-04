package gateway

import (
	"fmt"
	"sort"

	"aegis/internal/endpoint"
	"aegis/internal/listener"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/service"
)

// Dependency interfaces.

// RouteRepo defines route queries needed by the resolver and handler.
type RouteRepo interface {
	FindByDomain(domain string) (*route.Route, error)
	FindByID(id string) (*route.Route, error)
}

// ServiceRepo defines service queries needed by the resolver.
type ServiceRepo interface {
	FindByID(id string) (*service.Service, error)
}

// EndpointRepo defines endpoint queries needed by the resolver.
type EndpointRepo interface {
	FindEnabledByServiceID(serviceID string) ([]endpoint.Endpoint, error)
}

// NodeRepo defines node queries needed by the resolver.
type NodeRepo interface {
	FindCurrent() (*node.NodeRecord, error)
	FindAll() ([]node.NodeRecord, error)
	FindByNodeID(nodeID string) (*node.NodeRecord, error)
}

// GWLinkRepo defines gateway link queries needed by the resolver.
type GWLinkRepo interface {
	FindByID(id string) (*TrustedGateway, error)
}

// ListenerRepo defines listener queries needed by the resolver.
type ListenerRepo interface {
	FindAll() ([]listener.Listener, error)
}

// Dependencies holds all repos/services needed by the Resolver.
type Dependencies struct {
	RouteRepo    RouteRepo
	ServiceRepo  ServiceRepo
	EndpointRepo EndpointRepo
	NodeRepo     NodeRepo
	GWLinkRepo   GWLinkRepo
	ListenerRepo ListenerRepo
}

// Resolver resolves the managed egress relay path for a domain.
type Resolver struct {
	deps Dependencies
}

// NewResolver creates a new relay resolver.
func NewResolver(deps Dependencies) *Resolver {
	return &Resolver{deps: deps}
}

// ResolveManagedRelay determines the relay mode and path for a domain from a given source node.
func (r *Resolver) ResolveManagedRelay(domain, fromNodeID string) *RelayResult {
	res := &RelayResult{
		Domain:   domain,
		FromNodeID: fromNodeID,
	}

	// 1. Look up route by domain
	rt, err := r.deps.RouteRepo.FindByDomain(domain)
	if err != nil || rt == nil || rt.Status != "active" {
		return externalPassthrough(res, domain, "no active route found for domain")
	}

	res.Managed = true
	res.RouteID = rt.ID

	// 2. Look up service
	svc, err := r.deps.ServiceRepo.FindByID(rt.ServiceID)
	if err != nil || svc == nil {
		return unavailable(res, "service not found",
			fmt.Sprintf("route %s references service %s which does not exist", rt.ID, rt.ServiceID))
	}
	if svc.Kind != "http" {
		return unavailable(res, "service kind not supported for relay",
			fmt.Sprintf("service %s is kind %s, only http is supported for relay in v1.8B", svc.ID, svc.Kind))
	}
	res.ServiceID = svc.ID

	// 3. Get enabled endpoints sorted by priority (local first)
	eps, err := r.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
	if err != nil || len(eps) == 0 {
		return unavailable(res, "no enabled endpoints",
			fmt.Sprintf("service %s has no enabled endpoints", svc.ID))
	}

	// 4. Find the best endpoint: prefer local → matches node_id → fallback to first
	ep := pickBestEndpoint(eps, fromNodeID)
	if ep == nil {
		return unavailable(res, "no suitable endpoint",
			"no enabled endpoint could be matched to any node")
	}
	res.EndpointID = ep.ID

	targetHost, targetPort := ep.HostPort()

	// 5. Determine target node from endpoint.node_id or fallback
	var targetNode *node.NodeRecord
	if ep.NodeID != "" {
		targetNode, err = r.deps.NodeRepo.FindByNodeID(ep.NodeID)
		if err != nil {
			targetNode = nil
		}
	}
	// If endpoint has no node_id, try to find node by matching IP
	if targetNode == nil {
		targetNode = findNodeByHost(r.deps.NodeRepo, targetHost)
	}

	if targetNode == nil {
		return unavailable(res, "target node not found",
			fmt.Sprintf("could not determine which node hosts endpoint %s (%s)", ep.ID, ep.Address))
	}
	res.TargetNodeID = targetNode.NodeID
	res.TargetNodeHostname = targetNode.Hostname

	// 6. Get from-node for comparison
	fromNode, err := findNodeByAny(r.deps.NodeRepo, fromNodeID)
	if err != nil || fromNode == nil {
		return unavailable(res, "from_node not found",
			fmt.Sprintf("source node %s does not exist", fromNodeID))
	}
	res.FromNodeHostname = fromNode.Hostname

	// 7. Get gateway link (if any)
	var gwLink *TrustedGateway
	if rt.GatewayLinkID != "" {
		gwLink, _ = r.deps.GWLinkRepo.FindByID(rt.GatewayLinkID)
		if gwLink != nil {
			res.GatewayLinkID = gwLink.ID
		}
	}

	// 8. Get listener ports for gateway URL construction
	listeners, _ := r.deps.ListenerRepo.FindAll()
	httpPort := getHTTPListenerPort(listeners)
	httpsPort := getHTTPSListenerPort(listeners)

	// 9. Determine mode
	fromNodeIDStr := fromNode.NodeID
	targetNodeIDStr := targetNode.NodeID

	// Same node → local_gateway
	if fromNodeIDStr == targetNodeIDStr {
		return localGateway(res, targetNode, targetHost, targetPort, httpPort, listeners)
	}

	// ===== Cross-Node Relay: GatewayLink Required =====
	// Private gateway: requires GatewayLink
	if targetNode.PrivateIP != "" {
		if gwLink == nil || gwLink.AuthValue == "" {
			res.AddRisk(RiskGatewayLinkRequired, "error",
				fmt.Sprintf("route %s targets node %s via private IP but has no GatewayLink", rt.ID, targetNodeIDStr))
			return unavailable(res, "GatewayLink required for private egress relay",
				fmt.Sprintf("route %s targets node %s via private IP %s, requires GatewayLink", rt.ID, targetNodeIDStr, targetNode.PrivateIP))
		}
		return privateGateway(res, targetNode, targetHost, targetPort, httpPort, gwLink)
	}

	// Public gateway: requires GatewayLink
	if targetNode.PublicIP != "" {
		if gwLink == nil || gwLink.AuthValue == "" {
			res.AddRisk(RiskGatewayLinkRequired, "error",
				fmt.Sprintf("route %s targets node %s via public IP but has no GatewayLink", rt.ID, targetNodeIDStr))
			return unavailable(res, "GatewayLink required for public egress relay",
				fmt.Sprintf("route %s targets node %s via public IP %s, requires GatewayLink", rt.ID, targetNodeIDStr, targetNode.PublicIP))
		}
		return publicGateway(res, targetNode, targetHost, targetPort, httpsPort, gwLink)
	}

	return unavailable(res, "target node has no reachable gateway",
		fmt.Sprintf("node %s has no private or public IP configured", targetNodeIDStr))
}

// externalPassthrough sets mode to external_passthrough.
func externalPassthrough(res *RelayResult, domain, reason string) *RelayResult {
	res.Managed = false
	res.Mode = string(ModeExternalPassthrough)
	res.DirectTargetSuppressed = false
	res.Recommendation = "domain is not managed by Aegis — relay not available"
	res.AddRisk("UNKNOWN_DOMAIN", "info", reason)
	return res
}

// unavailable sets mode to unavailable with error info.
func unavailable(res *RelayResult, err, detail string) *RelayResult {
	res.Managed = true
	res.Mode = string(ModeUnavailable)
	res.DirectTargetSuppressed = true
	res.Error = err
	res.ErrorDetail = detail
	return res
}

// localGateway sets mode to local_gateway.
func localGateway(res *RelayResult, node *node.NodeRecord, targetHost string, targetPort, httpPort int, listeners []listener.Listener) *RelayResult {
	res.Mode = string(ModeLocalGateway)
	res.DirectTargetSuppressed = true
	res.GatewayHost = "127.0.0.1"
	res.GatewayPort = httpPort
	res.GatewayURL = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	res.FinalLocalTarget = fmt.Sprintf("127.0.0.1:%d", targetPort)

	// Check for self-loop: if target is a listener port, that's a loop
	if isListenerPort(listeners, targetPort) {
		res.AddRisk(RiskSelfLoop, "error",
			fmt.Sprintf("local gateway target 127.0.0.1:%d is a gateway listener port — would cause loop", targetPort))
	}

	// Safety: note that we're accessing local service
	res.Recommendation = fmt.Sprintf("request routed through local gateway 127.0.0.1:%d to 127.0.0.1:%d", httpPort, targetPort)
	return res
}

// privateGateway sets mode to private_gateway.
// Caller must ensure gwLink is non-nil (checked before call).
func privateGateway(res *RelayResult, node *node.NodeRecord, targetHost string, targetPort, httpPort int, gwLink *TrustedGateway) *RelayResult {
	res.Mode = string(ModePrivateGateway)
	res.DirectTargetSuppressed = true
	res.GatewayHost = node.PrivateIP
	res.GatewayPort = httpPort
	res.GatewayURL = fmt.Sprintf("http://%s:%d", node.PrivateIP, httpPort)
	res.FinalLocalTarget = fmt.Sprintf("127.0.0.1:%d", targetPort)
	res.GatewayLinkID = gwLink.ID

	res.Recommendation = fmt.Sprintf("send request to private gateway %s:%d (target node %s) with GatewayLink auth", node.PrivateIP, httpPort, node.NodeID)
	return res
}

// publicGateway sets mode to public_gateway.
func publicGateway(res *RelayResult, node *node.NodeRecord, targetHost string, targetPort, httpsPort int, gwLink *TrustedGateway) *RelayResult {
	res.Mode = string(ModePublicGateway)
	res.DirectTargetSuppressed = true
	res.GatewayHost = node.PublicIP
	res.GatewayPort = httpsPort
	res.GatewayURL = fmt.Sprintf("http://%s:%d", node.PublicIP, httpsPort)
	res.FinalLocalTarget = fmt.Sprintf("127.0.0.1:%d", targetPort)

	res.AddRisk(RiskPublicTargetEgress, "info",
		fmt.Sprintf("relay to node %s traverses public network — GatewayLink provides authorization", node.NodeID))

	res.Recommendation = fmt.Sprintf("send request to public gateway %s:%d with GatewayLink auth", node.PublicIP, httpsPort)
	return res
}

// pickBestEndpoint returns the best endpoint: prefer the one matching fromNodeID, then local type.
func pickBestEndpoint(eps []endpoint.Endpoint, fromNodeID string) *endpoint.Endpoint {
	if len(eps) == 0 {
		return nil
	}

	// Sort: local first, then private, then public
	sorted := make([]endpoint.Endpoint, len(eps))
	copy(sorted, eps)
	sort.Slice(sorted, func(i, j int) bool {
		pi := endpointPriority(sorted[i].Type)
		pj := endpointPriority(sorted[j].Type)
		return pi < pj
	})

	// Prefer endpoint with matching node_id
	for i := range sorted {
		if sorted[i].NodeID != "" && (sorted[i].NodeID == fromNodeID) {
			return &sorted[i]
		}
	}

	// Fallback to first (local) endpoint
	return &sorted[0]
}

func endpointPriority(typ string) int {
	switch typ {
	case "local":
		return 0
	case "private":
		return 1
	case "public":
		return 2
	default:
		return 99
	}
}

// findNodeByHost tries to find a node by matching its IPs.
func findNodeByHost(repo NodeRepo, host string) *node.NodeRecord {
	all, err := repo.FindAll()
	if err != nil || len(all) == 0 {
		return nil
	}
	for i := range all {
		n := &all[i]
		if n.LocalIP == host || n.PrivateIP == host || n.PublicIP == host {
			return n
		}
	}
	return nil
}

// findNodeByAny finds a node by node_id.
// Currently delegates to NodeRepo.FindByNodeID (the only lookup method available).
// If a separate DB-ID → NodeID lookup is added later, extend this function.
// Do NOT duplicate the FindByNodeID call as a "fallback" — that's dead code.
func findNodeByAny(repo NodeRepo, id string) (*node.NodeRecord, error) {
	return repo.FindByNodeID(id)
}

// getHTTPListenerPort returns the first active HTTP listener port (default 80).
func getHTTPListenerPort(listeners []listener.Listener) int {
	for _, l := range listeners {
		if l.Status == "active" && l.Purpose == "public_http" {
			return l.Port
		}
	}
	return 80
}

// getHTTPSListenerPort returns the first active HTTPS/TLS listener port (default 443).
func getHTTPSListenerPort(listeners []listener.Listener) int {
	for _, l := range listeners {
		if l.Status == "active" && (l.Purpose == "public_tls_mux" || l.Port == 443) {
			return l.Port
		}
	}
	return 443
}

// isListenerPort checks if a port is a registered gateway listener.
func isListenerPort(listeners []listener.Listener, port int) bool {
	for _, l := range listeners {
		if l.Port == port && l.Status == "active" {
			return true
		}
	}
	return false
}
