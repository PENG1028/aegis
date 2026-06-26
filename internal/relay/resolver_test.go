package relay

import (
	"testing"
	"time"

	"aegis/internal/endpoint"
	gatewaylink "aegis/internal/gateway_link"
	"aegis/internal/listener"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/service"
)

// Stub repository implementations for testing.

type stubRouteRepo struct {
	routes []route.Route
}

func (s *stubRouteRepo) FindByDomain(domain string) (*route.Route, error) {
	for i := range s.routes {
		if s.routes[i].Domain == domain {
			return &s.routes[i], nil
		}
	}
	return nil, nil
}

func (s *stubRouteRepo) FindByID(id string) (*route.Route, error) {
	for i := range s.routes {
		if s.routes[i].ID == id {
			return &s.routes[i], nil
		}
	}
	return nil, nil
}

type stubServiceRepo struct {
	services []service.Service
}

func (s *stubServiceRepo) FindByID(id string) (*service.Service, error) {
	for i := range s.services {
		if s.services[i].ID == id {
			return &s.services[i], nil
		}
	}
	return nil, nil
}

type stubEndpointRepo struct {
	endpoints []endpoint.Endpoint
}

func (s *stubEndpointRepo) FindEnabledByServiceID(serviceID string) ([]endpoint.Endpoint, error) {
	var result []endpoint.Endpoint
	for i := range s.endpoints {
		if s.endpoints[i].ServiceID == serviceID && s.endpoints[i].Enabled {
			result = append(result, s.endpoints[i])
		}
	}
	return result, nil
}

type stubNodeRepo struct {
	nodes []node.NodeRecord
}

func (s *stubNodeRepo) FindCurrent() (*node.NodeRecord, error) {
	for i := range s.nodes {
		if s.nodes[i].IsCurrent {
			return &s.nodes[i], nil
		}
	}
	return nil, nil
}

func (s *stubNodeRepo) FindAll() ([]node.NodeRecord, error) {
	return s.nodes, nil
}

func (s *stubNodeRepo) FindByNodeID(nodeID string) (*node.NodeRecord, error) {
	for i := range s.nodes {
		if s.nodes[i].NodeID == nodeID {
			return &s.nodes[i], nil
		}
	}
	return nil, nil
}

type stubGWLinkRepo struct {
	links []gatewaylink.TrustedGateway
}

func (s *stubGWLinkRepo) FindByID(id string) (*gatewaylink.TrustedGateway, error) {
	for i := range s.links {
		if s.links[i].ID == id {
			return &s.links[i], nil
		}
	}
	return nil, nil
}

type stubListenerRepo struct {
	listeners []listener.Listener
}

func (s *stubListenerRepo) FindAll() ([]listener.Listener, error) {
	return s.listeners, nil
}

// Helper to create test data.

func now() time.Time {
	return time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC)
}

func makeNode(id, nodeID, hostname, localIP, privateIP, publicIP string, current bool) node.NodeRecord {
	return node.NodeRecord{
		ID:        id,
		NodeID:    nodeID,
		Hostname:  hostname,
		LocalIP:   localIP,
		PrivateIP: privateIP,
		PublicIP:  publicIP,
		IsCurrent: current,
		LastSeen:  now(),
		CreatedAt: now(),
		UpdatedAt: now(),
	}
}

func makeRoute(id, domain, serviceID, gatewayLinkID, status string) route.Route {
	return route.Route{
		ID:            id,
		Domain:        domain,
		ServiceID:     serviceID,
		GatewayLinkID: gatewayLinkID,
		Status:        status,
		CreatedAt:     now(),
		UpdatedAt:     now(),
	}
}

func makeService(id, name, kind string) service.Service {
	return service.Service{
		ID:        id,
		Name:      name,
		Kind:      kind,
		CreatedAt: now(),
		UpdatedAt: now(),
	}
}

func makeEndpoint(id, serviceID, typ, address, nodeID string, enabled bool) endpoint.Endpoint {
	return endpoint.Endpoint{
		ID:        id,
		ServiceID: serviceID,
		Type:      typ,
		Address:   address,
		Enabled:   enabled,
		NodeID:    nodeID,
		CreatedAt: now(),
		UpdatedAt: now(),
	}
}

func makeGWLink(id, targetNodeID, authValue string) gatewaylink.TrustedGateway {
	return gatewaylink.TrustedGateway{
		ID:           id,
		Name:         "test-gw",
		AuthValue:    authValue,
		TargetNodeID: targetNodeID,
		Status:       "active",
		CreatedAt:    now(),
		UpdatedAt:    now(),
	}
}

