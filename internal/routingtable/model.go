package routingtable

// RouteStatus constants for routing table entries.
const (
	StatusAvailable          = "available"
	StatusUnavailable        = "unavailable"
	StatusDisabled           = "disabled"
	StatusMissingEndpoint    = "missing_endpoint"
	StatusMissingGateway     = "missing_gateway"
	StatusMissingGatewayLink = "missing_gateway_link"
	StatusTopologyUnreachable = "topology_unreachable"
	StatusPublicNotAllowed    = "public_not_allowed"
	StatusPolicyRejected      = "policy_rejected"
)

// CandidateMode constants.
const (
	CandidateModeLocal         = "local_gateway"
	CandidateModePrivate       = "private_gateway"
	CandidateModePublic        = "public_gateway"
)

// RoutingTableEntry is a single entry in a node's routing table.
type RoutingTableEntry struct {
	Domain       string        `json:"domain"`
	RouteID      string        `json:"route_id"`
	ServiceID    string        `json:"service_id"`
	EndpointID   string        `json:"endpoint_id"`
	FromNodeID   string        `json:"from_node_id"`
	TargetNodeID string        `json:"target_node_id"`
	TargetLocalHost string    `json:"target_local_host,omitempty"`
	TargetLocalPort int       `json:"target_local_port,omitempty"`
	Protocol     string        `json:"protocol"`
	GatewayPolicy GatewayPolicyInfo `json:"gateway_policy"`
	Candidates   []Candidate   `json:"candidates"`
	Status       string        `json:"status"`
	UnavailableReason string   `json:"unavailable_reason,omitempty"`
}

// GatewayPolicyInfo is the resolved policy metadata for a routing entry.
type GatewayPolicyInfo struct {
	Mode               string `json:"mode"`
	RequireGatewayLink bool   `json:"require_gateway_link"`
	RequireRelay       bool   `json:"require_relay"`
	PreserveHost       bool   `json:"preserve_host"`
	TLSMode            string `json:"tls_mode"`
}

// Candidate is a possible gateway path for a routing entry.
type Candidate struct {
	Mode              string `json:"mode"`
	GatewayID         string `json:"gateway_id"`
	GatewayURL        string `json:"gateway_url"`
	Priority          int    `json:"priority"`
	RequiresGatewayLink bool `json:"requires_gateway_link"`
	GatewayLinkID     string `json:"gateway_link_id,omitempty"`
}

// RoutingTable is the complete routing table for a single node.
type RoutingTable struct {
	NodeID    string              `json:"node_id"`
	Revision  int                 `json:"revision"`
	Entries   []RoutingTableEntry `json:"entries"`
	Warnings  []string            `json:"warnings"`
}

// GenerateWarning is a non-fatal issue during generation.
type GenerateWarning struct {
	RouteID string `json:"route_id"`
	Message string `json:"message"`
}

// ============================================================================
// Generator Input Types
// ============================================================================

// GenerateInput is all data required to generate a routing table for a node.
type GenerateInput struct {
	FromNodeID     string
	AllNodes       []NodeInfo
	AllServices    []ServiceInfo
	AllRoutes      []RouteInfo
	AllEndpoints   []EndpointInfo
	AllGateways    []GatewayInfo
	TopologyEdges  []TopologyEdgeInfo
	GatewayLinks   []GatewayLinkInfo
	ResolvePolicy  func(routeID, serviceID string) (policy PolicyInfo, err error)
}

// NodeInfo is a minimal node representation for routing.
type NodeInfo struct {
	NodeID string
}

// ServiceInfo is a minimal service representation for routing.
type ServiceInfo struct {
	ServiceID string
}

// RouteInfo is a minimal route representation for routing.
type RouteInfo struct {
	RouteID   string
	Domain    string
	ServiceID string
}

// EndpointInfo is a minimal endpoint representation for routing.
type EndpointInfo struct {
	EndpointID  string
	ServiceID   string
	Type        string
	Address     string
	NodeID      string
}

// GatewayInfo is a minimal gateway representation for routing.
type GatewayInfo struct {
	GatewayID         string
	NodeID            string
	Name              string
	Type              string
	Provider          string
	Host              string
	Port              int
	Scheme            string
	PublicAccessible  bool
	PrivateAccessible bool
	Enabled           bool
	Priority          int
	Status            string
}

// TopologyEdgeInfo is a minimal topology edge for routing.
type TopologyEdgeInfo struct {
	FromNodeID       string
	ToNodeID         string
	PrivateReachable bool
	PublicReachable  bool
	Status           string
	GatewayLinkID    string
}

// GatewayLinkInfo is a minimal gateway link for routing.
type GatewayLinkInfo struct {
	ID              string
	SourceNodeID    string
	TargetNodeID    string
}

// PolicyInfo is the resolved policy for routing decisions.
type PolicyInfo struct {
	Source             string
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
