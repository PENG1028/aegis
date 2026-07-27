package safety

import (
	"net"
	"strconv"
	"strings"
)

// ClassifyIP classifies an IP address into a category.
// Priority: invalid → hostname → loopback → private → public.
// "self" classification is no longer returned — use IsCurrentNodeAddress instead.
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

	// Priority: loopback → private → public
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

// IsCurrentNodeAddress returns true if the host matches any of the given node IPs.
func IsCurrentNodeAddress(host string, nodeIPs []string) bool {
	ip := net.ParseIP(NormalizeHost(host))
	if ip == nil {
		return false
	}
	for _, nodeIP := range nodeIPs {
		if ip.Equal(net.ParseIP(nodeIP)) {
			return true
		}
	}
	return false
}

// IsPublicIP returns true if the IP is a public unicast IP.
func IsPublicIP(host string) bool {
	return ClassifyIP(host, nil) == IPPublic
}

// IsPrivateIP returns true if the IP is private, loopback, or link-local.
func IsPrivateIP(host string) bool {
	c := ClassifyIP(host, nil)
	return c == IPPrivate || c == IPLoopback
}

// IsPrivateOrLinkLocal returns true if the IP is private, loopback, or link-local.
// Link-local addresses (169.254.x.x / fe80::) are non-routable and treated as internal.
func IsPrivateOrLinkLocal(host string) bool {
	if IsPrivateIP(host) {
		return true
	}
	ip := net.ParseIP(NormalizeHost(host))
	if ip == nil {
		return false
	}
	return ip.IsLinkLocalUnicast()
}

// NormalizeHost strips the port from a "host:port" string.
// This is THE canonical host-only extractor for the entire project.
// Do NOT create another parseHostPort/NormalizeHost/SplitHostPort variant
// that returns only the host — use this function.
// For splitting into (host, port), use SplitHostPort instead.
// Standard library: net.SplitHostPort
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

// SplitHostPort splits a "host:port" string into host and port.
// This is THE canonical host:port splitter for the entire project.
// Returns port = 0 if no port is found or the port is invalid.
// Do NOT create another parseHostPort/HostPort function — use this, or endpoint.HostPort()
// if you already have an Endpoint value.
// Standard library: net.SplitHostPort (returns string port + error)
func SplitHostPort(addr string) (host string, port int) {
	h, pStr, err := net.SplitHostPort(addr)
	if err != nil {
		return NormalizeHost(addr), 0
	}
	p, err := strconv.Atoi(pStr)
	if err != nil {
		return h, 0
	}
	return h, p
}
