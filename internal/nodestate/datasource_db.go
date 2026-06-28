package nodestate

import (
	"aegis/internal/endpoint"
	"aegis/internal/gateway"
	"log"
	gatewaylink "aegis/internal/gateway_link"
	"aegis/internal/node"
	"aegis/internal/route"
	"aegis/internal/routingpolicy"
	"aegis/internal/topology"
)

// DBDataSource implements DataSource using the real database repositories.
type DBDataSource struct {
	NodeRepo       *node.Repository
	RouteRepo      *route.Repository
	EndpointRepo   *endpoint.Repository
	GatewayInvRepo *gateway.InventoryRepository
	GWLinkRepo     *gatewaylink.Repository
	TopologyRepo   *topology.Repository
	PolicySvc      *routingpolicy.Service
}

// NewDBDataSource creates a new database-backed data source.
func NewDBDataSource(
	nodeRepo *node.Repository,
	routeRepo *route.Repository,
	endpointRepo *endpoint.Repository,
	gatewayInvRepo *gateway.InventoryRepository,
	gwLinkRepo *gatewaylink.Repository,
	topologyRepo *topology.Repository,
	policySvc *routingpolicy.Service,
) *DBDataSource {
	return &DBDataSource{
		NodeRepo:       nodeRepo,
		RouteRepo:      routeRepo,
		EndpointRepo:   endpointRepo,
		GatewayInvRepo: gatewayInvRepo,
		GWLinkRepo:     gwLinkRepo,
		TopologyRepo:   topologyRepo,
		PolicySvc:      policySvc,
	}
}

// ListNodes returns all registered nodes.
func (ds *DBDataSource) ListNodes() ([]NodeInfo, error) {
	nodes, err := ds.NodeRepo.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]NodeInfo, len(nodes))
	for i, n := range nodes {
		result[i] = NodeInfo{NodeID: n.NodeID}
	}
	if result == nil {
		result = []NodeInfo{}
	}
	return result, nil
}

// FindActiveRoutes returns all active routes.
func (ds *DBDataSource) FindActiveRoutes() ([]RouteInfo, error) {
	routes, err := ds.RouteRepo.FindActive()
	if err != nil {
		return nil, err
	}
	result := make([]RouteInfo, len(routes))
	for i, r := range routes {
		result[i] = RouteInfo{
			RouteID:   r.ID,
			Domain:    r.Domain,
			ServiceID: r.ServiceID,
		}
	}
	if result == nil {
		result = []RouteInfo{}
	}
	return result, nil
}

// FindEnabledEndpoints returns all enabled endpoints across active services.
func (ds *DBDataSource) FindEnabledEndpoints() ([]EndpointInfo, error) {
	routes, err := ds.RouteRepo.FindActive()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var result []EndpointInfo
	for _, rt := range routes {
		if seen[rt.ServiceID] {
			continue
		}
		seen[rt.ServiceID] = true
		eps, err := ds.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
		if err != nil {
			// Log but continue — a single service error shouldn't block all routing
			log.Printf("[desired-state] find endpoints for service %s failed: %v", rt.ServiceID, err)
			continue
		}
		for _, ep := range eps {
			result = append(result, EndpointInfo{
				EndpointID: ep.ID,
				ServiceID:  ep.ServiceID,
				Type:       ep.Type,
				Address:    ep.Address,
				NodeID:     ep.NodeID,
			})
		}
	}
	if result == nil {
		result = []EndpointInfo{}
	}
	return result, nil
}

// FindAllGateways returns all gateway inventory entries.
func (ds *DBDataSource) FindAllGateways() ([]GatewayInfo, error) {
	gws, err := ds.GatewayInvRepo.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]GatewayInfo, len(gws))
	for i, gw := range gws {
		result[i] = GatewayInfo{
			GatewayID:         gw.GatewayID,
			NodeID:            gw.NodeID,
			Type:              gw.Type,
			Host:              gw.Host,
			Scheme:            gw.Scheme,
			Port:              gw.Port,
			Enabled:           gw.Enabled,
			Priority:          gw.Priority,
			PublicAccessible:  gw.PublicAccessible,
			PrivateAccessible: gw.PrivateAccessible,
		}
	}
	if result == nil {
		result = []GatewayInfo{}
	}
	return result, nil
}

// FindAllGatewayLinks returns all gateway links.
func (ds *DBDataSource) FindAllGatewayLinks() ([]GatewayLinkInfo, error) {
	links, err := ds.GWLinkRepo.FindAll()
	if err != nil {
		return nil, err
	}
	result := make([]GatewayLinkInfo, len(links))
	for i, l := range links {
		result[i] = GatewayLinkInfo{
			ID:           l.ID,
			TargetNodeID: l.TargetNodeID,
		}
	}
	if result == nil {
		result = []GatewayLinkInfo{}
	}
	return result, nil
}

// FindTopologyEdges returns all topology edges.
func (ds *DBDataSource) FindTopologyEdges() ([]TopologyEdgeInfo, error) {
	edges, err := ds.TopologyRepo.ListEdges()
	if err != nil {
		return nil, err
	}
	result := make([]TopologyEdgeInfo, len(edges))
	for i, e := range edges {
		result[i] = TopologyEdgeInfo{
			FromNodeID:       e.FromNodeID,
			ToNodeID:         e.ToNodeID,
			PrivateReachable: e.PrivateReachable,
			PublicReachable:  e.PublicReachable,
			Status:           e.Status,
			GatewayLinkID:    e.GatewayLinkID,
		}
	}
	if result == nil {
		result = []TopologyEdgeInfo{}
	}
	return result, nil
}

// ResolvePolicy resolves the effective routing policy for a route+service pair.
func (ds *DBDataSource) ResolvePolicy(routeID, serviceID string) (PolicyInfo, error) {
	p, err := ds.PolicySvc.ResolvePolicy(routeID, serviceID)
	if err != nil {
		return PolicyInfo{}, err
	}
	return PolicyInfo{
		Mode:               p.Mode,
		PrimaryGatewayID:   p.PrimaryGatewayID,
		FallbackGatewayIDs: p.FallbackGatewayIDs,
		AllowLocal:         p.AllowLocal,
		AllowPrivate:       p.AllowPrivate,
		AllowPublic:        p.AllowPublic,
		RequireGatewayLink: p.RequireGatewayLink,
		RequireRelay:       p.RequireRelay,
		PreserveHost:       p.PreserveHost,
		TLSMode:            p.TLSMode,
	}, nil
}
