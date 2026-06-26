package safety

import (
	"net"
	"strings"
)

// ClassifyIP classifies an IP address into a category.
// Returns IPInvalid if the input is not a valid IP or is a hostname.
func ClassifyIP(host string, selfIPs []string) IPClassification {
	ip := net.ParseIP(host)
	if ip == nil {
		// Try resolving to see if it's an IP without the port
		// If it contains a port, strip it
		if h, _, err := net.SplitHostPort(host); err == nil {
			ip = net.ParseIP(h)
		}
	}
	if ip == nil {
		return IPHostname // not an IP, treat as hostname
	}

	// Check self IPs
	for _, selfIP := range selfIPs {
		if ip.Equal(net.ParseIP(selfIP)) {
			return IPSelf
		}
	}

	if ip.IsLoopback() {
		return IPLoopback
	}
	if ip.IsPrivate() {
		return IPPrivate
	}
	if ip.IsGlobalUnicast() {
		return IPPublic
	}
	return IPInvalid
}

// IsPublicIP returns true if the IP is a public unicast IP.
func IsPublicIP(host string) bool {
	return ClassifyIP(host, nil) == IPPublic
}

// IsPrivateIP returns true if the IP is private or loopback.
func IsPrivateIP(host string) bool {
	c := ClassifyIP(host, nil)
	return c == IPPrivate || c == IPLoopback
}

// NormalizeHost strips the port from a host:port string.
func NormalizeHost(addr string) string {
	if h, _, err := net.SplitHostPort(addr); err == nil {
		return h
	}
	// No port, check if it looks like IP:port with no brackets (IPv6 edge case)
	if strings.Count(addr, ":") > 1 {
		// Likely IPv6 without brackets, return as-is
		return addr
	}
	return addr
}
