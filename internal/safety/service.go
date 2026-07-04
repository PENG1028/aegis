package safety

import (
	"fmt"
	"net"
	"strconv"

	"aegis/internal/endpoint"
	gatewaylink "aegis/internal/gateway"
	"aegis/internal/listener"
	"aegis/internal/manageddomain"
	"aegis/internal/node"
	"aegis/internal/route"
)

// Dependencies holds all services needed by the safety checker.
type Dependencies struct {
	RouteRepo    *route.Repository
	MDRRepo      *manageddomain.Repository
	EndpointRepo *endpoint.Repository
	NodeRepo     *node.Repository
	GWLinkRepo   *gatewaylink.LinkRepository
	ListenerRepo *listener.Repository // v1.8A-4: listener-aware self-loop
}

// Service checks route safety and egress paths.
// v1.8A: Detection only. No enforcement.
type Service struct {
	deps Dependencies
}

// NewService creates a safety service.
func NewService(deps Dependencies) *Service {
	return &Service{deps: deps}
}

// CheckRouteSafety checks a single route for safety risks.
func (s *Service) CheckRouteSafety(routeID string) (*RouteSafetyResult, error) {
	if s.deps.RouteRepo == nil {
		return nil, fmt.Errorf("route repository not available")
	}
	rt, err := s.deps.RouteRepo.FindByID(routeID)
	if err != nil {
		return nil, fmt.Errorf("find route: %w", err)
	}
	if rt == nil {
		return nil, fmt.Errorf("route %s not found", routeID)
	}

	result := &RouteSafetyResult{
		RouteID:    rt.ID,
		Domain:     rt.Domain,
		TargetHost: "",
		TargetPort: 0,
	}

	// Resolve endpoint for target
	if s.deps.EndpointRepo != nil {
		endpoints, err := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
		if err == nil && len(endpoints) > 0 {
			host := NormalizeHost(endpoints[0].Address)
			result.TargetHost = host

			if h, pStr, err := net.SplitHostPort(endpoints[0].Address); err == nil {
				result.TargetHost = h
				if p, err := strconv.Atoi(pStr); err == nil {
					result.TargetPort = p
				}
			}

			selfIPs := s.getNodeIPs()
			class := ClassifyIP(result.TargetHost, selfIPs)
			result.IPClassification = string(class)
			result.IsCurrentNodeAddress = IsCurrentNodeAddress(result.TargetHost, selfIPs)
			result.IsGatewayListenerTarget = s.isGatewayListenerTarget(result.TargetHost, result.TargetPort)

			// Check GatewayLink
			result.GatewayLinkID = rt.GatewayLinkID
			result.HasGatewayLink = rt.GatewayLinkID != ""

			// Base classification risks
			switch class {
			case IPLoopback, IPPrivate:
				// safe — loopback/private targets are normal
			case IPPublic:
				if rt.GatewayLinkID == "" {
					result.Risks = append(result.Risks,
						Risk{Code: RiskPublicTargetEgress, Severity: SevWarning,
							Message: fmt.Sprintf("route %s targets public IP %s", rt.Domain, result.TargetHost)},
						Risk{Code: RiskGatewayLinkBypass, Severity: SevWarning,
							Message: fmt.Sprintf("route %s targets public IP %s without Gateway Link", rt.Domain, result.TargetHost)},
					)
					result.GatewayLinkRequired = true
					result.Recommendation = "attach a Gateway Link to authenticate this route"
				}
			}

			// Additional: listener-aware self-loop detection
			if result.IsGatewayListenerTarget {
				result.Risks = append(result.Risks, Risk{
					Code: RiskSelfLoop, Severity: SevError,
					Message: fmt.Sprintf("route target %s:%d matches a gateway listener — would cause self-loop", result.TargetHost, result.TargetPort),
				})
			}
		}
	}

	return result, nil
}

