// Package validate provides shared input validation helpers for HTTP handlers.
// All functions return a user-facing error message string, or "" if valid.
package validate

import (
	"fmt"
	"net"
	"strings"
)

// Required checks that a field is non-empty. Returns "" if valid.
func Required(value, fieldName string) string {
	if strings.TrimSpace(value) == "" {
		return fieldName + " is required"
	}
	return ""
}

// Port checks that a port number is in the valid range (1-65535).
func Port(port int, fieldName string) string {
	if port <= 0 || port > 65535 {
		return fmt.Sprintf("%s must be between 1 and 65535, got %d", fieldName, port)
	}
	return ""
}

// MaxLen checks that a string does not exceed maxLen bytes.
func MaxLen(value string, maxLen int, fieldName string) string {
	if len(value) > maxLen {
		return fmt.Sprintf("%s must be at most %d characters", fieldName, maxLen)
	}
	return ""
}

// Domain checks that a string looks like a valid domain name.
// This is a best-effort check, not a full RFC validation.
func Domain(value, fieldName string) string {
	if value == "" {
		return "" // use Required() for required check
	}
	if len(value) > 253 {
		return fmt.Sprintf("%s: domain name too long (max 253 characters)", fieldName)
	}
	if strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return fmt.Sprintf("%s: domain cannot start or end with a dot", fieldName)
	}
	if strings.Contains(value, "..") {
		return fmt.Sprintf("%s: domain cannot contain consecutive dots", fieldName)
	}
	// Check each label
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return fmt.Sprintf("%s: each domain label must be 1-63 characters", fieldName)
		}
	}
	return ""
}

// HostPort checks that a string is a valid host:port combination.
func HostPort(value, fieldName string) string {
	if value == "" {
		return "" // use Required() for required check
	}
	host, portStr, err := net.SplitHostPort(value)
	if err != nil {
		// Maybe it's just a hostname (no port)
		if strings.Contains(value, ":") {
			return fmt.Sprintf("%s: invalid host:port format: %s", fieldName, value)
		}
		return "" // bare hostname is acceptable
	}
	if host == "" {
		return fmt.Sprintf("%s: host cannot be empty in %q", fieldName, value)
	}
	port, err := net.LookupPort("tcp", portStr)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Sprintf("%s: invalid port in %q", fieldName, value)
	}
	return ""
}

// OneOf checks that a value is one of the allowed values.
func OneOf(value string, allowed []string, fieldName string) string {
	for _, a := range allowed {
		if value == a {
			return ""
		}
	}
	return fmt.Sprintf("%s must be one of: %s", fieldName, strings.Join(allowed, ", "))
}
