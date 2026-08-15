package udp

import (
	"testing"

	"aegis/internal/addr"
)

// TestResolveUDPTargetUnixgram is a regression test: "unixgram:///path"
// targets were parsed as UDP with host "unixgram:///path" because
// addr.Parse returns Port==0 for unixgram addresses and the parser required
// Port > 0 (the IsUnix branch inside was dead code).
func TestResolveUDPTargetUnixgram(t *testing.T) {
	a := resolveUDPTarget("unixgram:///run/app.sock", 0)
	if !a.IsUnix() {
		t.Fatalf("unixgram:// target parsed as network %q host %q — unixgram forwarding is broken", a.Network, a.Host)
	}
	if a.Path != "/run/app.sock" {
		t.Fatalf("path = %q, want /run/app.sock", a.Path)
	}
}

// TestResolveUDPTargetBareUnixPath keeps the bare-absolute-path form working.
func TestResolveUDPTargetBareUnixPath(t *testing.T) {
	a := resolveUDPTarget("/run/app.sock", 0)
	if !a.IsUnix() {
		t.Fatalf("bare path parsed as %q, want unixgram", a.Network)
	}
}

// TestResolveUDPTargetKeepsUDPForms intact.
func TestResolveUDPTargetKeepsUDPForms(t *testing.T) {
	a := resolveUDPTarget("10.0.0.5:5353", 0)
	if a.Network != addr.NetUDP || a.Host != "10.0.0.5" || a.Port != 5353 {
		t.Fatalf("host:port parsed as %+v", a)
	}
	a2 := resolveUDPTarget("udp://10.0.0.5:5353", 0)
	if !a2.IsUDP() || a2.Port != 5353 {
		t.Fatalf("udp:// parsed as %+v", a2)
	}
}
