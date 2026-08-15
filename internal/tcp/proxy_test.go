package tcp

import (
	"testing"

	"aegis/internal/addr"
)

// TestResolveTargetUnixSocket is a regression test: "unix:///path" targets
// were parsed as TCP with host "unix:///path" because addr.Parse returns
// Port==0 for unix addresses and the parser required Port > 0 — every dial
// failed, silently killing unix-socket forwarding.
func TestResolveTargetUnixSocket(t *testing.T) {
	a := resolveTarget("unix:///run/app.sock", 0)
	if !a.IsUnix() {
		t.Fatalf("unix:// target parsed as network %q host %q — unix forwarding is broken", a.Network, a.Host)
	}
	if a.Path != "/run/app.sock" {
		t.Fatalf("path = %q, want /run/app.sock", a.Path)
	}
}

// TestResolveTargetBareUnixPath keeps the bare-absolute-path form working.
func TestResolveTargetBareUnixPath(t *testing.T) {
	a := resolveTarget("/run/app.sock", 0)
	if !a.IsUnix() {
		t.Fatalf("bare path parsed as %q, want unix", a.Network)
	}
}

// TestResolveTargetTCP keeps host:port and tcp:// forms intact.
func TestResolveTargetTCP(t *testing.T) {
	a := resolveTarget("10.0.0.5:3306", 0)
	if a.Network != addr.NetTCP || a.Host != "10.0.0.5" || a.Port != 3306 {
		t.Fatalf("host:port parsed as %+v", a)
	}

	a2 := resolveTarget("tcp://10.0.0.5:3306", 0)
	if !a2.IsTCP() || a2.Port != 3306 {
		t.Fatalf("tcp:// parsed as %+v", a2)
	}
}
