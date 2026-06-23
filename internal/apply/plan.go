package apply

import (
	"aegis/internal/proxy"
)

// ApplyPlan holds the complete plan for a configuration apply.
type ApplyPlan struct {
	Routes             []proxy.RouteConfig `json:"routes"`
	Warnings           []ApplyWarning      `json:"warnings"`
	RenderedConfig     string              `json:"rendered_config"`
	ConfigPath         string              `json:"config_path"`
	TempPath           string              `json:"temp_path,omitempty"`
	BackupPath         string              `json:"backup_path,omitempty"`
	RouteCount         int                 `json:"route_count"`
	ManagedDomainCount int                 `json:"managed_domain_count"`
	SkippedCount       int                 `json:"skipped_count"`
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
