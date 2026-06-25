package trace

import (
	"context"
	"fmt"
	"net"
	"time"

	"aegis/internal/edgemux"
	"aegis/internal/endpoint"
	"aegis/internal/gateway_link"
	"aegis/internal/listener"
	"aegis/internal/node"
	"aegis/internal/provider"
	"aegis/internal/route"
)

// Dependencies provides read-only access to all subsystems needed for tracing.
type Dependencies struct {
	RouteRepo    *route.Repository
	EdgeSvc      *edgemux.AppService
	ListenerSvc  *listener.Service
	NodeRepo     *node.Repository
	EndpointRepo *endpoint.Repository
	GatewayLinkRepo *gatewaylink.Repository // v1.7W: for target address lookup
}

// Service traces access paths for domains, SNI hosts, and routes.
// Read-only — never modifies state.
type Service struct {
	deps       Dependencies
	tcpTimeout time.Duration
}

// NewService creates a trace service.
func NewService(deps Dependencies) *Service {
	return &Service{
		deps:       deps,
		tcpTimeout: 2 * time.Second,
	}
}

// TraceDomain traces the full access path for a domain name.
func (s *Service) TraceDomain(ctx context.Context, domain string) *AccessPathTrace {
	t := &AccessPathTrace{
		Input:     domain,
		InputType: "domain",
	}
	steps := make([]TraceStep, 0, 8)

	// Step 1: Find matching route
	rt, err := s.deps.RouteRepo.FindByDomain(domain)
	if err != nil || rt == nil {
		t.TraceStatus = StatusNotFound
		t.Errors = append(t.Errors, fmt.Sprintf("no route found for domain '%s'", domain))
		steps = append(steps, TraceStep{
			Order: 1, Component: "route", Name: "route_lookup",
			Status: "missing", Detail: fmt.Sprintf("no route matching domain '%s'", domain),
		})
		t.Steps = steps
		return t
	}

	steps = append(steps, TraceStep{
		Order: 1, Component: "route", Name: "route_lookup",
		Status: "matched",
		Detail: fmt.Sprintf("route %s: domain=%s tls=%v status=%s", rt.ID, rt.Domain, rt.TLSEnabled, rt.Status),
	})

	// v1.7W: Look up target from route's service endpoint
	var finalTarget *TargetInfo
	if s.deps.EndpointRepo != nil {
		endpoints, epErr := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
		if epErr == nil && len(endpoints) > 0 {
			host, port := parseHostPort(endpoints[0].Address)
			finalTarget = &TargetInfo{
				Host:     host,
				Port:     port,
				Protocol: "http",
			}
			if rt.TLSEnabled {
				finalTarget.Protocol = "https"
			}
		}
	}

	// Step 2: Determine entry — EdgeMux or direct Caddy
	if rt.TLSEnabled {
		// HTTPS path: goes through EdgeMux (HAProxy on 443) → Caddy on 8443

		// Check for EdgeMux listener
		listeners, _ := s.deps.ListenerSvc.ListAll()
		hasEdge := false
		for _, l := range listeners {
			if l.Port == 443 {
				hasEdge = true
				steps = append(steps, TraceStep{
					Order: 2, Component: "listener", Name: "edge_listener",
					Status: "matched", Detail: fmt.Sprintf("port %d (%s) via %s", l.Port, l.Protocol, l.Provider),
					Address: fmt.Sprintf("0.0.0.0:%d", l.Port),
				})
				break
			}
		}
		if !hasEdge {
			steps = append(steps, TraceStep{
				Order: 2, Component: "listener", Name: "edge_listener",
				Status: "missing", Detail: "no port 443 listener (EdgeMux may not be configured)",
			})
			t.Warnings = append(t.Warnings, "TLS enabled but no EdgeMux listener on port 443")
		}

		// Step 3: Find EdgeMux rule (SNI match)
		edgeRule, edgeErr := s.deps.EdgeSvc.FindBySNIHost(ctx, domain)
		if edgeErr != nil || edgeRule == nil {
			steps = append(steps, TraceStep{
				Order: 3, Component: "edge_mux", Name: "sni_match",
				Status: "missing", Detail: fmt.Sprintf("no edge rule for SNI host '%s'", domain),
			})
			t.Warnings = append(t.Warnings, "TLS domain has no matching edge_mux_rule — SNI passthrough may fail")
			t.TraceStatus = StatusIncomplete
		} else {
			steps = append(steps, TraceStep{
				Order: 3, Component: "edge_mux", Name: "sni_match",
				Status: "matched",
				Detail:  fmt.Sprintf("edge rule %s: sni=%s → %s:%d", edgeRule.ID, edgeRule.SNIHost, edgeRule.TargetHost, edgeRule.TargetPort),
				Address: fmt.Sprintf("%s:%d", edgeRule.TargetHost, edgeRule.TargetPort),
			})
		}

		// Step 4: Caddy step
		steps = append(steps, TraceStep{
			Order: 4, Component: "caddy", Name: "tls_termination",
			Status: "matched", Detail: "Caddy handles TLS termination on 127.0.0.1:8443",
			Address: "127.0.0.1:8443",
		})
	} else {
		// HTTP path: direct to Caddy on port 80
		listeners, _ := s.deps.ListenerSvc.ListAll()
		hasHTTP := false
		for _, l := range listeners {
			if l.Port == 80 {
				hasHTTP = true
				steps = append(steps, TraceStep{
					Order: 2, Component: "listener", Name: "http_listener",
					Status: "matched", Detail: fmt.Sprintf("port %d (%s) via %s", l.Port, l.Protocol, l.Provider),
					Address: fmt.Sprintf("0.0.0.0:%d", l.Port),
				})
				break
			}
		}
		if !hasHTTP {
			steps = append(steps, TraceStep{
				Order: 2, Component: "listener", Name: "http_listener",
				Status: "missing", Detail: "no port 80 listener",
			})
			t.Warnings = append(t.Warnings, "no HTTP listener on port 80")
		}

		steps = append(steps, TraceStep{
			Order: 3, Component: "caddy", Name: "http_direct",
			Status: "matched", Detail: "Caddy handles HTTP directly on port 80",
		})
	}

	// Step 5: Route target resolution
	steps = append(steps, TraceStep{
		Order: 5, Component: "route", Name: "route_detail",
		Status: "matched",
		Detail: fmt.Sprintf("route %s: service_id=%s status=%s link=%s", rt.ID, rt.ServiceID, rt.Status, rt.GatewayLinkID),
	})

	// v1.7W: Check target connectivity
	if finalTarget != nil {
		s.checkTargetConnectivity(finalTarget)
		t.FinalTarget = finalTarget
		if finalTarget.Reachable != nil && !*finalTarget.Reachable {
			steps = append(steps, TraceStep{
				Order: 6, Component: "target", Name: "connectivity",
				Status: "error",
				Detail: fmt.Sprintf("target %s:%d unreachable: %s", finalTarget.Host, finalTarget.Port, finalTarget.ErrorCode),
			})
			t.Warnings = append(t.Warnings, fmt.Sprintf("target %s:%d is unreachable (%s)", finalTarget.Host, finalTarget.Port, finalTarget.ErrorCode))
			if t.TraceStatus == "" {
				t.TraceStatus = StatusIncomplete
			}
		} else if finalTarget.Reachable != nil && *finalTarget.Reachable {
			steps = append(steps, TraceStep{
				Order: 6, Component: "target", Name: "connectivity",
				Status: "matched",
				Detail: fmt.Sprintf("target %s:%d reachable", finalTarget.Host, finalTarget.Port),
			})
		}
	}

	// v1.7AC-3: GatewayLinkInfo
	if rt.GatewayLinkID != "" && s.deps.GatewayLinkRepo != nil {
		if gw, err := s.deps.GatewayLinkRepo.FindByID(rt.GatewayLinkID); err == nil && gw != nil {
			t.GatewayLink = &GatewayLinkInfo{
				LinkID:           gw.ID,
				Enabled:          gw.Status == "active",
				HeaderInjected:   true,
				VerificationMode: "static_token",
				TargetHost:       gw.ResolveHost(),
			}
		} else {
			t.Warnings = append(t.Warnings, "GATEWAY_LINK_NOT_FOUND: link "+rt.GatewayLinkID+" not found")
		}
	}

	// v1.7W: Provider diagnostics using Diagnose()
	nextOrder := len(steps) + 1
	if rt.TLSEnabled {
		// HAProxy diagnostic
		haproxyDiag := provider.DiagnoseHAProxy()
		diagStep := TraceStep{
			Order: nextOrder, Component: "provider", Name: "haproxy_diag",
		}
		nextOrder++
		if haproxyDiag.LastErrorCode != "" {
			diagStep.Status = "error"
			diagStep.Detail = fmt.Sprintf("HAProxy: %s — %s", haproxyDiag.LastErrorCode, haproxyDiag.LastErrorMessage)
			t.Warnings = append(t.Warnings, fmt.Sprintf("HAProxy diagnostic: %s", haproxyDiag.LastErrorCode))
		} else {
			diagStep.Status = "matched"
			diagStep.Detail = fmt.Sprintf("HAProxy: available (v%s)", haproxyDiag.Version)
		}
		diagStep.ProviderDiagnostic = &haproxyDiag
		steps = append(steps, diagStep)

		// Caddy diagnostic
		caddyDiag := provider.DiagnoseCaddy()
		caddyStep := TraceStep{
			Order: nextOrder, Component: "provider", Name: "caddy_diag",
		}
		nextOrder++
		if caddyDiag.LastErrorCode != "" {
			caddyStep.Status = "error"
			caddyStep.Detail = fmt.Sprintf("Caddy: %s — %s", caddyDiag.LastErrorCode, caddyDiag.LastErrorMessage)
			t.Warnings = append(t.Warnings, fmt.Sprintf("Caddy diagnostic: %s", caddyDiag.LastErrorCode))
		} else {
			caddyStep.Status = "matched"
			caddyStep.Detail = fmt.Sprintf("Caddy: available (v%s)", caddyDiag.Version)
		}
		caddyStep.ProviderDiagnostic = &caddyDiag
		steps = append(steps, caddyStep)
	} else {
		// HTTP: Caddy diagnostic only
		caddyDiag := provider.DiagnoseCaddy()
		caddyStep := TraceStep{
			Order: nextOrder, Component: "provider", Name: "caddy_diag",
		}
		if caddyDiag.LastErrorCode != "" {
			caddyStep.Status = "error"
			caddyStep.Detail = fmt.Sprintf("Caddy: %s — %s", caddyDiag.LastErrorCode, caddyDiag.LastErrorMessage)
			t.Warnings = append(t.Warnings, fmt.Sprintf("Caddy diagnostic: %s", caddyDiag.LastErrorCode))
		} else {
			caddyStep.Status = "matched"
			caddyStep.Detail = fmt.Sprintf("Caddy: available (v%s)", caddyDiag.Version)
		}
		caddyStep.ProviderDiagnostic = &caddyDiag
		steps = append(steps, caddyStep)
	}

	if t.TraceStatus == "" {
		t.TraceStatus = StatusComplete
	}
	t.Steps = steps
	return t
}

