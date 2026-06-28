package nodestate

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"aegis/internal/routingtable"
)

// Generator builds desired state JSON for all registered nodes
// by running the routing table generator against the live database state.
type Generator struct {
	stateSvc   *Service
	rtGen      *routingtable.Generator
	dataSource DataSource
}

// DataSource provides all data needed to generate per-node routing tables.
type DataSource interface {
	ListNodes() ([]NodeInfo, error)
	FindActiveRoutes() ([]RouteInfo, error)
	FindEnabledEndpoints() ([]EndpointInfo, error)
	FindAllGateways() ([]GatewayInfo, error)
	FindAllGatewayLinks() ([]GatewayLinkInfo, error)
	FindTopologyEdges() ([]TopologyEdgeInfo, error)
	ResolvePolicy(routeID, serviceID string) (PolicyInfo, error)
}

// NodeInfo is a simplified node view for routing table generation.
type NodeInfo struct {
	NodeID string
}

// RouteInfo is a simplified route view.
type RouteInfo struct {
	RouteID   string
	Domain    string
	ServiceID string
}

// EndpointInfo is a simplified endpoint view.
type EndpointInfo struct {
	EndpointID string
	ServiceID  string
	Type       string
	Address    string
	NodeID     string
}

// GatewayInfo is a simplified gateway inventory view.
type GatewayInfo struct {
	GatewayID         string
	NodeID            string
	Type              string
	Host              string
	Scheme            string
	Port              int
	Enabled           bool
	Priority          int
	PublicAccessible  bool
	PrivateAccessible bool
}

// GatewayLinkInfo is a simplified gateway link view.
type GatewayLinkInfo struct {
	ID           string
	SourceNodeID string
	TargetNodeID string
}

// TopologyEdgeInfo is a simplified topology edge view.
type TopologyEdgeInfo struct {
	FromNodeID       string
	ToNodeID         string
	PrivateReachable bool
	PublicReachable  bool
	Status           string
	GatewayLinkID    string
}

// PolicyInfo is a simplified routing policy view.
type PolicyInfo struct {
	Mode               string
	PrimaryGatewayID   string
	FallbackGatewayIDs []string
	AllowLocal         bool
	AllowPrivate       bool
	AllowPublic        bool
	RequireGatewayLink bool
	RequireRelay       bool
	PreserveHost       bool
	TLSMode            string
}

// NewGenerator creates a new desired state generator.
func NewGenerator(stateSvc *Service, dataSource DataSource) *Generator {
	return &Generator{
		stateSvc:   stateSvc,
		rtGen:      routingtable.NewGenerator(),
		dataSource: dataSource,
	}
}

// GenerateForAllNodes regenerates desired state for all registered nodes.
func (g *Generator) GenerateForAllNodes(ctx context.Context) error {
	nodes, err := g.dataSource.ListNodes()
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}

	routes, err := g.dataSource.FindActiveRoutes()
	if err != nil {
		return fmt.Errorf("list routes: %w", err)
	}

	endpoints, err := g.dataSource.FindEnabledEndpoints()
	if err != nil {
		return fmt.Errorf("list endpoints: %w", err)
	}

	gateways, err := g.dataSource.FindAllGateways()
	if err != nil {
		return fmt.Errorf("list gateways: %w", err)
	}

	links, err := g.dataSource.FindAllGatewayLinks()
	if err != nil {
		return fmt.Errorf("list gateway links: %w", err)
	}

	topoEdges, err := g.dataSource.FindTopologyEdges()
	if err != nil {
		return fmt.Errorf("list topology edges: %w", err)
	}

	for _, node := range nodes {
		if err := g.generateForNode(ctx, node, routes, endpoints, gateways, links, topoEdges); err != nil {
			log.Printf("[desired-state] generate for node %s failed: %v", node.NodeID, err)
			continue
		}
	}

	return nil
}

func (g *Generator) generateForNode(
	ctx context.Context,
	node NodeInfo,
	routes []RouteInfo,
	endpoints []EndpointInfo,
	gateways []GatewayInfo,
	links []GatewayLinkInfo,
	topoEdges []TopologyEdgeInfo,
) error {
	// Convert domain types to routingtable types
	rtRoutes := make([]routingtable.RouteInfo, len(routes))
	for i, r := range routes {
		rtRoutes[i] = routingtable.RouteInfo{
			RouteID:   r.RouteID,
			Domain:    r.Domain,
			ServiceID: r.ServiceID,
		}
	}

	rtEndpoints := make([]routingtable.EndpointInfo, len(endpoints))
	for i, ep := range endpoints {
		rtEndpoints[i] = routingtable.EndpointInfo{
			EndpointID: ep.EndpointID,
			ServiceID:  ep.ServiceID,
			Type:       ep.Type,
			Address:    ep.Address,
			NodeID:     ep.NodeID,
		}
	}

	nodesAll, _ := g.dataSource.ListNodes()
	rtNodes := make([]routingtable.NodeInfo, len(nodesAll))
	for i, n := range nodesAll {
		rtNodes[i] = routingtable.NodeInfo{
			NodeID: n.NodeID,
		}
	}

	rtGateways := make([]routingtable.GatewayInfo, len(gateways))
	for i, gw := range gateways {
		rtGateways[i] = routingtable.GatewayInfo{
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

	rtLinks := make([]routingtable.GatewayLinkInfo, len(links))
	for i, l := range links {
		rtLinks[i] = routingtable.GatewayLinkInfo{
			ID:           l.ID,
			SourceNodeID: l.SourceNodeID,
			TargetNodeID: l.TargetNodeID,
		}
	}

	rtTopoEdges := make([]routingtable.TopologyEdgeInfo, len(topoEdges))
	for i, e := range topoEdges {
		rtTopoEdges[i] = routingtable.TopologyEdgeInfo{
			FromNodeID:       e.FromNodeID,
			ToNodeID:         e.ToNodeID,
			PrivateReachable: e.PrivateReachable,
			PublicReachable:  e.PublicReachable,
			Status:           e.Status,
			GatewayLinkID:    e.GatewayLinkID,
		}
	}

	resolvePolicy := func(routeID, serviceID string) (routingtable.PolicyInfo, error) {
		p, err := g.dataSource.ResolvePolicy(routeID, serviceID)
		if err != nil {
			return routingtable.PolicyInfo{}, err
		}
		return routingtable.PolicyInfo{
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

	input := routingtable.GenerateInput{
		FromNodeID:    node.NodeID,
		AllNodes:      rtNodes,
		AllRoutes:     rtRoutes,
		AllEndpoints:  rtEndpoints,
		AllGateways:   rtGateways,
		GatewayLinks:  rtLinks,
		TopologyEdges: rtTopoEdges,
		ResolvePolicy: resolvePolicy,
	}

	rt, err := g.rtGen.Generate(input)
	if err != nil {
		return fmt.Errorf("generate routing table: %w", err)
	}

	stateJSON, err := json.Marshal(map[string]interface{}{
		"local_routing_table": rt.Entries,
		"node_id":             node.NodeID,
	})
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	_, err = g.stateSvc.CreateDesiredState(CreateDesiredStateInput{
		NodeID:    node.NodeID,
		StateJSON: string(stateJSON),
		Reason:    "auto-generated after mutation",
		CreatedBy: "system",
	})
	if err != nil {
		return fmt.Errorf("create desired state: %w", err)
	}

	return nil
}
