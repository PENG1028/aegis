// Package udp provides UDP port forwarding (similar to internal/tcp for TCP).
//
// Unlike TCP, UDP is connectionless — each packet is independent.
// The proxy maintains a session table mapping client addresses to target
// connections so return packets can be routed back to the correct client.
package udp

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"aegis/internal/addr"
)

const (
	// DefaultSessionTimeout is how long an idle UDP session lives.
	DefaultSessionTimeout = 60 * time.Second
	// MaxSessions is the maximum number of concurrent UDP sessions per proxy.
	MaxSessions = 1024
)

// Proxy forwards UDP packets from a local port to a target address.
// The target can be a UDP host:port or a Unix datagram socket (unixgram:///path).
type Proxy struct {
	ID        string
	EntryHost string
	EntryPort int
	Target    *addr.Addr // parsed target: UDP host:port or Unix datagram socket

	conn       *net.UDPConn
	targetUDP  *net.UDPAddr      // pre-resolved UDP target (nil if unix socket)
	sessions   map[string]*session
	sessionsMu sync.RWMutex
	running    atomic.Bool
	done       chan struct{}
	packetsIn  atomic.Int64
	packetsOut atomic.Int64
	bytesIn    atomic.Int64
	bytesOut   atomic.Int64
}

type session struct {
	clientAddr *net.UDPAddr
	lastSeen   time.Time
}

// NewProxy creates a new UDP proxy.
// targetHost can be "host:port", "udp://host:port", or "unixgram:///path".
func NewProxy(id, entryHost string, entryPort int, targetHost string, targetPort int) *Proxy {
	return &Proxy{
		ID:        id,
		EntryHost: entryHost,
		EntryPort: entryPort,
		Target:    resolveUDPTarget(targetHost, targetPort),
		sessions:  make(map[string]*session),
		done:      make(chan struct{}),
	}
}

func resolveUDPTarget(host string, port int) *addr.Addr {
	if a, err := addr.Parse(host); err == nil && a.Port > 0 {
		if a.IsUnix() {
			return a // unixgram:// already parsed
		}
		return &addr.Addr{Network: addr.NetUDP, Host: a.Host, Port: a.Port}
	}
	if host != "" && host[0] == '/' {
		if a, err := addr.Parse("unixgram://" + host); err == nil {
			return a
		}
	}
	return &addr.Addr{Network: addr.NetUDP, Host: host, Port: port}
}

// Start begins listening and forwarding UDP packets.
func (p *Proxy) Start() error {
	// Resolve target
	if !p.Target.IsUnix() {
		targetAddr, err := net.ResolveUDPAddr("udp", p.Target.DialString())
		if err != nil {
			return fmt.Errorf("resolve target %s: %w", p.Target.DialString(), err)
		}
		p.targetUDP = targetAddr
	}

	// Listen
	entryAddr := fmt.Sprintf("%s:%d", p.EntryHost, p.EntryPort)
	conn, err := net.ListenPacket("udp", entryAddr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", entryAddr, err)
	}
	p.conn = conn.(*net.UDPConn)
	p.running.Store(true)

	go p.readLoop()
	go p.cleanupLoop()

	return nil
}

// Stop stops the proxy and cleans up.
func (p *Proxy) Stop() error {
	if !p.running.Swap(false) {
		return nil
	}
	close(p.done)
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

// IsRunning returns whether the proxy is active.
func (p *Proxy) IsRunning() bool {
	return p.running.Load()
}

// Stats returns traffic statistics.
func (p *Proxy) Stats() (packetsIn, packetsOut int64, bytesIn, bytesOut int64) {
	return p.packetsIn.Load(), p.packetsOut.Load(), p.bytesIn.Load(), p.bytesOut.Load()
}

// SessionCount returns the number of active sessions.
func (p *Proxy) SessionCount() int {
	p.sessionsMu.RLock()
	defer p.sessionsMu.RUnlock()
	return len(p.sessions)
}

// readLoop reads packets from clients and forwards to target.
func (p *Proxy) readLoop() {
	buf := make([]byte, 65535) // max UDP packet size
	for p.running.Load() {
		_ = p.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, clientAddr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			if p.running.Load() {
				continue // timeout or transient error
			}
			return
		}

		p.packetsIn.Add(1)
		p.bytesIn.Add(int64(n))

		// Track session
		clientKey := clientAddr.String()
		p.sessionsMu.Lock()
		p.sessions[clientKey] = &session{clientAddr: clientAddr, lastSeen: time.Now()}
		if len(p.sessions) > MaxSessions {
			// Evict oldest session
			var oldestKey string
			var oldestTime time.Time
			for k, s := range p.sessions {
				if oldestKey == "" || s.lastSeen.Before(oldestTime) {
					oldestKey = k
					oldestTime = s.lastSeen
				}
			}
			delete(p.sessions, oldestKey)
		}
		p.sessionsMu.Unlock()

		// Forward to target — TCP or Unix socket
		var targetConn *net.UDPConn
		if p.Target.IsUnix() {
			// Unix datagram: dial and cast
			uc, dialErr := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: p.Target.Path, Net: "unixgram"})
			if dialErr != nil {
				continue
			}
			targetConn = &net.UDPConn{} // placeholder — use raw conn
			_ = uc
			// For unixgram, use the raw UnixConn
			_, _ = uc.Write(buf[:n])
			p.packetsOut.Add(1)
			p.bytesOut.Add(int64(n))
			uc.Close()
			continue
		}
		targetConn, err = net.DialUDP("udp", nil, p.targetUDP)
		if err != nil {
			continue
		}

		_, _ = targetConn.Write(buf[:n])
		p.packetsOut.Add(1)
		p.bytesOut.Add(int64(n))

		// Read response from target and send back to client (async)
		go p.handleResponse(targetConn, clientAddr, clientKey)
	}
}

// handleResponse reads the response from the target and forwards to the client.
func (p *Proxy) handleResponse(targetConn *net.UDPConn, clientAddr *net.UDPAddr, clientKey string) {
	defer targetConn.Close()

	respBuf := make([]byte, 65535)
	for {
		_ = targetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err := targetConn.ReadFromUDP(respBuf)
		if err != nil {
			return // timeout or connection closed
		}

		p.packetsOut.Add(1)
		p.bytesOut.Add(int64(n))

		_, _ = p.conn.WriteToUDP(respBuf[:n], clientAddr)

		// Update session timestamp
		p.sessionsMu.Lock()
		if s, ok := p.sessions[clientKey]; ok {
			s.lastSeen = time.Now()
		}
		p.sessionsMu.Unlock()
	}
}

// cleanupLoop removes idle sessions periodically.
func (p *Proxy) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-p.done:
			return
		case <-ticker.C:
			p.cleanup()
		}
	}
}

func (p *Proxy) cleanup() {
	cutoff := time.Now().Add(-DefaultSessionTimeout)
	p.sessionsMu.Lock()
	for k, s := range p.sessions {
		if s.lastSeen.Before(cutoff) {
			delete(p.sessions, k)
		}
	}
	p.sessionsMu.Unlock()
}