func makeListener(id, nodeID, purpose string, port int, status string) listener.Listener {
	return listener.Listener{
		ID:        id,
		NodeID:    nodeID,
		Provider:  "caddy_http",
		Protocol:  "http",
		BindIP:    "0.0.0.0",
		Port:      port,
		Purpose:   purpose,
		Status:    status,
		CreatedAt: now(),
		UpdatedAt: now(),
	}
}

// createResolver builds a Resolver with the given stub repos.
func createResolver(
	routes []route.Route,
	services []service.Service,
	endpoints []endpoint.Endpoint,
	nodes []node.NodeRecord,
	gwLinks []gatewaylink.TrustedGateway,
	listeners []listener.Listener,
) *Resolver {
	return NewResolver(Dependencies{
		RouteRepo:    &stubRouteRepo{routes: routes},
		ServiceRepo:  &stubServiceRepo{services: services},
		EndpointRepo: &stubEndpointRepo{endpoints: endpoints},
		NodeRepo:     &stubNodeRepo{nodes: nodes},
		GWLinkRepo:   &stubGWLinkRepo{links: gwLinks},
		ListenerRepo: &stubListenerRepo{listeners: listeners},
	})
}

// --- Tests ---

func TestResolveUnknownDomain_ExternalPassthrough(t *testing.T) {
	r := createResolver(nil, nil, nil, nil, nil, nil)
	res := r.ResolveManagedRelay("unknown.example.com", "nd_a")
	if res.Mode != string(ModeExternalPassthrough) {
		t.Errorf("expected external_passthrough, got %s", res.Mode)
	}
	if res.Managed {
		t.Error("expected managed=false for unknown domain")
	}
	if res.DirectTargetSuppressed {
		t.Error("expected direct_target_suppressed=false for external passthrough")
	}
}

func TestResolveLocalGateway(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.local", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:3001", "nd_a", true)
	listeners := []listener.Listener{
		makeListener("l_80", "nd_a", "public_http", 80, "active"),
	}

	r := createResolver([]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep}, []node.NodeRecord{nodeA}, nil, listeners)
	res := r.ResolveManagedRelay("test.local", "nd_a")

	if res.Mode != string(ModeLocalGateway) {
		t.Errorf("expected local_gateway, got %s", res.Mode)
	}
	if !res.Managed {
		t.Error("expected managed=true")
	}
	if !res.DirectTargetSuppressed {
		t.Error("expected direct_target_suppressed=true")
	}
	if res.GatewayURL != "http://127.0.0.1:80" {
		t.Errorf("expected gateway http://127.0.0.1:80, got %s", res.GatewayURL)
	}
	if res.FinalLocalTarget != "127.0.0.1:3001" {
		t.Errorf("expected final target 127.0.0.1:3001, got %s", res.FinalLocalTarget)
	}
}

func TestResolvePrivateGateway(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "10.0.0.2", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.remote", "svc_1", "gw_1", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)
	gwLink := makeGWLink("gw_1", "nd_b", "test-secret")
	listeners := []listener.Listener{
		makeListener("l_80", "nd_b", "public_http", 80, "active"),
	}

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, []gatewaylink.TrustedGateway{gwLink}, listeners,
	)
	res := r.ResolveManagedRelay("test.remote", "nd_a")

	if res.Mode != string(ModePrivateGateway) {
		t.Errorf("expected private_gateway, got %s", res.Mode)
	}
	if res.GatewayURL != "http://10.0.0.2:80" {
		t.Errorf("expected gateway http://10.0.0.2:80, got %s", res.GatewayURL)
	}
	if res.FinalLocalTarget != "127.0.0.1:2724" {
		t.Errorf("expected final target 127.0.0.1:2724, got %s", res.FinalLocalTarget)
	}
	if res.GatewayLinkID != "gw_1" {
		t.Errorf("expected gateway_link_id gw_1, got %s", res.GatewayLinkID)
	}
}

func TestResolvePublicGatewayWithGatewayLink(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.public", "svc_1", "gw_1", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)
	gwLink := makeGWLink("gw_1", "nd_b", "test-secret")
	listeners := []listener.Listener{
		makeListener("l_443", "nd_b", "public_tls_mux", 443, "active"),
	}

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, []gatewaylink.TrustedGateway{gwLink}, listeners,
	)
	res := r.ResolveManagedRelay("test.public", "nd_a")

	if res.Mode != string(ModePublicGateway) {
		t.Errorf("expected public_gateway, got %s", res.Mode)
	}
	if res.GatewayURL != "http://43.0.0.2:443" {
		t.Errorf("expected gateway http://43.0.0.2:443, got %s", res.GatewayURL)
	}
	if res.GatewayLinkID != "gw_1" {
		t.Errorf("expected gateway_link_id gw_1, got %s", res.GatewayLinkID)
	}
}

