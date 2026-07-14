package tool

import (
	"fmt"

	"aegis/internal/hostdep"
)

// Host-dependency wrappers: each installable infra tool implements
// hostdep.HostDependency so it shares the detect + install contract with the
// gateway providers. Detection delegates to the existing Detect* functions;
// installation routes through hostdep.InstallPackage (the single install flow).

// Dependencies returns the host dependencies Aegis manages outside the gateway
// providers, for a given ACME email (used by the ACME dependency's detection).
func Dependencies(acmeEmail string) []hostdep.HostDependency {
	return []hostdep.HostDependency{
		ACMEDep{Email: acmeEmail},
		IPTablesDep{},
		DNSMasqDep{},
	}
}

// ─── ACME (embedded lego — not installable) ─────────────────────────────────

type ACMEDep struct{ Email string }

func (a ACMEDep) Name() string          { return "acme" }
func (a ACMEDep) Detect() hostdep.Status { return DetectACME(a.Email) }
func (ACMEDep) Installable() bool         { return false }
func (ACMEDep) Install() error {
	return fmt.Errorf("acme is embedded (lego) — nothing to install")
}

// ─── dnsmasq (installable package) ──────────────────────────────────────────

type DNSMasqDep struct{}

func (DNSMasqDep) Name() string          { return "dnsmasq" }
func (DNSMasqDep) Detect() hostdep.Status { return DetectDNSMasq() }
func (DNSMasqDep) Installable() bool       { return true }
func (DNSMasqDep) Install() error          { return hostdep.InstallPackage("dnsmasq", "dnsmasq") }

// ─── iptables (installable package; iptables itself is not a service) ───────

type IPTablesDep struct{}

func (IPTablesDep) Name() string          { return "iptables" }
func (IPTablesDep) Detect() hostdep.Status { return DetectIPTables() }
func (IPTablesDep) Installable() bool       { return true }
func (IPTablesDep) Install() error          { return hostdep.InstallPackage("iptables", "") }