// TraceSNI traces the access path for an SNI host (TLS passthrough).
func (s *Service) TraceSNI(ctx context.Context, sniHost string) *AccessPathTrace {
	t := &AccessPathTrace{
		Input:     sniHost,
		InputType: "sni",
	}
	steps := make([]TraceStep, 0, 6)

	// Step 1: Entry listener
	listeners, _ := s.deps.ListenerSvc.ListAll()
	has443 := false
	for _, l := range listeners {
		if l.Port == 443 {
			has443 = true
			steps = append(steps, TraceStep{
				Order: 1, Component: "listener", Name: "edge_listener",
				Status: "matched", Detail: fmt.Sprintf("port %d (%s) via %s", l.Port, l.Protocol, l.Provider),
				Address: fmt.Sprintf("0.0.0.0:%d", l.Port),
			})
			break
		}
	}
	if !has443 {
		steps = append(steps, TraceStep{
			Order: 1, Component: "listener", Name: "edge_listener",
			Status: "missing", Detail: "no port 443 listener configured",
		})
		t.Warnings = append(t.Warnings, "no EdgeMux listener on port 443")
	}

	// Step 2: EdgeMux SNI match
	edgeRule, err := s.deps.EdgeSvc.FindBySNIHost(ctx, sniHost)
	if err != nil || edgeRule == nil {
		steps = append(steps, TraceStep{
			Order: 2, Component: "edge_mux", Name: "sni_match",
			Status: "missing", Detail: fmt.Sprintf("no edge rule for SNI host '%s'", sniHost),
		})
		t.TraceStatus = StatusNotFound
		t.Errors = append(t.Errors, fmt.Sprintf("no edge_mux_rule for SNI '%s'", sniHost))
		t.Steps = steps
		return t
	}

	steps = append(steps, TraceStep{
		Order: 2, Component: "edge_mux", Name: "sni_match",
		Status: "matched",
		Detail:  fmt.Sprintf("edge rule %s: sni=%s → %s:%d", edgeRule.ID, edgeRule.SNIHost, edgeRule.TargetHost, edgeRule.TargetPort),
		Address: fmt.Sprintf("%s:%d", edgeRule.TargetHost, edgeRule.TargetPort),
	})

	// Step 3: Determine if this goes to Caddy or direct to backend
	if edgeRule.TargetHost == "127.0.0.1" && edgeRule.TargetPort == 8443 {
		steps = append(steps, TraceStep{
			Order: 3, Component: "caddy", Name: "tls_termination",
			Status: "matched", Detail: "SNI routes to Caddy on 127.0.0.1:8443 for TLS termination",
			Address: "127.0.0.1:8443",
		})

		// Look up matching route for the original SNI host
		rt, rtErr := s.deps.RouteRepo.FindByDomain(sniHost)
		if rtErr == nil && rt != nil {
			steps = append(steps, TraceStep{
				Order: 4, Component: "route", Name: "route_match",
				Status: "matched", Detail: fmt.Sprintf("route %s: domain=%s status=%s", rt.ID, rt.Domain, rt.Status),
			})

			// v1.7W: Look up endpoint for target connectivity
			if s.deps.EndpointRepo != nil {
				endpoints, epErr := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
				if epErr == nil && len(endpoints) > 0 {
					host, port := parseHostPort(endpoints[0].Address)
					target := &TargetInfo{
						Host: host, Port: port, Protocol: "https",
					}
					t.FinalTarget = target
					s.checkTargetConnectivity(target)
				}
			}
		}
	} else {
		// Direct TLS passthrough to backend
		target := &TargetInfo{
			Host: edgeRule.TargetHost, Port: edgeRule.TargetPort,
			Protocol: "tls_passthrough",
		}
		t.FinalTarget = target
		s.checkTargetConnectivity(target)
	}

	// v1.7W: HAProxy diagnostic
	haproxyDiag := provider.DiagnoseHAProxy()
	diagStep := TraceStep{
		Order: 5, Component: "provider", Name: "haproxy_diag",
	}
	if haproxyDiag.LastErrorCode != "" {
		diagStep.Status = "error"
		diagStep.Detail = fmt.Sprintf("HAProxy: %s — %s", haproxyDiag.LastErrorCode, haproxyDiag.LastErrorMessage)
		t.Warnings = append(t.Warnings, fmt.Sprintf("HAProxy diagnostic: %s", haproxyDiag.LastErrorCode))
	} else {
		diagStep.Status = "matched"
		diagStep.Detail = fmt.Sprintf("HAProxy: available (v%s)", haproxyDiag.Version)
	}
	diagStep.ProviderDiagnostic = &haproxyDiag
	steps = append(steps, diagStep)

	if t.TraceStatus == "" {
		t.TraceStatus = StatusComplete
	}
	t.Steps = steps
	return t
}

