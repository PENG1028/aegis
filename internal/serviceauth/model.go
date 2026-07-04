// Package serviceauth provides zero-config service-to-service authentication
// within a trusted cluster. Services register on startup and receive a shared
// cluster secret. Every inter-service call carries an HMAC ticket that the
// receiver verifies locally — the auth server (Aegis or serviceauthd) is never
// in the data path.
//
//	v1: cluster-wide mutual trust — any registered service may call any API
//	    of any other registered service. Admin can block services/APIs.
package serviceauth

import "time"

// ============================================================================
// Domain models (DB rows)
// ============================================================================

// ServiceRecord represents a registered service instance.
// Name + Host + Port form the natural key — same service on different hosts
// or ports are distinct instances.
type ServiceRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	NodeHost  string    `json:"node_host"` // os.Hostname() of the machine
	APIsJSON  string    `json:"apis_json"` // JSON array of APIDef
	Status    string    `json:"status"`    // "active" | "blocked"
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIDef describes one API endpoint a service exposes.
type APIDef struct {
	Name   string `json:"name"`   // logical name, e.g. "createProject"
	Path   string `json:"path"`   // e.g. "/api/v1/projects"
	Method string `json:"method"` // GET | POST | PUT | DELETE | PATCH
	Params string `json:"params,omitempty"` // JSON Schema for request params (v2)
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
// When APIName is "*" the entire service is blocked.
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
	ServiceName string   `json:"service_name"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	NodeHost    string   `json:"node_host"`
	APIs        []APIDef `json:"apis"`
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	ServiceID     string            `json:"service_id"`
	ClusterSecret string            `json:"cluster_secret"` // base64-encoded
	Instances     []ServiceInstance `json:"instances"`      // all known instances
	APIs          []APIDef          `json:"apis"`           // APIs of all services
	Blocklist     []BlocklistEntry  `json:"blocklist"`
	BlVersion     int64             `json:"bl_version"`
	CatVersion    int64             `json:"cat_version"`
	SyncInterval  int              `json:"sync_interval"` // seconds
}

// ServiceInstance is a lightweight view of a service endpoint.
type ServiceInstance struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	NodeHost string `json:"node_host"`
}

// SyncResponse is returned by the sync endpoint.
// Uses version-based change detection — when nothing changed the response
// is nearly empty.
type SyncResponse struct {
	Blocklist    []BlocklistEntry   `json:"blocklist,omitempty"`
	BlVersion    int64              `json:"bl_version"`
	NewInstances []ServiceInstance  `json:"new_instances,omitempty"`
	RemovedIDs   []string           `json:"removed_ids,omitempty"`
	CatVersion   int64              `json:"cat_version"`
	NotModified  bool               `json:"not_modified"`
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
	Caller  string `json:"caller"`
	Target  string `json:"target"`
	API     string `json:"api"`
	Count   int64  `json:"count"`
	LastSeen string `json:"last_seen"`
}

// TopologyData is the full service call topology.
type TopologyData struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// ============================================================================
// Ticket
// ============================================================================

// TicketClaims is the decoded content of a service ticket.
type TicketClaims struct {
	CallerService string `json:"caller"`
	TargetService string `json:"target"`
	TargetAPI     string `json:"api"`
	ExpiresAt     int64  `json:"exp"`
}
