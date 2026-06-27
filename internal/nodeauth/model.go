package nodeauth

import "time"

// JoinToken represents a one-time registration token for new nodes.
type JoinToken struct {
	ID               string    `json:"id"`
	TokenHash        string    `json:"-"`                 // SHA-256 of raw token
	Name             string    `json:"name"`              // human-readable description
	AllowedRoles     []string  `json:"allowed_roles"`     // empty = any role
	ExpectedNodeName string    `json:"expected_node_name"` // empty = any name
	AllowedSourceCIDR string   `json:"allowed_source_cidr"` // empty = any IP
	ExpiresAt        time.Time `json:"expires_at"`
	UsedAt           time.Time `json:"used_at,omitempty"`
	UsedByNodeID     string    `json:"used_by_node_id,omitempty"`
	RevokedAt        time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// IsExpired returns true if the token is past its expiration time.
func (t *JoinToken) IsExpired() bool {
	return !t.ExpiresAt.IsZero() && time.Now().After(t.ExpiresAt)
}

// IsUsed returns true if the token has already been used.
func (t *JoinToken) IsUsed() bool {
	return !t.UsedAt.IsZero()
}

// IsRevoked returns true if the token was revoked before use.
func (t *JoinToken) IsRevoked() bool {
	return !t.RevokedAt.IsZero()
}

// IsValid returns true if the token can still be used for registration.
func (t *JoinToken) IsValid() bool {
	if t.IsExpired() {
		return false
	}
	if t.IsUsed() {
		return false
	}
	if t.IsRevoked() {
		return false
	}
	return true
}

// NodeCredential represents a long-term credential for a registered node.
type NodeCredential struct {
	ID         string    `json:"id"`
	NodeID     string    `json:"node_id"`
	TokenHash  string    `json:"-"` // SHA-256 of raw node credential
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
}

// IsRevoked returns true if the credential has been revoked.
func (c *NodeCredential) IsRevoked() bool {
	return !c.RevokedAt.IsZero()
}

// CreateJoinTokenInput is the input for creating a join token.
type CreateJoinTokenInput struct {
	Name             string   `json:"name"`
	AllowedRoles     []string `json:"allowed_roles"`
	ExpectedNodeName string   `json:"expected_node_name"`
	AllowedSourceCIDR string  `json:"allowed_source_cidr"`
	ExpiresInSeconds int      `json:"expires_in_seconds"` // 0 = use default (3600)
}

// JoinRequest is the payload for POST /api/node/v1/join.
type JoinRequest struct {
	JoinToken    string   `json:"join_token"`
	NodeName     string   `json:"node_name"`
	Roles        []string `json:"roles"`
	Hostname     string   `json:"hostname"`
	OS           string   `json:"os"`
	Arch         string   `json:"arch"`
	AgentVersion string   `json:"agent_version"`
	PublicIP     string   `json:"public_ip,omitempty"`
	PrivateIP    string   `json:"private_ip,omitempty"`
}

// JoinResponse is the response for successful registration.
type JoinResponse struct {
	NodeID           string   `json:"node_id"`
	NodeToken        string   `json:"node_token"`
	NodeTokenRedacted bool    `json:"node_token_redacted"`
	Status           string   `json:"status"`
	HeartbeatAfter   int      `json:"heartbeat_after_seconds"`
}

// HeartbeatGateway is a gateway entry in the heartbeat payload.
type HeartbeatGateway struct {
	GatewayID         string `json:"gateway_id,omitempty"`
	Name              string `json:"name"`
	Type              string `json:"type"`
	Provider          string `json:"provider"`
	BindAddr          string `json:"bind_addr"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Scheme            string `json:"scheme"`
	PublicAccessible  bool   `json:"public_accessible"`
	PrivateAccessible bool   `json:"private_accessible"`
	Enabled           bool   `json:"enabled"`
	Status            string `json:"status"`
	LastError         string `json:"last_error,omitempty"`
}

// HeartbeatRequest is the payload for POST /api/node/v1/heartbeat.
type HeartbeatRequest struct {
	NodeID          string            `json:"node_id"`
	AgentVersion    string            `json:"agent_version"`
	Hostname        string            `json:"hostname"`
	PublicIP        string            `json:"public_ip"`
	PrivateIP       string            `json:"private_ip"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Listeners       []interface{}     `json:"listeners,omitempty"`
	ProviderStatus  interface{}       `json:"provider_status,omitempty"`
	RelayStatus     interface{}       `json:"relay_status,omitempty"`
	LocalGWStatus   interface{}       `json:"local_gateway_status,omitempty"`
	Gateways        []HeartbeatGateway `json:"gateways,omitempty"`
	AppliedRevision int               `json:"applied_revision"`
	Status          string            `json:"status"`
	LastError       string            `json:"last_error,omitempty"`
}

// HeartbeatResponse is the response for heartbeat.
type HeartbeatResponse struct {
	NodeID            string `json:"node_id"`
	Status            string `json:"status"`
	LatestRevision    int    `json:"latest_revision"`
	DesiredStateAvail bool   `json:"desired_state_available"`
	NodeIsOutdated    bool   `json:"node_is_outdated,omitempty"`
}
