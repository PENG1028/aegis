package routingtable

import (
	"testing"
)

// Helper to create a basic test input.

func defaultGenerateInput() GenerateInput {
	return GenerateInput{
		FromNodeID: "nd_a",
		AllNodes: []NodeInfo{
			{NodeID: "nd_a"},
			{NodeID: "nd_b"},
		},
		AllServices: []ServiceInfo{
			{ServiceID: "svc_api"},
		},
		AllRoutes: []RouteInfo{
			{RouteID: "rt_api", Domain: "api.example.com", ServiceID: "svc_api"},
		},
		AllEndpoints: []EndpointInfo{
			{EndpointID: "ep_api", ServiceID: "svc_api", Type: "http", Address: "127.0.0.1:8080", NodeID: "nd_a"},
		},
		AllGateways:    []GatewayInfo{},
		TopologyEdges:  []TopologyEdgeInfo{},
		GatewayLinks:   []GatewayLinkInfo{},
		ResolvePolicy: func(routeID, serviceID string) (PolicyInfo, error) {
			return PolicyInfo{
				Source:             "default",
				Mode:               "auto",
				AllowLocal:         true,
				AllowPrivate:       true,
				AllowPublic:        false,
				RequireGatewayLink: true,
				RequireRelay:       true,
				PreserveHost:       true,
				TLSMode:            "http_only",
			}, nil
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestLocalEndpointProducesLocalCandidate(t *testing.T) {
	input := defaultGenerateInput()
	gen := NewGenerator()

	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(table.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(table.Entries))
	}
	entry := table.Entries[0]
	if entry.TargetNodeID != "nd_a" {
		t.Errorf("expected target nd_a, got %s", entry.TargetNodeID)
	}
	if entry.Status != StatusAvailable {
		t.Errorf("expected available, got %s", entry.Status)
	}
}

func TestRemoteEndpointPrivateTopology(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.TopologyEdges = []TopologyEdgeInfo{
		{FromNodeID: "nd_a", ToNodeID: "nd_b", PrivateReachable: true, Status: "verified"},
	}
	input.GatewayLinks = []GatewayLinkInfo{
		{ID: "gl_ab", SourceNodeID: "nd_a", TargetNodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_private", NodeID: "nd_b", Name: "private-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(table.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(table.Entries))
	}
	entry := table.Entries[0]
	if entry.Status != StatusAvailable {
		t.Errorf("expected available, got %s (reason: %s)", entry.Status, entry.UnavailableReason)
	}
	if len(entry.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	if entry.Candidates[0].Mode != CandidateModePrivate {
		t.Errorf("expected private_gateway candidate, got %s", entry.Candidates[0].Mode)
	}
}

func TestRemoteEndpointPublicGateway(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "43.x.x.x:8080", NodeID: "nd_b"},
	}
	input.GatewayLinks = []GatewayLinkInfo{
		{ID: "gl_ab", SourceNodeID: "nd_a", TargetNodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_public", NodeID: "nd_b", Name: "public-gw",
			Type: "public", Host: "43.x.x.x", Port: 443, Scheme: "https",
			PublicAccessible: true, Enabled: true, Priority: 100},
	}
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		return PolicyInfo{
			Mode: "auto", AllowLocal: true, AllowPrivate: true, AllowPublic: true,
			RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
		}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if entry.Status != StatusAvailable {
		t.Errorf("expected available with allow_public=true, got %s: %s", entry.Status, entry.UnavailableReason)
	}
	if len(entry.Candidates) == 0 {
		t.Fatal("expected at least one candidate")
	}
	hasPublic := false
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePublic {
			hasPublic = true
			break
		}
	}
	if !hasPublic {
		t.Error("expected public_gateway candidate when allow_public=true")
	}
}

func TestRemoteEndpointMissingGatewayLink(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_private", NodeID: "nd_b", Name: "private-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}
	// No GatewayLink set up
	// No topology edge set up

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if entry.Status != StatusMissingGatewayLink && entry.Status != StatusUnavailable {
		t.Errorf("expected missing_gateway_link or unavailable, got %s", entry.Status)
	}
	if len(entry.Candidates) != 0 {
		t.Errorf("expected 0 candidates when gateway link is missing, got %d", len(entry.Candidates))
	}
}

