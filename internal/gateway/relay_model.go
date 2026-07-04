// Package relay provides Managed Egress Relay — forcing Aegis managed domains
// through gateway listeners instead of allowing direct access to remote target ports.
package gateway

import (
	"net"
	"strconv"
)

// RelayMode enumerates the possible relay path modes.
type RelayMode string

const (
	ModeLocalGateway         RelayMode = "local_gateway"
	ModePrivateGateway       RelayMode = "private_gateway"
	ModePublicGateway        RelayMode = "public_gateway"
	ModeExternalPassthrough  RelayMode = "external_passthrough"
	ModeUnavailable          RelayMode = "unavailable"
)

// Risk represents a safety risk associated with this relay path.
type Risk struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// RelayResult is the output of ResolveManagedRelay.
type RelayResult struct {
	Domain                string   `json:"domain"`
	Managed               bool     `json:"managed"`
	Mode                  string   `json:"mode"`
	FromNodeID            string   `json:"from_node_id"`
	FromNodeHostname      string   `json:"from_node_hostname,omitempty"`
	TargetNodeID          string   `json:"target_node_id,omitempty"`
	TargetNodeHostname    string   `json:"target_node_hostname,omitempty"`
	GatewayURL            string   `json:"gateway_url,omitempty"`
	GatewayPort           int      `json:"gateway_port,omitempty"`
	GatewayHost           string   `json:"gateway_host,omitempty"`
	RouteID               string   `json:"route_id,omitempty"`
	ServiceID             string   `json:"service_id,omitempty"`
	EndpointID            string   `json:"endpoint_id,omitempty"`
	GatewayLinkID         string   `json:"gateway_link_id,omitempty"`
	DirectTargetSuppressed bool    `json:"direct_target_suppressed"`
	FinalLocalTarget      string   `json:"final_local_target,omitempty"`
	Risks                 []Risk   `json:"risks"`
	Recommendation        string   `json:"recommendation,omitempty"`
	Error                 string   `json:"error,omitempty"`
	ErrorDetail           string   `json:"error_detail,omitempty"`
}

// AddRisk adds a risk to the result.
func (r *RelayResult) AddRisk(code, severity, message string) {
	r.Risks = append(r.Risks, Risk{Code: code, Severity: severity, Message: message})
}

// ParseHostPort splits "host:port" and returns host, port.
// Returns host, 0 if port is missing or invalid.
// Delegates to safety.SplitHostPort — the project's canonical host:port splitter.
// Do NOT rewrite this function; use safety.SplitHostPort or endpoint.HostPort() instead.
func ParseHostPort(addr string) (string, int) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 0
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

// riskCodes used in relay decisions.
const (
	RiskTargetGatewayUnreachable = "TARGET_GATEWAY_UNREACHABLE"
	RiskGatewayLinkRequired      = "GATEWAY_LINK_REQUIRED"
	RiskSelfLoop                 = "SELF_LOOP"
	RiskPublicTargetEgress       = "PUBLIC_TARGET_EGRESS"
)