// TraceRoute traces the access path for a specific route by ID.
func (s *Service) TraceRoute(ctx context.Context, routeID string) *AccessPathTrace {
	t := &AccessPathTrace{
		Input:     routeID,
		InputType: "route_id",
	}
	steps := make([]TraceStep, 0, 5)

	// Step 1: Find route
	rt, err := s.deps.RouteRepo.FindByID(routeID)
	if err != nil || rt == nil {
		t.TraceStatus = StatusNotFound
		t.Errors = append(t.Errors, fmt.Sprintf("route %s not found", routeID))
		steps = append(steps, TraceStep{
			Order: 1, Component: "route", Name: "route_lookup",
			Status: "missing", Detail: fmt.Sprintf("no route with id '%s'", routeID),
		})
		t.Steps = steps
		return t
	}

	steps = append(steps, TraceStep{
		Order: 1, Component: "route", Name: "route_lookup",
		Status: "matched",
		Detail: fmt.Sprintf("route %s: domain=%s tls=%v status=%s space_id=%s", rt.ID, rt.Domain, rt.TLSEnabled, rt.Status, rt.SpaceID),
	})

	// v1.7W: Look up target for connectivity check
	if s.deps.EndpointRepo != nil {
		endpoints, epErr := s.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
		if epErr == nil && len(endpoints) > 0 {
			host, port := parseHostPort(endpoints[0].Address)
			target := &TargetInfo{
				Host: host, Port: port, Protocol: "http",
			}
			if rt.TLSEnabled {
				target.Protocol = "https"
			}
			t.FinalTarget = target
			s.checkTargetConnectivity(target)
		}
	}

	// Delegate to TraceDomain for the rest of the path
	domainTrace := s.TraceDomain(ctx, rt.Domain)
	domainTrace.Input = routeID
	domainTrace.InputType = "route_id"
	domainTrace.Steps = append(steps, domainTrace.Steps...)

	// v1.7W: Merge target info from our lookup if domainTrace didn't set one
	if t.FinalTarget != nil && domainTrace.FinalTarget == nil {
		domainTrace.FinalTarget = t.FinalTarget
	}

	// Check for incompleteness
	if rt.TLSEnabled {
		edgeRule, edgeErr := s.deps.EdgeSvc.FindBySNIHost(ctx, rt.Domain)
		if edgeErr != nil || edgeRule == nil {
			domainTrace.Warnings = append(domainTrace.Warnings,
				fmt.Sprintf("TLS route %s has no matching edge_mux_rule", routeID))
			if domainTrace.TraceStatus == StatusComplete {
				domainTrace.TraceStatus = StatusIncomplete
			}
		}
	}

	return domainTrace
}

