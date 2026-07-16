package transparent

import "fmt"

// RedirectRule defines a transparent interception rule for one IP:port pair.
type RedirectRule struct {
	ID              string `json:"id"`
	OriginalIP      string `json:"original_ip"`       // the IP being intercepted (e.g. 192.168.1.100)
	OriginalPort    int    `json:"original_port"`     // the port being intercepted (e.g. 9100)
	LocalProxyPort  int    `json:"local_proxy_port"`  // local port to redirect to (auto-assigned or explicit)
	TargetServiceID string `json:"target_service_id"` // which Aegis service handles this traffic
	TargetNodeID    string `json:"target_node_id"`    // which node the service endpoint is on
	TargetEdgeAddr  string `json:"target_edge_addr"`  // remote Aegis edge address, e.g. 203.0.113.10:80
	Description     string `json:"description"`       // human-readable
}

// Validate checks the rule for basic correctness.
func (r *RedirectRule) Validate() error {
	if r.OriginalIP == "" {
		return fmt.Errorf("original_ip is required")
	}
	if r.OriginalPort <= 0 || r.OriginalPort > 65535 {
		return fmt.Errorf("original_port must be 1-65535")
	}
	if r.TargetServiceID == "" {
		return fmt.Errorf("target_service_id is required")
	}
	return nil
}

// Key returns a unique key for the original destination.
func (r *RedirectRule) Key() string {
	return fmt.Sprintf("%s:%d", r.OriginalIP, r.OriginalPort)
}

// RuleStatus reports the current state of a redirect rule.
type RuleStatus struct {
	Rule      RedirectRule `json:"rule"`
	Active    bool         `json:"active"`
	ProxyPort int          `json:"proxy_port"`
	BytesIn   int64        `json:"bytes_in,omitempty"`
	BytesOut  int64        `json:"bytes_out,omitempty"`
	Error     string       `json:"error,omitempty"`
}
