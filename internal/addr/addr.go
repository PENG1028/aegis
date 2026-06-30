// Package addr provides a unified network address type that handles
// both TCP/UDP host:port pairs and Unix domain sockets.
//
// Supported formats:
//
//	host:port              → TCP (shorthand)
//	tcp://host:port        → TCP
//	udp://host:port        → UDP
//	unix:///absolute/path  → Unix stream socket
//	unixgram:///absolute/path → Unix datagram socket
//
// This abstraction prevents scattering format-detection logic across
// the proxy, endpoint, and config rendering layers.
package addr

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"
)

// Network constants for net.Dial / net.Listen.
const (
	NetTCP      = "tcp"
	NetUDP      = "udp"
	NetUnix     = "unix"
	NetUnixgram = "unixgram"
)

// Addr is a parsed network address that can be either a TCP/UDP host:port
// or a Unix domain socket path.
type Addr struct {
	Network string // tcp, udp, unix, unixgram
	Host    string // for TCP/UDP: IP or hostname
	Port    int    // for TCP/UDP: port number, 0 if unset
	Path    string // for Unix: absolute filesystem path
}

// Parse parses a raw address string into an Addr.
//
// Examples:
//
//	"127.0.0.1:5432"          → TCP, host=127.0.0.1, port=5432
//	"tcp://10.0.0.5:6379"     → TCP, host=10.0.0.5, port=6379
//	"udp://0.0.0.0:5353"      → UDP, host=0.0.0.0, port=5353
//	"unix:///run/pg.sock"     → Unix stream
//	"unixgram:///run/dns.sock" → Unix datagram
//	":8080"                   → TCP, host="", port=8080
func Parse(raw string) (*Addr, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty address")
	}

	// Scheme-prefixed: tcp://, udp://, unix://, unixgram://
	if idx := strings.Index(raw, "://"); idx >= 0 {
		scheme := strings.ToLower(raw[:idx])
		rest := raw[idx+3:]
		switch scheme {
		case "tcp":
			return parseHostPort(rest, NetTCP)
		case "udp":
			return parseHostPort(rest, NetUDP)
		case "unix":
			return parseUnix(rest, NetUnix)
		case "unixgram":
			return parseUnix(rest, NetUnixgram)
		default:
			return nil, fmt.Errorf("unknown address scheme: %s", scheme)
		}
	}

	// Short-hand: host:port (implicit TCP)
	if strings.Contains(raw, ":") {
		return parseHostPort(raw, NetTCP)
	}

	// Bare path starting with / → treat as Unix socket
	if strings.HasPrefix(raw, "/") {
		return parseUnix(raw, NetUnix)
	}

	return nil, fmt.Errorf("cannot parse address: %s (use host:port, tcp://host:port, or unix:///path)", raw)
}

// MustParse is like Parse but panics on error. For use in tests and
// compile-time constants only.
func MustParse(raw string) *Addr {
	a, err := Parse(raw)
	if err != nil {
		panic(err)
	}
	return a
}

// DialString returns the string for net.Dial or net.Listen.
// For TCP/UDP: "host:port". For Unix: the path.
func (a *Addr) DialString() string {
	if a.IsUnix() {
		return a.Path
	}
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// IsUnix returns true if this is a Unix domain socket address.
func (a *Addr) IsUnix() bool {
	return a.Network == NetUnix || a.Network == NetUnixgram
}

// IsTCP returns true if this is a TCP address.
func (a *Addr) IsTCP() bool {
	return a.Network == NetTCP
}

// IsUDP returns true if this is a UDP address.
func (a *Addr) IsUDP() bool {
	return a.Network == NetUDP
}

// CaddyTarget returns the address in a format suitable for Caddy's
// reverse_proxy directive.
//
//	"127.0.0.1:3000"          → "127.0.0.1:3000"
//	"unix:///run/app.sock"    → "unix//run/app.sock"
func (a *Addr) CaddyTarget() string {
	if a.IsUnix() {
		return "unix/" + a.Path[1:] // Caddy uses unix//path (single slash after unix/)
	}
	return a.DialString()
}

// String returns a canonical representation.
func (a *Addr) String() string {
	if a.IsUnix() {
		return a.Network + "://" + a.Path
	}
	if a.Network == NetUDP {
		return "udp://" + a.DialString()
	}
	// TCP shorthand: just host:port
	return a.DialString()
}

// WithPort returns a copy with the port set. Useful when the port is
// specified separately from the host (e.g., Exposure forms).
func (a *Addr) WithPort(port int) *Addr {
	return &Addr{
		Network: a.Network,
		Host:    a.Host,
		Port:    port,
		Path:    a.Path,
	}
}

// WithHost returns a copy with the host set.
func (a *Addr) WithHost(host string) *Addr {
	return &Addr{
		Network: a.Network,
		Host:    host,
		Port:    a.Port,
		Path:    a.Path,
	}
}

// ─── internal parsers ───

func parseHostPort(raw string, network string) (*Addr, error) {
	// Strip brackets from IPv6 addresses: [::1]:8080
	host, portStr, err := net.SplitHostPort(raw)
	if err != nil {
		// Maybe it's just a host without port
		if raw != "" && !strings.Contains(raw, ":") {
			return &Addr{Network: network, Host: raw}, nil
		}
		return nil, fmt.Errorf("parse %s host:port %q: %w", network, raw, err)
	}
	port, err := net.LookupPort(network, portStr)
	if err != nil {
		return nil, fmt.Errorf("parse port in %q: %w", raw, err)
	}
	return &Addr{Network: network, Host: host, Port: port}, nil
}

func parseUnix(raw string, network string) (*Addr, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty unix socket path")
	}
	if !filepath.IsAbs(raw) {
		return nil, fmt.Errorf("unix socket path must be absolute: %s", raw)
	}
	return &Addr{Network: network, Path: raw}, nil
}
