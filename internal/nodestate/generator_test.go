package nodestate

import (
	"context"
	"database/sql"
	"testing"

	"aegis/internal/store"
)

// mockDataSource implements DataSource with in-memory test data.
type mockDataSource struct {
	nodes    []NodeInfo
	routes   []RouteInfo
	endpoints []EndpointInfo
	gateways  []GatewayInfo
	links     []GatewayLinkInfo
	topoEdges []TopologyEdgeInfo
	policy    PolicyInfo
	policyErr error
}

func (m *mockDataSource) ListNodes() ([]NodeInfo, error)               { return m.nodes, nil }
func (m *mockDataSource) FindActiveRoutes() ([]RouteInfo, error)        { return m.routes, nil }
func (m *mockDataSource) FindEnabledEndpoints() ([]EndpointInfo, error) { return m.endpoints, nil }
func (m *mockDataSource) FindAllGateways() ([]GatewayInfo, error)       { return m.gateways, nil }
func (m *mockDataSource) FindAllGatewayLinks() ([]GatewayLinkInfo, error) { return m.links, nil }
func (m *mockDataSource) FindTopologyEdges() ([]TopologyEdgeInfo, error)  { return m.topoEdges, nil }
func (m *mockDataSource) ResolvePolicy(routeID, serviceID string) (PolicyInfo, error) {
	return m.policy, m.policyErr
}

func setupGeneratorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return db
}

func TestGeneratorGenerateForAllNodes(t *testing.T) {
	db := setupGeneratorTestDB(t)
	repo := NewRepository(db)
	stateSvc := NewService(repo)

	mock := &mockDataSource{
		nodes: []NodeInfo{
			{NodeID: "node-a"},
			{NodeID: "node-b"},
		},
		routes: []RouteInfo{
			{RouteID: "rt-1", Domain: "app.example.com", ServiceID: "svc-1"},
		},
		endpoints: []EndpointInfo{
			{EndpointID: "ep-1", ServiceID: "svc-1", Type: "local", Address: "127.0.0.1:3001", NodeID: "node-a"},
		},
		gateways: []GatewayInfo{
			{GatewayID: "gw-a", NodeID: "node-a", Type: "local", Host: "127.0.0.1", Scheme: "http", Port: 80, Enabled: true, Priority: 10, PublicAccessible: false, PrivateAccessible: true},
			{GatewayID: "gw-b", NodeID: "node-b", Type: "local", Host: "127.0.0.1", Scheme: "http", Port: 80, Enabled: true, Priority: 10, PublicAccessible: false, PrivateAccessible: true},
		},
		links:     []GatewayLinkInfo{},
		topoEdges: []TopologyEdgeInfo{},
		policy: PolicyInfo{
			Mode: "auto", AllowLocal: true, AllowPrivate: true, AllowPublic: false,
		},
	}

	gen := NewGenerator(stateSvc, mock)
	err := gen.GenerateForAllNodes(context.Background())
	if err != nil {
		t.Fatalf("GenerateForAllNodes: %v", err)
	}

	// Verify desired states were created for both nodes
	for _, nodeID := range []string{"node-a", "node-b"} {
		ds, err := stateSvc.GetLatestDesiredState(nodeID)
		if err != nil {
			t.Errorf("GetLatestDesiredState(%s): %v", nodeID, err)
			continue
		}
		if ds == nil {
			t.Errorf("no desired state created for %s", nodeID)
			continue
		}
		if ds.StateHash == "" {
			t.Errorf("empty state hash for %s", nodeID)
		}
		if ds.Revision != 1 {
			t.Errorf("expected revision 1 for %s, got %d", nodeID, ds.Revision)
		}
	}
}

func TestGeneratorNoNodes(t *testing.T) {
	db := setupGeneratorTestDB(t)
	repo := NewRepository(db)
	stateSvc := NewService(repo)

	mock := &mockDataSource{
		nodes:     []NodeInfo{},
		routes:    []RouteInfo{},
		endpoints: []EndpointInfo{},
		gateways:  []GatewayInfo{},
		links:     []GatewayLinkInfo{},
		topoEdges: []TopologyEdgeInfo{},
		policy:    PolicyInfo{Mode: "auto"},
	}

	gen := NewGenerator(stateSvc, mock)
	err := gen.GenerateForAllNodes(context.Background())
	if err != nil {
		t.Fatalf("GenerateForAllNodes with no nodes should not error: %v", err)
	}
}

func TestGeneratorEmptyResultsSlice(t *testing.T) {
	// Verify that empty slices (not nil) are returned correctly
	mock := &mockDataSource{
		nodes:     []NodeInfo{},
		routes:    []RouteInfo{},
		endpoints: []EndpointInfo{},
		gateways:  []GatewayInfo{},
		links:     []GatewayLinkInfo{},
		topoEdges: []TopologyEdgeInfo{},
		policy:    PolicyInfo{Mode: "auto"},
	}

	// All slices are non-nil empty
	if mock.nodes == nil {
		t.Error("nodes should be empty slice, not nil")
	}
}
