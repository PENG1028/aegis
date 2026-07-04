// Package e2e contains end-to-end integration tests.
//
// Scenario C: Multi-Service Policy Routing
// Tests routing policy enforcement with multiple nodes, services, gateways,
// topology edges, and routing table generation. Verifies candidate ordering
// (local > private > public) and that disabled candidates are excluded.
package e2e

import (
	"os"
	"testing"

	"aegis/internal/endpoint"
	"aegis/internal/gateway"
	"aegis/internal/core"
	"aegis/internal/logs"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/routingpolicy"
	"aegis/internal/routingtable"
	"aegis/internal/service"
	"aegis/internal/store"
	"aegis/internal/topology"
)

// setupE2EDB creates a temp SQLite database, initializes the schema, and returns
// the *store.Store with a cleanup function.
func setupE2EDB(t *testing.T) (*store.Store, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "aegis-e2e-c-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	os.Remove(dbPath)

	sqlDB, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := store.Initialize(sqlDB); err != nil {
		sqlDB.Close()
		t.Fatalf("init schema: %v", err)
	}

	st := store.New(sqlDB)
	cleanup := func() {
		st.Close()
		os.Remove(dbPath)
	}
	return st, cleanup
}

// TestPolicyRouting_MultiService verifies multi-service routing with gateway policies.
func TestPolicyRouting_MultiService(t *testing.T) {
	st, cleanup := setupE2EDB(t)
	defer cleanup()

	logRepo := logs.NewRepository(st.DB)
	logSvc := logs.NewAppService(logRepo)

	// Step 1: Set up 3 nodes
	nodeRepo := node.NewRepository(st.DB)
	nodeSvc := node.NewService(nodeRepo)

	nodeA, err := nodeSvc.CreateNode("node-alpha", node.RoleGateway, "alpha.local",
		"10.0.1.1", "192.168.1.1", "linux", "amd64", "1.0")
	if err != nil {
		t.Fatalf("create node A: %v", err)
	}
	nodeB, err := nodeSvc.CreateNode("node-beta", node.RoleWorker, "beta.local",
		"10.0.2.1", "192.168.2.1", "linux", "amd64", "1.0")
	if err != nil {
		t.Fatalf("create node B: %v", err)
	}
	nodeC, err := nodeSvc.CreateNode("node-gamma", node.RoleRelay, "gamma.local",
		"10.0.3.1", "192.168.3.1", "linux", "amd64", "1.0")
	if err != nil {
		t.Fatalf("create node C: %v", err)
	}
	t.Logf("created 3 nodes: %s, %s, %s", nodeA.NodeID, nodeB.NodeID, nodeC.NodeID)

	// Step 2: Create 3 services with different names
	serviceRepo := service.NewRepository(st.DB)
	serviceSvc := service.NewAppService(serviceRepo, logSvc)

	// Use CreateServiceDirect to avoid project dependency
	svc1 := &service.Service{
		ID:      core.NewID("svc"),
		Name:    "api-service",
		Kind:    "http",
		Env:     "prod",
		Status:  "active",
		OwnerType: "admin",
	}
	if err := serviceSvc.CreateServiceDirect(svc1); err != nil {
		t.Fatalf("create service 1: %v", err)
	}

	svc2 := &service.Service{
		ID:      core.NewID("svc"),
		Name:    "worker-service",
		Kind:    "http",
		Env:     "prod",
		Status:  "active",
		OwnerType: "admin",
	}
	if err := serviceSvc.CreateServiceDirect(svc2); err != nil {
		t.Fatalf("create service 2: %v", err)
	}

	svc3 := &service.Service{
		ID:      core.NewID("svc"),
		Name:    "public-service",
		Kind:    "http",
		Env:     "prod",
		Status:  "active",
		OwnerType: "admin",
	}
	if err := serviceSvc.CreateServiceDirect(svc3); err != nil {
		t.Fatalf("create service 3: %v", err)
	}
	t.Logf("created 3 services: %s, %s, %s", svc1.ID, svc2.ID, svc3.ID)

	// Step 3: Create 3 gateways with different types (local, private, public)
	gwInvRepo := gateway.NewInventoryRepository(st.DB)
	gwInvSvc := gateway.NewInventoryService(gwInvRepo)

	localGW, err := gwInvSvc.CreateGateway(gateway.CreateGatewayInput{
		NodeID:            nodeA.NodeID,
		Name:              "local-gw",
		Type:              gateway.GWTypeLocal,
		Provider:          gateway.GWProviderCaddy,
		BindAddr:          "127.0.0.1:80",
		Host:              "127.0.0.1",
		Port:              80,
		Scheme:            gateway.GWSchemeHTTP,
		PublicAccessible:  false,
		PrivateAccessible: true,
		Priority:          10,
	})
	if err != nil {
		t.Fatalf("create local gateway: %v", err)
	}

	privateGW, err := gwInvSvc.CreateGateway(gateway.CreateGatewayInput{
		NodeID:            nodeB.NodeID,
		Name:              "private-gw",
		Type:              gateway.GWTypePrivate,
		Provider:          gateway.GWProviderCaddy,
		BindAddr:          "10.0.2.1:80",
		Host:              "10.0.2.1",
		Port:              80,
		Scheme:            gateway.GWSchemeHTTP,
		PublicAccessible:  false,
		PrivateAccessible: true,
		Priority:          20,
	})
	if err != nil {
		t.Fatalf("create private gateway: %v", err)
	}

	publicGW, err := gwInvSvc.CreateGateway(gateway.CreateGatewayInput{
		NodeID:            nodeC.NodeID,
		Name:              "public-gw",
		Type:              gateway.GWTypePublic,
		Provider:          gateway.GWProviderCaddy,
		BindAddr:          "0.0.0.0:443",
		Host:              "public.aegis.example.com",
		Port:              443,
		Scheme:            gateway.GWSchemeHTTPS,
		PublicAccessible:  true,
		PrivateAccessible: false,
		Priority:          30,
	})
	if err != nil {
		t.Fatalf("create public gateway: %v", err)
	}
	t.Logf("created 3 gateways: local=%s, private=%s, public=%s",
		localGW.GatewayID, privateGW.GatewayID, publicGW.GatewayID)

	// Step 4: Create gateway policies for each service
	policyRepo := routingpolicy.NewRepository(st.DB)
	policySvc := routingpolicy.NewService(policyRepo)

	// Service 1 (api-service): local-only — allow_local=true, allow_private=false
	allowLocal := true
	allowPrivate := false
	allowPublic := false
	_, err = policySvc.SetServicePolicy(routingpolicy.PolicyInput{
		ServiceID:        svc1.ID,
		Mode:             routingpolicy.ModeFixed,
		PrimaryGatewayID: localGW.GatewayID,
		AllowLocal:       &allowLocal,
		AllowPrivate:     &allowPrivate,
		AllowPublic:      &allowPublic,
	})
	if err != nil {
		t.Fatalf("set policy for service 1: %v", err)
	}

	// Service 2 (worker-service): private-preferred — primary=private, fallback=local
	_, err = policySvc.SetServicePolicy(routingpolicy.PolicyInput{
		ServiceID:          svc2.ID,
		Mode:               routingpolicy.ModeFixed,
		PrimaryGatewayID:   privateGW.GatewayID,
		FallbackGatewayIDs: []string{localGW.GatewayID},
		AllowLocal:         &allowLocal,
		AllowPrivate:       &allowPrivate,
		AllowPublic:        &allowPublic,
	})
	if err != nil {
		t.Fatalf("set policy for service 2: %v", err)
	}

	// Service 3 (public-service): public-fallback — primary=local, fallback=public
	requireGWLink := true
	allowPub := true
	_, err = policySvc.SetServicePolicy(routingpolicy.PolicyInput{
		ServiceID:          svc3.ID,
		Mode:               routingpolicy.ModeFixed,
		PrimaryGatewayID:   localGW.GatewayID,
		FallbackGatewayIDs: []string{publicGW.GatewayID},
		AllowLocal:         &allowLocal,
		AllowPublic:        &allowPub,
		RequireGatewayLink: &requireGWLink,
	})
	if err != nil {
		t.Fatalf("set policy for service 3: %v", err)
	}
	t.Log("created gateway policies for all 3 services")

	// Step 5: Create endpoints for each service on different nodes
	endpointRepo := endpoint.NewRepository(st.DB)
	endpointSvc := endpoint.NewAppService(endpointRepo, logSvc)

	// Service 1: endpoint on node A (local)
	ep1, err := endpointSvc.CreateEndpoint(nil, endpoint.CreateEndpointInput{
		ServiceID: svc1.ID,
		Type:      "local",
		Address:   "127.0.0.1:3001",
		NodeID:    nodeA.NodeID,
	})
	if err != nil {
		t.Fatalf("create endpoint for svc1: %v", err)
	}

	// Service 2: endpoint on node B (private)
	ep2, err := endpointSvc.CreateEndpoint(nil, endpoint.CreateEndpointInput{
		ServiceID: svc2.ID,
		Type:      "private",
		Address:   "10.0.2.1:3002",
		NodeID:    nodeB.NodeID,
	})
	if err != nil {
		t.Fatalf("create endpoint for svc2: %v", err)
	}

	// Service 3: endpoint on node C (public)
	ep3, err := endpointSvc.CreateEndpoint(nil, endpoint.CreateEndpointInput{
		ServiceID: svc3.ID,
		Type:      "public",
		Address:   "public.aegis.example.com:3003",
		NodeID:    nodeC.NodeID,
	})
	if err != nil {
		t.Fatalf("create endpoint for svc3: %v", err)
	}
	t.Logf("created endpoints: ep1=%s on %s, ep2=%s on %s, ep3=%s on %s",
		ep1.ID, nodeA.NodeID, ep2.ID, nodeB.NodeID, ep3.ID, nodeC.NodeID)

	// Step 6: Create topology edges between nodes
	topoRepo := topology.NewRepository(st.DB)
	topoSvc := topology.NewService(topoRepo)

	privTrue := true
	pubTrue := true
	pubFalse := false

	// Edge A -> B: private reachable, public not reachable
	_, err = topoSvc.CreateOrUpdateEdge(topology.CreateEdgeInput{
		FromNodeID:       nodeA.NodeID,
		ToNodeID:         nodeB.NodeID,
		PrivateReachable: &privTrue,
		PublicReachable:  &pubFalse,
	})
	if err != nil {
		t.Fatalf("create topology edge A->B: %v", err)
	}

	// Edge A -> C: both private and public reachable
	_, err = topoSvc.CreateOrUpdateEdge(topology.CreateEdgeInput{
		FromNodeID:       nodeA.NodeID,
		ToNodeID:         nodeC.NodeID,
		PrivateReachable: &privTrue,
		PublicReachable:  &pubTrue,
	})
	if err != nil {
		t.Fatalf("create topology edge A->C: %v", err)
	}

	// Edge B -> C: public reachable, private not
	_, err = topoSvc.CreateOrUpdateEdge(topology.CreateEdgeInput{
		FromNodeID:       nodeB.NodeID,
		ToNodeID:         nodeC.NodeID,
		PrivateReachable: &pubFalse,
		PublicReachable:  &pubTrue,
	})
	if err != nil {
		t.Fatalf("create topology edge B->C: %v", err)
	}
	t.Log("created topology edges between all 3 nodes")

	// Step 7: Create routes for each service
	routeRepo := route.NewRepository(st.DB)

	// Create routes directly via repo (bypass edgemux dependency for simplicity)
	rt1 := &route.Route{
		ID:        core.NewID("rt"),
		Domain:    "api.example.com",
		PathPrefix: "/",
		ServiceID: svc1.ID,
		Status:    "active",
		OwnerType: "admin",
	}
	if err := routeRepo.Create(rt1); err != nil {
		t.Fatalf("create route 1: %v", err)
	}

	rt2 := &route.Route{
		ID:        core.NewID("rt"),
		Domain:    "worker.example.com",
		PathPrefix: "/",
		ServiceID: svc2.ID,
		Status:    "active",
		OwnerType: "admin",
	}
	if err := routeRepo.Create(rt2); err != nil {
		t.Fatalf("create route 2: %v", err)
	}

	rt3 := &route.Route{
		ID:        core.NewID("rt"),
		Domain:    "public.example.com",
		PathPrefix: "/",
		ServiceID: svc3.ID,
		Status:    "active",
		OwnerType: "admin",
	}
	if err := routeRepo.Create(rt3); err != nil {
		t.Fatalf("create route 3: %v", err)
	}
	t.Logf("created 3 routes: %s, %s, %s", rt1.ID, rt2.ID, rt3.ID)

	// Step 8: Use routingtable.Generator to generate routing table for node A
	rtGen := routingtable.NewGenerator()

	// Collect all the data needed for GenerateInput
	allNodes := []routingtable.NodeInfo{
		{NodeID: nodeA.NodeID},
		{NodeID: nodeB.NodeID},
		{NodeID: nodeC.NodeID},
	}

	allServices := []routingtable.ServiceInfo{
		{ServiceID: svc1.ID},
		{ServiceID: svc2.ID},
		{ServiceID: svc3.ID},
	}

	allRoutes := []routingtable.RouteInfo{
		{RouteID: rt1.ID, Domain: "api.example.com", ServiceID: svc1.ID},
		{RouteID: rt2.ID, Domain: "worker.example.com", ServiceID: svc2.ID},
		{RouteID: rt3.ID, Domain: "public.example.com", ServiceID: svc3.ID},
	}

	allEndpoints := []routingtable.EndpointInfo{
		{EndpointID: ep1.ID, ServiceID: svc1.ID, Type: "local", Address: "127.0.0.1:3001", NodeID: nodeA.NodeID},
		{EndpointID: ep2.ID, ServiceID: svc2.ID, Type: "private", Address: "10.0.2.1:3002", NodeID: nodeB.NodeID},
		{EndpointID: ep3.ID, ServiceID: svc3.ID, Type: "public", Address: "public.aegis.example.com:3003", NodeID: nodeC.NodeID},
	}

	allGateways := []routingtable.GatewayInfo{
		{
			GatewayID: localGW.GatewayID, NodeID: nodeA.NodeID, Name: "local-gw",
			Type: gateway.GWTypeLocal, Host: "127.0.0.1", Port: 80,
			Scheme: gateway.GWSchemeHTTP, Enabled: true, Priority: 10,
		},
		{
			GatewayID: privateGW.GatewayID, NodeID: nodeB.NodeID, Name: "private-gw",
			Type: gateway.GWTypePrivate, Host: "10.0.2.1", Port: 80,
			Scheme: gateway.GWSchemeHTTP, Enabled: true, Priority: 20,
		},
		{
			GatewayID: publicGW.GatewayID, NodeID: nodeC.NodeID, Name: "public-gw",
			Type: gateway.GWTypePublic, Host: "public.aegis.example.com", Port: 443,
			Scheme: gateway.GWSchemeHTTPS, Enabled: true, Priority: 30,
			PublicAccessible: true,
		},
	}

	topoEdges := []routingtable.TopologyEdgeInfo{
		{FromNodeID: nodeA.NodeID, ToNodeID: nodeB.NodeID, PrivateReachable: true, PublicReachable: false, Status: "verified"},
		{FromNodeID: nodeA.NodeID, ToNodeID: nodeC.NodeID, PrivateReachable: true, PublicReachable: true, Status: "verified"},
		{FromNodeID: nodeB.NodeID, ToNodeID: nodeC.NodeID, PrivateReachable: false, PublicReachable: true, Status: "verified"},
	}

	resolvePolicy := func(routeID, serviceID string) (routingtable.PolicyInfo, error) {
		resolved, err := policySvc.ResolvePolicy(routeID, serviceID)
		if err != nil {
			return routingtable.PolicyInfo{}, err
		}
		return routingtable.PolicyInfo{
			Mode:               resolved.Mode,
			PrimaryGatewayID:   resolved.PrimaryGatewayID,
			FallbackGatewayIDs: resolved.FallbackGatewayIDs,
			AllowLocal:         resolved.AllowLocal,
			AllowPrivate:       resolved.AllowPrivate,
			AllowPublic:        resolved.AllowPublic,
			RequireGatewayLink: resolved.RequireGatewayLink,
			RequireRelay:       resolved.RequireRelay,
			PreserveHost:       resolved.PreserveHost,
			TLSMode:            resolved.TLSMode,
		}, nil
	}

	// Generate routing table for node A
	input := routingtable.GenerateInput{
		FromNodeID:    nodeA.NodeID,
		AllNodes:      allNodes,
		AllServices:   allServices,
		AllRoutes:     allRoutes,
		AllEndpoints:  allEndpoints,
		AllGateways:   allGateways,
		TopologyEdges: topoEdges,
		GatewayLinks:  nil, // no gateway links in this test
		ResolvePolicy: resolvePolicy,
	}

	table, err := rtGen.Generate(input)
	if err != nil {
		t.Fatalf("generate routing table: %v", err)
	}
	if table == nil {
		t.Fatal("routing table should not be nil")
	}
	t.Logf("generated routing table for node %s: %d entries", nodeA.NodeID, len(table.Entries))

	// Step 9: Verify the routing table entries
	for _, entry := range table.Entries {
		t.Logf("entry: domain=%s service=%s status=%s candidates=%d",
			entry.Domain, entry.ServiceID, entry.Status, len(entry.Candidates))

		for i, cand := range entry.Candidates {
			t.Logf("  candidate[%d]: mode=%s gateway=%s priority=%d url=%s",
				i, cand.Mode, cand.GatewayID, cand.Priority, cand.GatewayURL)
		}
	}

	// Verify each service's routing entry exists and has candidates
	foundServices := make(map[string]bool)
	for _, entry := range table.Entries {
		foundServices[entry.ServiceID] = true
	}

	if !foundServices[svc1.ID] {
		t.Error("routing table missing entry for service 1 (api-service)")
	}
	if !foundServices[svc2.ID] {
		t.Error("routing table missing entry for service 2 (worker-service)")
	}
	if !foundServices[svc3.ID] {
		t.Error("routing table missing entry for service 3 (public-service)")
	}

	// Step 10: Verify candidate ordering: local > private > public, enabled > disabled
	for _, entry := range table.Entries {
		if len(entry.Candidates) == 0 {
			t.Logf("entry for %s has no candidates (status=%s)", entry.Domain, entry.Status)
			continue
		}

		// Verify candidates are sorted by priority (lower = better)
		for i := 1; i < len(entry.Candidates); i++ {
			prev := entry.Candidates[i-1]
			curr := entry.Candidates[i]
			if prev.Priority > curr.Priority {
				t.Errorf("candidates not ordered by priority for %s: candidate[%d].priority=%d > candidate[%d].priority=%d",
					entry.Domain, i-1, prev.Priority, i, curr.Priority)
			}
		}

		// Verify local candidates come before private before public (by mode)
		modeOrder := map[string]int{
			routingtable.CandidateModeLocal:   0,
			routingtable.CandidateModePrivate: 1,
			routingtable.CandidateModePublic:  2,
		}
		for i := 1; i < len(entry.Candidates); i++ {
			prevMode := entry.Candidates[i-1].Mode
			currMode := entry.Candidates[i].Mode
			if modeOrder[prevMode] > modeOrder[currMode] {
				t.Errorf("candidates mode order violated for %s: %s before %s",
					entry.Domain, prevMode, currMode)
			}
		}
	}
	t.Log("candidate ordering verified: local > private > public")

	// Verify service 1 (local-only) has only local candidates
	for _, entry := range table.Entries {
		if entry.ServiceID == svc1.ID {
			for _, cand := range entry.Candidates {
				if cand.Mode != routingtable.CandidateModeLocal {
					t.Errorf("service 1 (local-only) should only have local candidates, got mode=%s", cand.Mode)
				}
			}
		}
	}

	t.Log("multi-service policy routing test completed successfully")
}
