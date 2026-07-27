// Package udp provides UDP port forwarding (similar to internal/tcp for TCP).
//
// Unlike TCP, UDP is connectionless — each packet is independent.
// The proxy maintains a session table mapping client addresses to target
// connections so return packets can be routed back to the correct client.
package udp

import (
	"context"
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
	sessionsMu sync.Mutex
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
	targetConn *net.UDPConn // shared connected UDP socket for this session
	ctx        context.Context
	cancel     context.CancelFunc
	handlerActive bool // true if a handler goroutine is running for this session
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

	// Cancel all session handlers
	p.sessionsMu.Lock()
	for _, s := range p.sessions {
		if s.cancel != nil {
			s.cancel()
		}
		if s.targetConn != nil {
			s.targetConn.Close()
		}
	}
	p.sessions = make(map[string]*session)
	p.sessionsMu.Unlock()

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
	p.sessionsMu.Lock()
	defer p.sessionsMu.Unlock()
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

		clientKey := clientAddr.String()

		p.sessionsMu.Lock()

		// Check if session already exists
		s, exists := p.sessions[clientKey]
		if exists {
			s.lastSeen = time.Now()
		} else {
			// Evict oldest if at capacity
			if len(p.sessions) >= MaxSessions {
				var oldestKey string
				var oldestTime time.Time
				for k, s := range p.sessions {
					if oldestKey == "" || s.lastSeen.Before(oldestTime) {
						oldestKey = k
						oldestTime = s.lastSeen
					}
				}
				if oldestKey != "" {
					old := p.sessions[oldestKey]
					if old.cancel != nil {
						old.cancel()
					}
					if old.targetConn != nil {
						old.targetConn.Close()
					}
					delete(p.sessions, oldestKey)
				}
			}

			ctx, cancel := context.WithCancel(context.Background())
			s = &session{
				clientAddr: clientAddr,
				lastSeen:   time.Now(),
				ctx:        ctx,
				cancel:     cancel,
			}
			p.sessions[clientKey] = s
		}

		// If no handler is active, spawn one with a shared connection
		if !s.handlerActive {
			s.handlerActive = true

			if p.Target.IsUnix() {
				uc, dialErr := net.DialUnix("unixgram", nil, &net.UnixAddr{Name: p.Target.Path, Net: "unixgram"})
				if dialErr != nil {
					s.handlerActive = false
					p.sessionsMu.Unlock()
					continue
				}
				_, _ = uc.Write(buf[:n])
				p.packetsOut.Add(1)
				p.bytesOut.Add(int64(n))
				go p.handleResponseUnix(uc, clientAddr, clientKey, s.ctx, func() {
					p.sessionsMu.Lock()
					s.handlerActive = false
					p.sessionsMu.Unlock()
				})
				p.sessionsMu.Unlock()
				continue
			}

			targetConn, dialErr := net.DialUDP("udp", nil, p.targetUDP)
			if dialErr != nil {
				s.handlerActive = false
				p.sessionsMu.Unlock()
				continue
			}
			s.targetConn = targetConn

			_, _ = targetConn.Write(buf[:n])
			p.packetsOut.Add(1)
			p.bytesOut.Add(int64(n))

			go p.handleResponse(targetConn, clientAddr, clientKey, s.ctx, func() {
				p.sessionsMu.Lock()
				s.handlerActive = false
				if s.targetConn != nil {
					s.targetConn.Close()
					s.targetConn = nil
				}
				p.sessionsMu.Unlock()
			})
			p.sessionsMu.Unlock()
			continue
		}

		// Handler already running — write through existing connection
		if s.targetConn != nil {
			_, _ = s.targetConn.Write(buf[:n])
			p.packetsOut.Add(1)
			p.bytesOut.Add(int64(n))
		}
		p.sessionsMu.Unlock()
	}
}

// handleResponse reads the response from the target and forwards to the client.
// The onExit callback is called when the goroutine exits.
func (p *Proxy) handleResponse(targetConn *net.UDPConn, clientAddr *net.UDPAddr, clientKey string, ctx context.Context, onExit func()) {
	defer onExit()

	respBuf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = targetConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err := targetConn.ReadFromUDP(respBuf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			// Check if session still exists; if not, stop
			p.sessionsMu.Lock()
			_, exists := p.sessions[clientKey]
			p.sessionsMu.Unlock()
			if !exists {
				return
			}
			continue
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

// handleResponseUnix reads the response from a Unix datagram socket target
// and forwards it back to the client.
func (p *Proxy) handleResponseUnix(uc *net.UnixConn, clientAddr *net.UDPAddr, clientKey string, ctx context.Context, onExit func()) {
	defer uc.Close()
	defer onExit()

	respBuf := make([]byte, 65535)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = uc.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, _, err := uc.ReadFromUnix(respBuf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
			p.sessionsMu.Lock()
			_, exists := p.sessions[clientKey]
			p.sessionsMu.Unlock()
			if !exists {
				return
			}
			continue
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
			if s.cancel != nil {
				s.cancel()
			}
			if s.targetConn != nil {
				s.targetConn.Close()
			}
			delete(p.sessions, k)
		}
	}
	p.sessionsMu.Unlock()
}
