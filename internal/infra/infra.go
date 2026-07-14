// Package infra — infrastructure dependency detection.
//
// Detects external tools that Aegis depends on (certbot, iptables, dnsmasq).
// These are NOT gateway Providers — they don't route traffic, don't participate
// in capability matching, and don't render config through the Provider interface.
// They ARE host dependencies: each installable tool implements
// hostdep.HostDependency so detection + installation share one contract with the
// gateway providers' install path.
//
// Each tool gets its own file:
//
//	certbot.go   — ACME/lego detection (embedded — not installable)
//	iptables.go  — iptables detection (linux: real, !linux: stub)
//	dnsmasq.go   — dnsmasq detection
package infra

import "aegis/internal/hostdep"

// Status describes the state of an infrastructure dependency. It is an alias of
// hostdep.Status so detection results are one shared type across provider/infra.
type Status = hostdep.Status