// checkTargetConnectivity performs a TCP connectivity check to the target.
func (s *Service) checkTargetConnectivity(target *TargetInfo) {
	if target == nil {
		return
	}
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)

	// DNS resolution check
	ips, err := net.LookupHost(target.Host)
	if err != nil || len(ips) == 0 {
		unreachable := false
		target.Reachable = &unreachable
		target.ConnectError = fmt.Sprintf("DNS resolution failed for %s", target.Host)
		target.ErrorCode = ErrTargetDNSFailed
		return
	}

	// TCP connect check
	conn, err := net.DialTimeout("tcp", addr, s.tcpTimeout)
	if err != nil {
		unreachable := false
		target.Reachable = &unreachable
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			target.ErrorCode = ErrTargetTimeout
			target.ConnectError = fmt.Sprintf("connection to %s timed out after %v", addr, s.tcpTimeout)
		} else if isConnRefused(err) {
			target.ErrorCode = ErrTargetConnRefused
			target.ConnectError = fmt.Sprintf("connection refused: %s", addr)
		} else {
			target.ErrorCode = ErrTargetUnreachable
			target.ConnectError = fmt.Sprintf("cannot reach %s: %v", addr, err)
		}
		return
	}
	conn.Close()

	reachable := true
	target.Reachable = &reachable
}

// isConnRefused checks if the error is a connection refused error.
func isConnRefused(err error) bool {
	if opErr, ok := err.(*net.OpError); ok {
		return opErr.Err.Error() == "connectex: No connection could be made because the target machine actively refused it." ||
			opErr.Err.Error() == "connection refused"
	}
	return false
}

// parseHostPort splits an address like "host:port" or "1.2.3.4:8080".
func parseHostPort(addr string) (host string, port int) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 80
	}
	portNum := 80
	if n, err := fmt.Sscanf(p, "%d", &portNum); err == nil && n == 1 {
		port = portNum
	}
	return h, port
}