func TestPublicGatewayRejectedWhenAllowPublicFalse(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "43.x.x.x:8080", NodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_public", NodeID: "nd_b", Name: "public-gw",
			Type: "public", Host: "43.x.x.x", Port: 443, Scheme: "https",
			PublicAccessible: true, Enabled: true, Priority: 100},
	}
	// allow_public defaults to false in the default policy

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	// Should be unavailable or missing_gateway_link, but NOT have a public candidate
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePublic {
			t.Errorf("public candidate should not appear when allow_public=false")
		}
	}
}

func TestPrivateGatewayRejectedWhenAllowPrivateFalse(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_private", NodeID: "nd_b", Name: "private-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		return PolicyInfo{
			Mode: "auto", AllowLocal: true, AllowPrivate: false, AllowPublic: false,
			RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
		}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePrivate {
			t.Errorf("private candidate should not appear when allow_private=false")
		}
	}
}

func TestNoGatewayProducesMissingGateway(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	// No gateways at all on target node

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	// Should be unavailable with no candidates
	if len(entry.Candidates) != 0 {
		t.Errorf("expected 0 candidates when target has no gateways, got %d", len(entry.Candidates))
	}
}

func TestMultiCandidateOrderStable(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.TopologyEdges = []TopologyEdgeInfo{
		{FromNodeID: "nd_a", ToNodeID: "nd_b", PrivateReachable: true, Status: "verified"},
	}
	input.GatewayLinks = []GatewayLinkInfo{
		{ID: "gl_ab", SourceNodeID: "nd_a", TargetNodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_private", NodeID: "nd_b", Name: "private-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
		{GatewayID: "gw_b_public", NodeID: "nd_b", Name: "public-gw",
			Type: "public", Host: "43.x.x.x", Port: 443, Scheme: "https",
			PublicAccessible: true, Enabled: true, Priority: 50},
	}
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		return PolicyInfo{
			Mode: "multi", AllowLocal: true, AllowPrivate: true, AllowPublic: true,
			RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
			PrimaryGatewayID: "gw_b_private",
			FallbackGatewayIDs: []string{"gw_b_public"},
		}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if len(entry.Candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(entry.Candidates))
	}
	if entry.Candidates[0].GatewayID != "gw_b_private" {
		t.Errorf("expected primary first, got %s", entry.Candidates[0].GatewayID)
	}
	if entry.Candidates[1].GatewayID != "gw_b_public" {
		t.Errorf("expected fallback second, got %s", entry.Candidates[1].GatewayID)
	}
}

