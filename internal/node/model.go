package node

import "time"

// NodeRecord represents a machine identity in the Aegis cluster.
type NodeRecord struct {
	ID              string    `json:"id"`
	NodeID          string    `json:"node_id"`
	Hostname        string    `json:"hostname"`
	LocalIP         string    `json:"local_ip"`    // 127.0.0.1
	PrivateIP       string    `json:"private_ip"`  // e.g. 10.x, 172.16.x, 192.168.x (optional)
	PublicIP        string    `json:"public_ip"`   // external IP
	IsCurrent       bool      `json:"is_current"`
	IsLeader        bool      `json:"is_leader"`
	LeaderElectedAt time.Time `json:"leader_elected_at"`
	StateVersion    uint64    `json:"state_version"`
	IPMigrated      bool      `json:"ip_migrated"` // true if IP changed since last registration
	LastSeen        time.Time `json:"last_seen"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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