func TestResolvePublicGatewayNoGatewayLink_Unavailable(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.nogw", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, nil, nil,
	)
	res := r.ResolveManagedRelay("test.nogw", "nd_a")

	if res.Mode != string(ModeUnavailable) {
		t.Errorf("expected unavailable, got %s", res.Mode)
	}
	if res.Error != "GatewayLink required for public egress relay" {
		t.Errorf("expected GatewayLink required error, got %s", res.Error)
	}
	if res.DirectTargetSuppressed != true {
		t.Error("expected direct_target_suppressed=true for unavailable")
	}
}

func TestResolveNoFallbackToDirectTarget(t *testing.T) {
	// When target gateway is unreachable, the resolver should NOT fallback to the remote target host:port
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "", "", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.nogw", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, nil, nil,
	)
	res := r.ResolveManagedRelay("test.nogw", "nd_a")

	if res.Mode != string(ModeUnavailable) {
		t.Errorf("expected unavailable, got %s", res.Mode)
	}
	// Confirm no fallback target is exposed
	if res.FinalLocalTarget != "" {
		t.Errorf("expected no final_local_target for unavailable, got %s", res.FinalLocalTarget)
	}
	if res.GatewayURL != "" {
		t.Errorf("expected no gateway_url for unavailable, got %s", res.GatewayURL)
	}
}

func TestResolveNonHTTPKind_Unavailable(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	svc := makeService("svc_1", "test-tcp", "tcp") // TCP kind, not supported
	rt := makeRoute("rt_1", "test.tcp", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:3001", "nd_a", true)

	r := createResolver([]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep}, []node.NodeRecord{nodeA}, nil, nil)
	res := r.ResolveManagedRelay("test.tcp", "nd_a")

	if res.Mode != string(ModeUnavailable) {
		t.Errorf("expected unavailable for non-HTTP service, got %s", res.Mode)
	}
}

func TestResolveEndpointWithNodeID(t *testing.T) {
	// Endpoint explicitly has node_id matching the from_node
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.local", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "private", "10.0.0.99:3001", "nd_a", true)
	listeners := []listener.Listener{
		makeListener("l_80", "nd_a", "public_http", 80, "active"),
	}

	r := createResolver([]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep}, []node.NodeRecord{nodeA}, nil, listeners)
	res := r.ResolveManagedRelay("test.local", "nd_a")

	if res.Mode != string(ModeLocalGateway) {
		t.Errorf("expected local_gateway for endpoint with matching node_id, got %s", res.Mode)
	}
}

func TestModelParseHostPort(t *testing.T) {
	host, port := ParseHostPort("127.0.0.1:3001")
	if host != "127.0.0.1" || port != 3001 {
		t.Errorf("expected 127.0.0.1:3001, got %s:%d", host, port)
	}

	host, port = ParseHostPort("10.0.0.5:80")
	if host != "10.0.0.5" || port != 80 {
		t.Errorf("expected 10.0.0.5:80, got %s:%d", host, port)
	}

	host, port = ParseHostPort("invalid")
	if port != 0 {
		t.Errorf("expected port 0 for invalid address, got %d", port)
	}
}

// --- v1.8B-2 Auth Tightening Tests ---

func TestResolvePrivateGatewayWithoutGWLink_Unavailable(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "10.0.0.2", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.nogw", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)
	listeners := []listener.Listener{
		makeListener("l_80", "nd_b", "public_http", 80, "active"),
	}

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, nil, listeners,
	)
	res := r.ResolveManagedRelay("test.nogw", "nd_a")

	if res.Mode != string(ModeUnavailable) {
		t.Errorf("expected unavailable for private_gateway without GatewayLink, got %s", res.Mode)
	}
	if res.Error != "GatewayLink required for private egress relay" {
		t.Errorf("expected 'GatewayLink required for private egress relay', got %s", res.Error)
	}
	if res.DirectTargetSuppressed != true {
		t.Error("expected direct_target_suppressed=true")
	}
	if res.FinalLocalTarget != "" {
		t.Errorf("expected no final_local_target leak, got %s", res.FinalLocalTarget)
	}
	if res.GatewayURL != "" {
		t.Errorf("expected no gateway_url leak, got %s", res.GatewayURL)
	}
}