// CheckAllRoutesSafety checks all active routes.
func (s *Service) CheckAllRoutesSafety() ([]RouteSafetyResult, error) {
	if s.deps.RouteRepo == nil {
		return nil, fmt.Errorf("route repository not available")
	}
	routes, err := s.deps.RouteRepo.FindActive()
	if err != nil {
		return nil, fmt.Errorf("find active routes: %w", err)
	}
	var results []RouteSafetyResult
	for _, rt := range routes {
		r, err := s.CheckRouteSafety(rt.ID)
		if err != nil {
			continue
		}
		results = append(results, *r)
	}
	return results, nil
}

// TraceEgress traces the egress path for a domain.
func (s *Service) TraceEgress(domain, fromNodeID string) (*EgressTraceResult, error) {
	result := &EgressTraceResult{
		Domain:      domain,
		GatewayNode: fromNodeID,
		CurrentNode: s.getCurrentNodeID(),
	}

	// Step 1: Check if route exists
	var rt *route.Route
	var routeErr error
	if s.deps.RouteRepo != nil {
		rt, routeErr = s.deps.RouteRepo.FindByDomain(domain)
	}
	if routeErr == nil && rt != nil {
		result.MatchedRouteID = rt.ID
		safe, err := s.CheckRouteSafety(rt.ID)
		if err == nil {
			result.TargetHost = safe.TargetHost
			result.TargetPort = safe.TargetPort
			result.HasGatewayLink = safe.HasGatewayLink
			result.GatewayLinkID = safe.GatewayLinkID
			result.IPClassification = safe.IPClassification
			result.IsCurrentNodeAddress = safe.IsCurrentNodeAddress
			result.IsGatewayListenerTarget = safe.IsGatewayListenerTarget
			result.Risks = safe.Risks
		}
		return result, nil
	}

	// Step 2: Check if managed domain
	if s.deps.MDRRepo != nil {
		md, mdErr := s.deps.MDRRepo.FindByDomain(domain)
		if mdErr == nil && md != nil {
			result.IsManagedDomain = true
			result.IPClassification = string(IPHostname)
			return result, nil
		}
	}

	// Step 3: DNS resolution
	ips, err := net.LookupHost(domain)
	if err != nil || len(ips) == 0 {
		result.Risks = append(result.Risks, Risk{
			Code: RiskUnknownDomain, Severity: SevInfo,
			Message: fmt.Sprintf("domain %s does not resolve", domain),
		})
		result.Recommendation = "check if the domain is correct or register it as a managed domain"
		return result, nil
	}

	result.ResolvedIPs = ips
	selfIPs := s.getNodeIPs()
	firstIP := ips[0]
	class := ClassifyIP(firstIP, selfIPs)
	result.IPClassification = string(class)

	// DNS-only: check domain resolution risks
	switch class {
	case IPPublic:
		result.Risks = append(result.Risks, Risk{
			Code: RiskPublicDomainBounce, Severity: SevWarning,
			Message: fmt.Sprintf("%s resolves to public IP %s with no Aegis route", domain, firstIP),
		})
		result.Recommendation = fmt.Sprintf("bind %s using bind-http-domain to control egress", domain)
	}

	// Check if resolved domain points to this gateway
	if IsCurrentNodeAddress(firstIP, selfIPs) {
		result.IsCurrentNodeAddress = true
		result.Risks = append(result.Risks, Risk{
			Code: RiskDomainResolvesToSelf, Severity: SevError,
			Message: fmt.Sprintf("%s resolves to this gateway (%s)", domain, firstIP),
		})
	}

	return result, nil
}

