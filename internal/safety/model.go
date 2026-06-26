package safety

// RiskCode constants — v1.8A Egress Trace & Path Diagnosis
const (
	RiskPublicDomainBounce     = "PUBLIC_DOMAIN_BOUNCE"
	RiskPublicTargetEgress     = "PUBLIC_TARGET_EGRESS"
	RiskGatewayLinkBypass      = "GATEWAY_LINK_BYPASS_RISK"
	RiskSelfLoop               = "SELF_LOOP"
	RiskDomainResolvesToSelf   = "DOMAIN_RESOLVES_TO_SELF"
	RiskInternalTargetAvail    = "INTERNAL_TARGET_AVAILABLE"
	RiskUnknownDomain          = "UNKNOWN_DOMAIN"
)

// Severity constants.
const (
	SevError   = "error"
	SevWarning = "warning"
	SevInfo    = "info"
)

// IPClassification describes a resolved IP address.
type IPClassification string

const (
	IPPublic   IPClassification = "public"
	IPPrivate  IPClassification = "private"
	IPLoopback IPClassification = "loopback"
	IPInvalid  IPClassification = "invalid"
	IPHostname IPClassification = "hostname"
)

// Risk describes a detected safety risk.
type Risk struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// RouteSafetyResult is the output of a route safety check.
type RouteSafetyResult struct {
	RouteID                string   `json:"route_id"`
	Domain                 string   `json:"domain"`
	TargetHost             string   `json:"target_host"`
	TargetPort             int      `json:"target_port"`
	IPClassification       string   `json:"ip_classification"`
	IsCurrentNodeAddress   bool     `json:"is_current_node_address"`
	IsGatewayListenerTarget bool    `json:"is_gateway_listener_target"`
	HasGatewayLink         bool     `json:"has_gateway_link"`
	GatewayLinkID          string   `json:"gateway_link_id,omitempty"`
	GatewayLinkRequired    bool     `json:"gateway_link_required,omitempty"`
	Risks                  []Risk   `json:"risks"`
	Recommendation         string   `json:"recommendation,omitempty"`
}

// EgressTraceResult is the output of an egress trace.
type EgressTraceResult struct {
	Domain                 string   `json:"domain"`
	ResolvedIPs            []string `json:"resolved_ips"`
	IPClassification       string   `json:"ip_classification"`
	IsManagedDomain        bool     `json:"is_aegis_managed_domain"`
	MatchedRouteID         string   `json:"matched_route_id,omitempty"`
	GatewayNode            string   `json:"gateway_node"`
	CurrentNode            string   `json:"current_node"`
	TargetHost             string   `json:"target_host,omitempty"`
	TargetPort             int      `json:"target_port,omitempty"`
	HasGatewayLink         bool     `json:"has_gateway_link"`
	GatewayLinkID          string   `json:"gateway_link_id,omitempty"`
	IsCurrentNodeAddress   bool     `json:"is_current_node_address"`
	IsGatewayListenerTarget bool    `json:"is_gateway_listener_target"`
	InternalTargetAvail    bool     `json:"internal_target_available"`
	Risks                  []Risk   `json:"risks"`
	Recommendation         string   `json:"recommendation,omitempty"`
}