func TestResolvePrivateGatewayWithGWLink_Pass(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "10.0.0.1", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "10.0.0.2", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.gw", "svc_1", "gw_1", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)
	gwLink := makeGWLink("gw_1", "nd_b", "test-secret")
	listeners := []listener.Listener{
		makeListener("l_80", "nd_b", "public_http", 80, "active"),
	}

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, []gatewaylink.TrustedGateway{gwLink}, listeners,
	)
	res := r.ResolveManagedRelay("test.gw", "nd_a")

	if res.Mode != string(ModePrivateGateway) {
		t.Errorf("expected private_gateway with GatewayLink, got %s", res.Mode)
	}
	if res.GatewayLinkID != "gw_1" {
		t.Errorf("expected gateway_link_id gw_1, got %s", res.GatewayLinkID)
	}
	if res.GatewayURL != "http://10.0.0.2:80" {
		t.Errorf("expected gateway http://10.0.0.2:80, got %s", res.GatewayURL)
	}
}

func TestResolvePublicGatewayWithoutGWLink_Unavailable(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.nogw", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, nil, nil,
	)
	res := r.ResolveManagedRelay("test.nogw", "nd_a")

	if res.Mode != string(ModeUnavailable) {
		t.Errorf("expected unavailable, got %s", res.Mode)
	}
	if res.Error != "GatewayLink required for public egress relay" {
		t.Errorf("expected GatewayLink required error, got %s", res.Error)
	}
}

func TestResolvePublicGatewayWithGWLink_Pass(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "", "43.0.0.2", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.pubgw", "svc_1", "gw_1", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)
	gwLink := makeGWLink("gw_1", "nd_b", "test-secret")
	listeners := []listener.Listener{
		makeListener("l_443", "nd_b", "public_tls_mux", 443, "active"),
	}

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, []gatewaylink.TrustedGateway{gwLink}, listeners,
	)
	res := r.ResolveManagedRelay("test.pubgw", "nd_a")

	if res.Mode != string(ModePublicGateway) {
		t.Errorf("expected public_gateway with GatewayLink, got %s", res.Mode)
	}
	if res.GatewayLinkID != "gw_1" {
		t.Errorf("expected gateway_link_id gw_1, got %s", res.GatewayLinkID)
	}
}

// unavailable must not leak internal target
func TestUnavailableNoFinalLocalTargetLeak(t *testing.T) {
	nodeA := makeNode("id_a", "nd_a", "server-a", "127.0.0.1", "", "43.0.0.1", true)
	nodeB := makeNode("id_b", "nd_b", "server-b", "127.0.0.1", "10.0.0.2", "", false)
	svc := makeService("svc_1", "test-svc", "http")
	rt := makeRoute("rt_1", "test.nogw", "svc_1", "", "active")
	ep := makeEndpoint("ep_1", "svc_1", "local", "127.0.0.1:2724", "nd_b", true)

	r := createResolver(
		[]route.Route{rt}, []service.Service{svc}, []endpoint.Endpoint{ep},
		[]node.NodeRecord{nodeA, nodeB}, nil, nil,
	)
	res := r.ResolveManagedRelay("test.nogw", "nd_a")

	if res.FinalLocalTarget != "" {
		t.Errorf("unavailable must not leak final_local_target, got %s", res.FinalLocalTarget)
	}
	if res.GatewayURL != "" {
		t.Errorf("unavailable must not leak gateway_url, got %s", res.GatewayURL)
	}
}

// external_passthrough must not show internal target
func TestExternalPassthroughNoInternalTargetLeak(t *testing.T) {
	r := createResolver(nil, nil, nil, nil, nil, nil)
	res := r.ResolveManagedRelay("unknown.example.com", "nd_a")

	if res.FinalLocalTarget != "" {
		t.Errorf("external_passthrough must not show final_local_target, got %s", res.FinalLocalTarget)
	}
	if res.GatewayURL != "" {
		t.Errorf("external_passthrough must not show gateway_url, got %s", res.GatewayURL)
	}
	if res.EndpointID != "" {
		t.Errorf("external_passthrough must not show endpoint_id, got %s", res.EndpointID)
	}
	if res.RouteID != "" {
		t.Errorf("external_passthrough must not show route_id, got %s", res.RouteID)
	}
	if res.ServiceID != "" {
		t.Errorf("external_passthrough must not show service_id, got %s", res.ServiceID)
	}
}
