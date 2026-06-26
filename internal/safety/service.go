package safety

import (
	"fmt"
	"net"
	"strings"

	"aegis/internal/endpoint"
	gatewaylink "aegis/internal/gateway_link"
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
	GWLinkRepo   *gatewaylink.Repository
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
	endpoints, err := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
	if err == nil && len(endpoints) > 0 {
		host := NormalizeHost(endpoints[0].Address)
		result.TargetHost = host

		if h, p, err := net.SplitHostPort(endpoints[0].Address); err == nil {
			result.TargetHost = h
			fmt.Sscanf(p, "%d", &result.TargetPort)
		}

		// Classify target
		selfIPs := s.getNodeIPs()
		class := ClassifyIP(result.TargetHost, selfIPs)
		result.IPClassification = string(class)

		// Check GatewayLink
		result.GatewayLinkID = rt.GatewayLinkID
		result.HasGatewayLink = rt.GatewayLinkID != ""

		switch class {
		case IPLoopback, IPPrivate:
			// safe
		case IPSelf:
			result.Risks = append(result.Risks, Risk{
				Code: RiskSelfLoop, Severity: SevError,
				Message: fmt.Sprintf("route target %s is the gateway itself — would cause loop", result.TargetHost),
			})
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
	}

	return result, nil
}

// CheckAllRoutesSafety checks all active routes.
func (s *Service) CheckAllRoutesSafety() ([]RouteSafetyResult, error) {
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
// If the domain has a route, delegates to route safety.
// If not, performs DNS-based classification.
func (s *Service) TraceEgress(domain, fromNodeID string) (*EgressTraceResult, error) {
	result := &EgressTraceResult{
		Domain:      domain,
		GatewayNode: fromNodeID,
		CurrentNode: s.getCurrentNodeID(),
	}

	// Step 1: Check if route exists
	rt, err := s.deps.RouteRepo.FindByDomain(domain)
	if err == nil && rt != nil {
		result.MatchedRouteID = rt.ID
		safe, err := s.CheckRouteSafety(rt.ID)
		if err == nil {
			result.TargetHost = safe.TargetHost
			result.TargetPort = safe.TargetPort
			result.HasGatewayLink = safe.HasGatewayLink
			result.GatewayLinkID = safe.GatewayLinkID
			result.IPClassification = safe.IPClassification
			result.Risks = safe.Risks
		}
		return result, nil
	}

	// Step 2: Check if managed domain
	md, err := s.deps.MDRRepo.FindByDomain(domain)
	if err == nil && md != nil {
		result.IsManagedDomain = true
		result.IPClassification = string(IPHostname)
		return result, nil
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

	switch class {
	case IPSelf:
		result.Risks = append(result.Risks, Risk{
			Code: RiskDomainResolvesToSelf, Severity: SevError,
			Message: fmt.Sprintf("%s resolves to this gateway (%s)", domain, firstIP),
		})
	case IPPublic:
		result.Risks = append(result.Risks, Risk{
			Code: RiskPublicDomainBounce, Severity: SevWarning,
			Message: fmt.Sprintf("%s resolves to public IP %s with no Aegis route", domain, firstIP),
		})
		result.Recommendation = fmt.Sprintf("bind %s using bind-http-domain to control egress", domain)
	}

	return result, nil
}

// GetPlannerWarnings returns safety warnings for the Planner.
// v1.8A: Warning only, does NOT block apply.
func (s *Service) GetPlannerWarnings(domain, targetHost, gatewayLinkID string, nodeIPs []string) []Risk {
	var risks []Risk
	class := ClassifyIP(NormalizeHost(targetHost), nodeIPs)

	switch class {
	case IPSelf:
		risks = append(risks, Risk{
			Code: RiskSelfLoop, Severity: SevError,
			Message: fmt.Sprintf("route %s targets the gateway itself (%s)", domain, targetHost),
		})
	case IPPublic:
		if gatewayLinkID == "" {
			risks = append(risks, Risk{
				Code: RiskGatewayLinkBypass, Severity: SevWarning,
				Message: fmt.Sprintf("route %s targets public IP %s without Gateway Link", domain, targetHost),
			})
		}
	}
	return risks
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

// Ensure strings.Contains is used somewhere
var _ = strings.Contains