func TestFixedCandidateOnlyPrimary(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.GatewayLinks = []GatewayLinkInfo{
		{ID: "gl_ab", SourceNodeID: "nd_a", TargetNodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_fixed", NodeID: "nd_b", Name: "fixed-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		return PolicyInfo{
			Mode: "fixed", AllowLocal: true, AllowPrivate: true, AllowPublic: false,
			RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
			PrimaryGatewayID: "gw_b_fixed",
		}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if len(entry.Candidates) != 1 {
		t.Errorf("expected exactly 1 candidate in fixed mode, got %d", len(entry.Candidates))
	}
}

func TestDisabledRouteProducesDisabledEntry(t *testing.T) {
	input := defaultGenerateInput()
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		return PolicyInfo{
			Mode: "disabled", AllowLocal: true, AllowPrivate: true, AllowPublic: false,
			RequireGatewayLink: true, RequireRelay: true, PreserveHost: true, TLSMode: "http_only",
		}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if entry.Status != StatusDisabled {
		t.Errorf("expected disabled status, got %s", entry.Status)
	}
	if len(entry.Candidates) != 0 {
		t.Errorf("expected 0 candidates for disabled policy, got %d", len(entry.Candidates))
	}
}

func TestDirectRemoteTargetNotPresent(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	// Validate that no candidate has direct/raw mode
	validation := Validate(table)
	if !validation.IsValid {
		for _, e := range validation.Errors {
			t.Errorf("validation error: %s", e)
		}
	}

	// Check no candidate has direct_remote_target
	for _, entry := range table.Entries {
		for _, c := range entry.Candidates {
			if c.Mode == "direct_remote_target" || c.Mode == "raw_target" {
				t.Errorf("forbidden candidate mode: %s", c.Mode)
			}
		}
	}
}

func TestCrossNodeCandidateHasGatewayLinkID(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	input.TopologyEdges = []TopologyEdgeInfo{
		{FromNodeID: "nd_a", ToNodeID: "nd_b", PrivateReachable: true, Status: "verified"},
	}
	input.GatewayLinks = []GatewayLinkInfo{
		{ID: "gl_ab", SourceNodeID: "nd_a", TargetNodeID: "nd_b"},
	}
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_priv", NodeID: "nd_b", Name: "priv-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePrivate && c.GatewayLinkID == "" {
			t.Errorf("cross-node candidate should have gateway_link_id")
		}
	}
}

func TestTopologyUnreachable(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_b", ServiceID: "svc_api", Type: "http", Address: "10.0.0.5:8080", NodeID: "nd_b"},
	}
	// No topology edge, no gateway link
	input.AllGateways = []GatewayInfo{
		{GatewayID: "gw_b_priv", NodeID: "nd_b", Name: "priv-gw",
			Type: "private", Host: "10.0.0.5", Port: 80, Scheme: "http",
			PrivateAccessible: true, Enabled: true, Priority: 100},
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	// With no topology and no gateway link, private candidate should not appear
	for _, c := range entry.Candidates {
		if c.Mode == CandidateModePrivate {
			t.Errorf("private candidate should not appear without topology edge or gateway link")
		}
	}
}

func TestMultipleRoutes(t *testing.T) {
	input := defaultGenerateInput()
	input.AllRoutes = []RouteInfo{
		{RouteID: "rt_api", Domain: "api.example.com", ServiceID: "svc_api"},
		{RouteID: "rt_app", Domain: "app.example.com", ServiceID: "svc_app"},
	}
	input.AllServices = []ServiceInfo{
		{ServiceID: "svc_api"},
		{ServiceID: "svc_app"},
	}
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_api", ServiceID: "svc_api", Type: "http", Address: "127.0.0.1:8080", NodeID: "nd_a"},
		{EndpointID: "ep_app", ServiceID: "svc_app", Type: "http", Address: "10.0.0.5:3000", NodeID: "nd_b"},
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(table.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(table.Entries))
	}
}

func TestValidationNoDirectRemoteCandidates(t *testing.T) {
	entry := RoutingTableEntry{
		RouteID: "rt_test",
		FromNodeID: "nd_a", TargetNodeID: "nd_b",
		Status: StatusAvailable,
		Candidates: []Candidate{
			{Mode: "direct_remote_target", GatewayID: "bad"},
		},
	}
	result := ValidateEntry(entry)
	if result.IsValid {
		t.Error("expected validation to reject direct_remote_target candidate")
	}
}

func TestPolicyRoutePrecedence(t *testing.T) {
	input := defaultGenerateInput()
	input.AllEndpoints = []EndpointInfo{
		{EndpointID: "ep_a", ServiceID: "svc_api", Type: "http", Address: "127.0.0.1:8080", NodeID: "nd_a"},
	}
	input.ResolvePolicy = func(routeID, serviceID string) (PolicyInfo, error) {
		// Simulate: route policy differs from default
		if routeID == "rt_api" {
			return PolicyInfo{
				Source: "route", Mode: "fixed",
				PrimaryGatewayID: "gw_local",
				AllowLocal: true, AllowPrivate: false, AllowPublic: false,
				RequireGatewayLink: false, RequireRelay: false, PreserveHost: true, TLSMode: "http_only",
			}, nil
		}
		return PolicyInfo{Source: "default", Mode: "auto"}, nil
	}

	gen := NewGenerator()
	table, err := gen.Generate(input)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	entry := table.Entries[0]
	if entry.GatewayPolicy.Mode != "fixed" {
		t.Errorf("expected fixed mode from route policy, got %s", entry.GatewayPolicy.Mode)
	}
}

func TestServiceValidate(t *testing.T) {
	svc := NewService()
	input := defaultGenerateInput()

	table, validation, err := svc.Preview(input)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if table == nil {
		t.Fatal("expected non-nil table")
	}
	if validation == nil {
		t.Fatal("expected non-nil validation")
	}
}
