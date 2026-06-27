package node

import "time"

// Node status constants.
const (
	StatusOnline   = "online"
	StatusOffline  = "offline"
	StatusDegraded = "degraded"
	StatusUnknown  = "unknown"
)

// NodeRole constants.
const (
	RoleControlPlane = "control_plane"
	RoleGateway      = "gateway"
	RoleWorker       = "worker"
	RoleRelay        = "relay"
	RoleDev          = "dev"
)

// NodeRecord represents a machine identity in the Aegis cluster.
type NodeRecord struct {
	ID              string    `json:"id"`
	NodeID          string    `json:"node_id"`
	Name            string    `json:"name"`             // v1.8C — human-readable name
	Role            string    `json:"role"`             // v1.8C — control_plane | gateway | worker | relay | dev
	Status          string    `json:"status"`           // v1.8C — online | offline | degraded | unknown
	Hostname        string    `json:"hostname"`
	LocalIP         string    `json:"local_ip"`         // 127.0.0.1
	PrivateIP       string    `json:"private_ip"`       // e.g. 10.x, 172.16.x, 192.168.x (optional)
	PublicIP        string    `json:"public_ip"`        // external IP
	Region          string    `json:"region,omitempty"`          // v1.8C — datacenter/region
	NetworkID       string    `json:"network_id,omitempty"`      // v1.8C — private network group
	OS              string    `json:"os,omitempty"`              // v1.8C — linux, darwin, windows
	Arch            string    `json:"arch,omitempty"`            // v1.8C — amd64, arm64
	AgentVersion    string    `json:"agent_version,omitempty"`   // v1.8C — aegis binary version
	LastHeartbeatAt time.Time `json:"last_heartbeat_at,omitempty"` // v1.8C
	LastError       string    `json:"last_error,omitempty"`      // v1.8C

	// Legacy fields (v1.7)
	IsCurrent       bool           `json:"is_current"`
	IsLeader        bool           `json:"is_leader"`
	LeaderElectedAt time.Time      `json:"leader_elected_at"`
	StateVersion    uint64         `json:"state_version"`
	IPMigrated      bool           `json:"ip_migrated"` // true if IP changed since last registration
	Capabilities    NodeCapabilities `json:"capabilities"`
	LastSeen        time.Time      `json:"last_seen"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ResolveIP returns the best available IP for the given preference order.
func (n *NodeRecord) ResolveIP(prefer string) string {
	switch prefer {
	case "local":
		if n.LocalIP != "" {
			return n.LocalIP
		}
		fallthrough
	case "private":
		if n.PrivateIP != "" {
			return n.PrivateIP
		}
		fallthrough
	case "public":
		if n.PublicIP != "" {
			return n.PublicIP
		}
		fallthrough
	default:
		// Fallback chain: private → public → local
		if n.PrivateIP != "" {
			return n.PrivateIP
		}
		if n.PublicIP != "" {
			return n.PublicIP
		}
		return n.LocalIP
	}
}
