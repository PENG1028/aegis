package apply

// ApplyPlan holds the complete result of a configuration apply operation.
// v1.8L: Routes field removed — proxy.RouteConfig is no longer the data model.
// TopologyPlanner and Provider work with RouteSpec directly.
type ApplyPlan struct {
	Warnings           []ApplyWarning `json:"warnings"`
	RenderedConfig     string         `json:"rendered_config"`
	ConfigPath         string         `json:"config_path"`
	TempPath           string         `json:"temp_path,omitempty"`
	BackupPath         string         `json:"backup_path,omitempty"`
	RouteCount         int            `json:"route_count"`
	ManagedDomainCount int            `json:"managed_domain_count"`
	SkippedCount       int            `json:"skipped_count"`
}

// ApplyWarning represents a warning during apply planning.
type ApplyWarning struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Target   string `json:"target"`
	Severity string `json:"severity"` // info | warning | critical
}

// Warning code constants.
const (
	WarningServiceDisabled        = "SERVICE_DISABLED"
	WarningNoAvailableEndpoint    = "NO_AVAILABLE_ENDPOINT"
	WarningManagedDomainNotActive = "MANAGED_DOMAIN_NOT_ACTIVE"
	WarningRouteSkipped           = "ROUTE_SKIPPED"
	WarningEndpointUnreachable    = "ENDPOINT_UNREACHABLE"
	WarningDNSPending             = "DNS_PENDING"
)