// GetPlannerWarnings returns safety warnings for the Planner.
// v1.8A: Warning only, does NOT block apply.
func (s *Service) GetPlannerWarnings(domain, targetHost, gatewayLinkID string, externalNodeIPs ...string) []Risk {
	var risks []Risk

	nodeIPs := s.getNodeIPs()
	if len(externalNodeIPs) > 0 {
		nodeIPs = append(nodeIPs, externalNodeIPs...)
	}

	// Extract host and port from address (e.g., "127.0.0.1:3001" or "<SERVER_B_IP>:80")
	host := NormalizeHost(targetHost)
	port := 0
	if h, pStr, err := net.SplitHostPort(targetHost); err == nil {
		host = h
		if p, err := strconv.Atoi(pStr); err == nil {
			port = p
		}
	}

	class := ClassifyIP(host, nodeIPs)
	isNodeAddr := IsCurrentNodeAddress(host, nodeIPs)

	// Public target warnings (regardless of self-loop)
	if class == IPPublic {
		risks = append(risks, Risk{
			Code: RiskPublicTargetEgress, Severity: SevWarning,
			Message: fmt.Sprintf("route %s targets public IP %s", domain, host),
		})
		if gatewayLinkID == "" {
			risks = append(risks, Risk{
				Code: RiskGatewayLinkBypass, Severity: SevWarning,
				Message: fmt.Sprintf("route %s targets public IP %s without Gateway Link", domain, host),
			})
		}
	}

	// Listener-aware self-loop: only flag if target is a gateway listener port
	if isNodeAddr || class == IPLoopback {
		if s.isGatewayListenerPort(port) {
			risks = append(risks, Risk{
				Code: RiskSelfLoop, Severity: SevError,
				Message: fmt.Sprintf("route %s targets a gateway listener (%s:%d) — would cause self-loop", domain, host, port),
			})
		}
	}

	return risks
}

// isGatewayListenerTarget checks if host+port points to a gateway listener.
func (s *Service) isGatewayListenerTarget(host string, port int) bool {
	if port <= 0 {
		return false
	}
	nodeIPs := s.getNodeIPs()
	class := ClassifyIP(host, nodeIPs)
	isNodeAddr := IsCurrentNodeAddress(host, nodeIPs)

	// Must be a loopback or current node address
	if class != IPLoopback && !isNodeAddr {
		return false
	}

	return s.isGatewayListenerPort(port)
}

// isGatewayListenerPort checks if the port is a registered gateway listener.
func (s *Service) isGatewayListenerPort(port int) bool {
	if port <= 0 {
		return false
	}
	ports := s.getGatewayListenerPorts()
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

// getGatewayListenerPorts returns the set of ports that the gateway listens on.
func (s *Service) getGatewayListenerPorts() []int {
	if s.deps.ListenerRepo != nil {
		listeners, err := s.deps.ListenerRepo.FindAll()
		if err == nil && len(listeners) > 0 {
			seen := make(map[int]bool)
			for _, l := range listeners {
				if l.Status == "active" || l.Status == "planned" {
					seen[l.Port] = true
				}
			}
			ports := make([]int, 0, len(seen))
			for p := range seen {
				ports = append(ports, p)
			}
			return ports
		}
	}

	// Fallback: standard gateway ports
	return []int{80, 443, 8443}
}

// getNodeIPs collects the current node's known IPs.
func (s *Service) getNodeIPs() []string {
	if s.deps.NodeRepo == nil {
		return nil
	}
	cur, err := s.deps.NodeRepo.FindCurrent()
	if err != nil || cur == nil {
		return nil
	}
	var ips []string
	if cur.LocalIP != "" {
		ips = append(ips, cur.LocalIP)
	}
	if cur.PrivateIP != "" {
		ips = append(ips, cur.PrivateIP)
	}
	if cur.PublicIP != "" {
		ips = append(ips, cur.PublicIP)
	}
	return ips
}

// getCurrentNodeID returns the current node's ID.
func (s *Service) getCurrentNodeID() string {
	if s.deps.NodeRepo == nil {
		return ""
	}
	cur, err := s.deps.NodeRepo.FindCurrent()
	if err != nil || cur == nil {
		return ""
	}
	return cur.NodeID
}
