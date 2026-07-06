// Package infra — infrastructure dependency detection.
//
// Detects external tools that Aegis depends on (certbot, iptables, dnsmasq).
// These are NOT gateway Providers — they don't route traffic, don't participate
// in capability matching, and don't render config through the Provider interface.
//
// Each tool gets its own file:
//
//	certbot.go   — certbot detection (also checks email config)
//	iptables.go  — iptables detection (linux: real, !linux: stub)
//	dnsmasq.go   — dnsmasq detection
package infra

// Status describes the state of an infrastructure dependency.
type Status struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Category  string `json:"category"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
	Message   string `json:"message"`
}
