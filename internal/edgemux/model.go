package edgemux

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Kind constants for edge mux rules.
const (
	KindHTTPSApp        = "https_app"
	KindProxyNode       = "proxy_node"
	KindDBProxy         = "db_proxy"
	KindTunnel          = "tunnel"
	KindUnknownTLSBackend = "unknown_tls_backend"
)

// Rule represents a single SNI-based routing rule.
type Rule struct {
	ID           string    `json:"id"`
	SNIHost      string    `json:"sni_host"`
	DeclaredKind string    `json:"declared_kind"`
	TargetHost   string    `json:"target_host"`
	TargetPort   int       `json:"target_port"`
	ServiceID    string    `json:"service_id"`
	ManagedBy    string    `json:"managed_by"` // manual | http_route
	SourceRef    string    `json:"source_ref"` // route_id when managed_by=http_route
	Status       string    `json:"status"`    // active | disabled | failed
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateRuleInput is the input for creating an edge mux rule.
type CreateRuleInput struct {
	SNIHost      string
	DeclaredKind string
	TargetHost   string
	TargetPort   int
	ServiceID    string
}

// ValidateSNIHost checks if an SNI hostname is valid.
func ValidateSNIHost(host string) error {
	if host == "" {
		return fmt.Errorf("SNI host is required")
	}
	if strings.Contains(host, "*") {
		return fmt.Errorf("wildcard SNI not supported")
	}
	if strings.Contains(host, "..") {
		return fmt.Errorf("invalid hostname")
	}
	// Basic hostname validation
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`, host)
	if !matched {
		return fmt.Errorf("invalid SNI hostname: %s", host)
	}
	return nil
}

// ValidateTarget ensures the target is a safe internal address.
func ValidateTarget(host string) (bool, string) {
	if host == "127.0.0.1" || host == "localhost" {
		return true, "loopback allowed"
	}
	// Check private IP ranges
	if strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "172.16.") ||
		strings.HasPrefix(host, "192.168.") {
		return true, "private IP allowed"
	}
	return false, "target must be 127.0.0.1 or private IP"
}
