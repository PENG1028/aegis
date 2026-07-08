package serviceauth

import "time"

// ============================================================================
// Domain models (DB rows)
// ============================================================================

// ServiceRecord represents a registered service instance.
type ServiceRecord struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`       // unique logical identity (immutable)
	PublicKey  string    `json:"public_key"`  // Ed25519 public key (base64)
	InstanceID string    `json:"instance_id"` // unique per instance, heartbeat tracking
	Status     string    `json:"status"`      // "active" | "blocked" | "inactive"
	LastSeen   time.Time `json:"last_seen"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	// Deprecated: kept for DB scan compatibility, not populated on register.
	Host     string `json:"-"`
	Port     int    `json:"-"`
	NodeHost string `json:"-"`
	APIsJSON string `json:"-"`
}

// CallLog records one inter-service call.
type CallLog struct {
	ID            string    `json:"id"`
	CallerService string    `json:"caller_service"`
	TargetService string    `json:"target_service"`
	TargetAPI     string    `json:"target_api"`
	CallerHost    string    `json:"caller_host"`
	TargetHost    string    `json:"target_host"`
	Allowed       bool      `json:"allowed"`
	LatencyMs     int       `json:"latency_ms"`
	ErrorMsg      string    `json:"error_msg,omitempty"`
	CalledAt      time.Time `json:"called_at"`
}

// BlocklistEntry records a blocked service or API.
type BlocklistEntry struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	APIName   string `json:"api_name"` // "*" = entire service
	Reason    string `json:"reason"`
	Version   int64  `json:"version"`
}

// ============================================================================
// Protocol types (SDK ↔ server)
// ============================================================================

// RegisterRequest is sent by a service on startup.
type RegisterRequest struct {
	ServiceName string `json:"service_name"`
	PublicKey   string `json:"public_key"`  // Ed25519 public key (base64)
	InstanceID  string `json:"instance_id"` // optional, for heartbeat tracking
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	ServiceID    string              `json:"service_id"`
	PublicKeys   map[string][]string `json:"public_keys"`
	Blocklist    []BlocklistEntry    `json:"blocklist"`
	BlVersion    int64               `json:"bl_version"`
	SyncInterval int                 `json:"sync_interval"`
	Warnings     []string            `json:"warnings,omitempty"`
}

// SyncResponse is returned by the sync endpoint.
type SyncResponse struct {
	Blocklist   []BlocklistEntry    `json:"blocklist,omitempty"`
	BlVersion   int64               `json:"bl_version"`
	PublicKeys  map[string][]string `json:"public_keys,omitempty"`
	NotModified bool                `json:"not_modified"`
}

// ReportRequest carries an async call-log entry from the SDK.
type ReportRequest struct {
	CallerService string `json:"caller_service"`
	TargetService string `json:"target_service"`
	TargetAPI     string `json:"target_api"`
	CallerHost    string `json:"caller_host"`
	TargetHost    string `json:"target_host"`
	Allowed       bool   `json:"allowed"`
	LatencyMs     int    `json:"latency_ms"`
	ErrorMsg      string `json:"error_msg,omitempty"`
}

// TopologyNode is one node in the service call topology graph.
type TopologyNode struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	NodeHost string `json:"node_host"`
	Status   string `json:"status"`
}

// TopologyEdge is a directed edge between two services.
type TopologyEdge struct {
	Caller   string `json:"caller"`
	Target   string `json:"target"`
	API      string `json:"api"`
	Count    int64  `json:"count"`
	LastSeen string `json:"last_seen"`
}

// TopologyData is the full service call topology.
type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}
