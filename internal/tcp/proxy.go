package tcp

import (
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Proxy represents a single TCP forwarding proxy.
type Proxy struct {
	ID         string
	EntryHost  string
	EntryPort  int
	TargetHost string
	TargetPort int
	Status     string // running | stopped | error
	Message    string

	listener   net.Listener
	connCount  int64
	mu         sync.Mutex
	stopCh     chan struct{}
}

// NewProxy creates a new TCP proxy (does not start listening).
func NewProxy(id, entryHost string, entryPort int, targetHost string, targetPort int) *Proxy {
	return &Proxy{
		ID:         id,
		EntryHost:  entryHost,
		EntryPort:  entryPort,
		TargetHost: targetHost,
		TargetPort: targetPort,
		Status:     "stopped",
		stopCh:     make(chan struct{}),
	}
}

// Start begins listening and forwarding connections.
func (p *Proxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Status == "running" {
		return fmt.Errorf("proxy %s is already running", p.ID)
	}

	addr := fmt.Sprintf("%s:%d", p.EntryHost, p.EntryPort)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		p.Status = "error"
		p.Message = fmt.Sprintf("listen failed: %v", err)
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	p.listener = listener
	p.Status = "running"
	p.Message = fmt.Sprintf("listening on %s, forwarding to %s:%d", addr, p.TargetHost, p.TargetPort)
	p.stopCh = make(chan struct{})

	go p.acceptLoop()
	return nil
}

// Stop stops the proxy and closes all connections.
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
	p.Message = "stopped"
	return nil
}

// ConnectionCount returns the number of active connections processed.
func (p *Proxy) ConnectionCount() int64 {
	return atomic.LoadInt64(&p.connCount)
}

// IsRunning returns true if the proxy is actively listening.
func (p *Proxy) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Status == "running"
}

// CheckTarget verifies the target is TCP reachable.
func (p *Proxy) CheckTarget() (bool, string) {
	target := fmt.Sprintf("%s:%d", p.TargetHost, p.TargetPort)
	conn, err := net.DialTimeout("tcp", target, 3*time.Second)
	if err != nil {
		return false, fmt.Sprintf("target %s unreachable: %v", target, err)
	}
	conn.Close()
	return true, fmt.Sprintf("target %s reachable", target)
}

// ListenAddr returns the address this proxy is listening on.
func (p *Proxy) ListenAddr() string {
	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return fmt.Sprintf("%s:%d", p.EntryHost, p.EntryPort)
}

func (p *Proxy) acceptLoop() {
	for {
		select {
		case <-p.stopCh:
			return
		default:
		}

		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.stopCh:
				return
			default:
				p.mu.Lock()
				p.Message = fmt.Sprintf("accept error: %v", err)
				p.mu.Unlock()
			}
			continue
		}

		atomic.AddInt64(&p.connCount, 1)
		go p.handleConn(conn)
	}
}

func (p *Proxy) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	target := fmt.Sprintf("%s:%d", p.TargetHost, p.TargetPort)
	backendConn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		p.mu.Lock()
		p.Message = fmt.Sprintf("backend dial failed: %v", err)
		p.mu.Unlock()
		return
	}
	defer backendConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(backendConn, clientConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(clientConn, backendConn)
	}()

	wg.Wait()
}
