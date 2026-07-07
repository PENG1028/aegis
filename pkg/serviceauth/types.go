package serviceauth

// This file defines the SDK-side types.

// RegisterRequest is sent to the cluster on startup.
type RegisterRequest struct {
	ServiceName string `json:"service_name"`
	PublicKey   string `json:"public_key"` // Ed25519 public key (base64)
}

// RegisterResponse is returned after successful registration.
type RegisterResponse struct {
	ServiceID    string            `json:"service_id"`
	PublicKeys   map[string]string `json:"public_keys"`    // name → public_key
	Groups       []ServiceGroup    `json:"groups,omitempty"`
	Policies     []Policy          `json:"policies,omitempty"`
	Blocklist    []BlocklistEntry  `json:"blocklist"`
	BlVersion    int64             `json:"bl_version"`
	CatVersion   int64             `json:"cat_version"`
	SyncInterval int               `json:"sync_interval"`
}

// ServiceGroup is a named collection of services.
type ServiceGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// Policy is an access control rule.
type Policy struct {
	ID            string `json:"id"`
	Subject       string `json:"subject"`
	TargetService string `json:"target_service"`
	Action        string `json:"action"`
	Effect        string `json:"effect"`
	Priority      int    `json:"priority"`
	Enabled       bool   `json:"enabled"`
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
	PublicKeys   map[string]string `json:"public_keys,omitempty"`
	Groups       []ServiceGroup    `json:"groups,omitempty"`
	Policies     []Policy          `json:"policies,omitempty"`
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
