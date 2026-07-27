package tcp

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"aegis/internal/addr"
	recovery "aegis/internal/core"
)

// Proxy represents a single TCP forwarding proxy.
// The entry is always a TCP host:port. The target can be a TCP host:port
// or a Unix domain socket (unix:///path).
type Proxy struct {
	ID        string
	EntryHost string
	EntryPort int
	Target    *addr.Addr // parsed target: TCP host:port or Unix socket path
	Status    string     // running | stopped | error
	Message   string

	listener  net.Listener
	connCount int64
	mu        sync.Mutex
	stopCh    chan struct{}
}

// NewProxy creates a new TCP proxy (does not start listening).
// targetHost can be "host", "host:port", "tcp://host:port", or "unix:///path".
// targetPort is used when targetHost is a bare hostname.
func NewProxy(id, entryHost string, entryPort int, targetHost string, targetPort int) *Proxy {
	target := resolveTarget(targetHost, targetPort)
	return &Proxy{
		ID:        id,
		EntryHost: entryHost,
		EntryPort: entryPort,
		Target:    target,
		Status:    "stopped",
		stopCh:    make(chan struct{}),
	}
}

// resolveTarget constructs an Addr from host and port strings.
// If host contains a scheme (tcp://, unix://), port is ignored.
func resolveTarget(host string, port int) *addr.Addr {
	// Try parsing as a full address first (handles tcp://, unix://, etc.)
	if a, err := addr.Parse(host); err == nil && a.Port > 0 {
		return a
	}
	// If host is a Unix socket path (starts with /), parse as unix
	if host != "" && host[0] == '/' {
		if a, err := addr.Parse("unix://" + host); err == nil {
			return a
		}
	}
	// Bare host: default
	return &addr.Addr{Network: addr.NetTCP, Host: host, Port: port}
}

// Start begins listening on entryHost:entryPort and forwarding to the target.
func (p *Proxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	entryAddr := fmt.Sprintf("%s:%d", p.EntryHost, p.EntryPort)
	ln, err := net.Listen("tcp", entryAddr)
	if err != nil {
		p.Status = "error"
		p.Message = fmt.Sprintf("listen %s: %v", entryAddr, err)
		return err
	}

	p.listener = ln
	p.Status = "running"
	p.Message = fmt.Sprintf("TCP proxy active: %s:%d -> %s", p.EntryHost, p.EntryPort, p.Target.String())

	recovery.Go("tcp-accept-"+p.ID, p.acceptLoop)
	return nil
}

func (p *Proxy) acceptLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		// Use a short accept timeout so we can check stopCh
		if tcpLn, ok := p.listener.(*net.TCPListener); ok {
			tcpLn.SetDeadline(time.Now().Add(1 * time.Second))
		}

		clientConn, err := p.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			// Listener closed (Stop called)
			return
		}

		recovery.Go("tcp-conn-"+p.ID, func() { p.handleConn(clientConn) })
	}
}

func (p *Proxy) handleConn(clientConn net.Conn) {
	defer clientConn.Close()
	atomic.AddInt64(&p.connCount, 1)
	defer atomic.AddInt64(&p.connCount, -1)

	// Dial the target — TCP or Unix socket
	var targetConn net.Conn
	var err error

	if p.Target.IsUnix() {
		targetConn, err = net.DialTimeout(p.Target.Network, p.Target.Path, 5*time.Second)
	} else {
		targetConn, err = net.DialTimeout("tcp", p.Target.DialString(), 5*time.Second)
	}
	if err != nil {
		p.mu.Lock()
		p.Status = "error"
		p.Message = fmt.Sprintf("connect to %s: %v", p.Target.String(), err)
		p.mu.Unlock()
		return
	}
	defer targetConn.Close()

	// Bidirectional copy
	done := make(chan struct{}, 2)
	go func() { io.Copy(targetConn, clientConn); done <- struct{}{} }()
	go func() { io.Copy(clientConn, targetConn); done <- struct{}{} }()
	<-done
}

// Stop stops the proxy and closes the listener.
func (p *Proxy) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status != "running" {
		return nil
	}

	close(p.stopCh)
	if p.listener != nil {
		p.listener.Close()
	}
	p.Status = "stopped"
	p.Message = "proxy stopped"
	return nil
}

// IsRunning returns whether the proxy is active.
func (p *Proxy) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status == "running"
}

// ConnectionCount returns the number of active connections.
func (p *Proxy) ConnectionCount() int64 {
	return atomic.LoadInt64(&p.connCount)
}

// CheckTarget tests whether the target is reachable.
func (p *Proxy) CheckTarget(timeout time.Duration) error {
	if p.Target.IsUnix() {
		conn, err := net.DialTimeout(p.Target.Network, p.Target.Path, timeout)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}
	conn, err := net.DialTimeout("tcp", p.Target.DialString(), timeout)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}
