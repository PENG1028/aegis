package trace

import "aegis/internal/provider"

// AccessPathTrace represents the complete access path for a domain, SNI host, or route.
type AccessPathTrace struct {
	Input       string        `json:"input"`
	InputType   string        `json:"input_type"` // domain | sni | route_id
	TraceStatus string        `json:"trace_status"` // complete | incomplete | not_found | error
	Steps       []TraceStep   `json:"steps"`
	FinalTarget *TargetInfo   `json:"final_target,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
	Errors      []string      `json:"errors,omitempty"`
}

// TraceStep represents one hop in the access path.
type TraceStep struct {
	Order              int                          `json:"order"`
	Component          string                       `json:"component"` // listener | edge_mux | caddy | route | endpoint | target | provider
	Name               string                       `json:"name"`
	Status             string                       `json:"status"` // matched | skipped | missing | error
	Detail             string                       `json:"detail"`
	Address            string                       `json:"address,omitempty"`
	ProviderDiagnostic *provider.ProviderDiagnostic `json:"provider_diagnostic,omitempty"` // v1.7W: full diagnostic for provider steps
}

// TargetInfo describes the final backend target.
type TargetInfo struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Protocol     string `json:"protocol"`
	Reachable    *bool  `json:"reachable,omitempty"`
	ConnectError string `json:"connect_error,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
}

// Trace status constants.
const (
	StatusComplete  = "complete"
	StatusIncomplete = "incomplete"
	StatusNotFound  = "not_found"
	StatusError     = "error"
)

// Connectivity check error codes.
const (
	ErrTargetUnreachable      = "TARGET_UNREACHABLE"
	ErrTargetTimeout          = "TARGET_TIMEOUT"
	ErrTargetDNSFailed        = "TARGET_DNS_FAILED"
	ErrTargetConnRefused      = "TARGET_CONNECTION_REFUSED"
	ErrTraceNotFound          = "TRACE_NOT_FOUND"
	ErrTraceIncomplete        = "TRACE_INCOMPLETE"
)
