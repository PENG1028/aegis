package serviceauth

// This file defines the SDK-side types. They are a subset of the server-side
// types in internal/serviceauth/model.go — deliberately duplicated so the SDK
// has zero imports from aegis/internal/.

// APIDef describes one API endpoint a service exposes.
type APIDef struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Method string `json:"method"`
	Params string `json:"params,omitempty"`
}

// RegisterRequest is sent to the cluster on startup.
type RegisterRequest struct {
	ServiceName string   `json:"service_name"`
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	NodeHost    string   `json:"node_host"`
	APIs        []APIDef `json:"apis"`
	PublicKey   string   `json:"public_key"` // Ed25519 public key (base64)
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	ServiceID     string            `json:"service_id"`
	Instances     []ServiceInstance `json:"instances"`
	PublicKeys    map[string]string `json:"public_keys"` // name → public_key
	APIs          []APIDef          `json:"apis"`
	Blocklist     []BlocklistEntry  `json:"blocklist"`
	BlVersion     int64             `json:"bl_version"`
	CatVersion    int64             `json:"cat_version"`
	SyncInterval  int               `json:"sync_interval"`
}

// ServiceInstance is a lightweight view of a service endpoint.
type ServiceInstance struct {
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	NodeHost string `json:"node_host"`
}

// BlocklistEntry records a blocked service or API.
type BlocklistEntry struct {
	ID        string `json:"id"`
	ServiceID string `json:"service_id"`
	APIName   string `json:"api_name"`
	Reason    string `json:"reason"`
	Version   int64  `json:"version"`
}

// SyncResponse is returned by the sync endpoint.
type SyncResponse struct {
	Blocklist    []BlocklistEntry  `json:"blocklist,omitempty"`
	BlVersion    int64             `json:"bl_version"`
	NewInstances []ServiceInstance `json:"new_instances,omitempty"`
	PublicKeys   map[string]string `json:"public_keys,omitempty"` // name → public_key
	RemovedIDs   []string          `json:"removed_ids,omitempty"`
	CatVersion   int64             `json:"cat_version"`
	NotModified  bool              `json:"not_modified"`
}

// ReportRequest carries an async call-log entry.
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

// CallerInfo is injected into the request context by the Guard middleware.
type CallerInfo struct {
	ServiceName string `json:"service_name"`
	CallerHost  string `json:"caller_host"`
}
