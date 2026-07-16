package transparent

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TransparentProxy accepts TCP connections that were redirected by iptables DNAT
// and forwards them to a configured backend. For cross-node traffic, the backend
// is the local Caddy (:80) which routes via gateway link.
//
// On Linux, callers may use getOriginalDst(conn) to discover the pre-DNAT
// destination and perform dynamic routing decisions.
type TransparentProxy struct {
	id         string
	listenAddr string
	targetHost string
	targetPort int
	tunnel     *TunnelConfig

	listener  net.Listener
	connCount int64
	bytesIn   int64
	bytesOut  int64

	mu      sync.Mutex
	stopCh  chan struct{}
	stopped bool

	// OnLinuxConn is an optional callback invoked for each accepted connection
	// on Linux. It receives the original (pre-DNAT) destination IP:port.
	// If nil, connections are forwarded directly to targetHost:targetPort.
	OnLinuxConn func(originalDst string)
}

// ProxyConfig configures a transparent proxy instance.
type ProxyConfig struct {
	ID         string
	ListenAddr string // e.g. "127.0.0.1:19100" — where this proxy listens
	TargetHost string // where to forward accepted connections
	TargetPort int
	Tunnel     *TunnelConfig
}

// NewProxy creates a transparent proxy. Call Start() to begin listening.
func NewProxy(cfg ProxyConfig) *TransparentProxy {
	return &TransparentProxy{
		id:         cfg.ID,
		listenAddr: cfg.ListenAddr,
		targetHost: cfg.TargetHost,
		targetPort: cfg.TargetPort,
		tunnel:     cfg.Tunnel,
		stopCh:     make(chan struct{}),
	}
}

// Start begins listening. Returns an error if the address is already in use.
func (p *TransparentProxy) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return fmt.Errorf("transparent proxy %s has been stopped", p.id)
	}

	ln, err := net.Listen("tcp", p.listenAddr)
	if err != nil {
		return fmt.Errorf("transparent proxy %s: listen on %s: %w", p.id, p.listenAddr, err)
	}

	p.listener = ln
	go p.acceptLoop()
	log.Printf("[transparent] proxy %s listening on %s → %s:%d",
		p.id, p.listenAddr, p.targetHost, p.targetPort)
	return nil
}

// Stop gracefully stops the proxy. After Stop, Start cannot be called again.
func (p *TransparentProxy) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}
	p.stopped = true
	close(p.stopCh)

	if p.listener != nil {
		p.listener.Close()
	}
	log.Printf("[transparent] proxy %s stopped (conns=%d, in=%d, out=%d)",
		p.id, p.connCount, p.bytesIn, p.bytesOut)
}

// ListenAddr returns the address this proxy is listening on.
// Useful when the proxy was started with port 0 (OS-assigned).
func (p *TransparentProxy) ListenAddr() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return p.listenAddr
}

// Stats returns connection/byte counters.
func (p *TransparentProxy) Stats() (conns, bytesIn, bytesOut int64) {
	return atomic.LoadInt64(&p.connCount),
		atomic.LoadInt64(&p.bytesIn),
		atomic.LoadInt64(&p.bytesOut)
}

// ─── internal ──────────────────────────────────────────────────────────

func (p *TransparentProxy) acceptLoop() {
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
				continue
			}
		}

		atomic.AddInt64(&p.connCount, 1)
		go p.handleConn(conn)
	}
}

func (p *TransparentProxy) handleConn(clientConn net.Conn) {
	defer clientConn.Close()

	// If an OnLinuxConn callback is registered, allow the caller to inspect
	// the original destination (via SO_ORIGINAL_DST) for dynamic routing.
	if p.OnLinuxConn != nil {
		originalDst := getOriginalDst(clientConn)
		if originalDst != "" {
			p.OnLinuxConn(originalDst)
		}
	}

	backendConn, err := p.dialBackend()
	if err != nil {
		log.Printf("[transparent] %s: dial backend: %v", p.id, err)
		return
	}
	defer backendConn.Close()

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		n, _ := io.Copy(backendConn, clientConn)
		atomic.AddInt64(&p.bytesIn, n)
	}()

	go func() {
		defer wg.Done()
		n, _ := io.Copy(clientConn, backendConn)
		atomic.AddInt64(&p.bytesOut, n)
	}()

	wg.Wait()
}

func (p *TransparentProxy) dialBackend() (net.Conn, error) {
	if p.tunnel != nil {
		return dialTunnel(*p.tunnel)
	}
	return net.DialTimeout("tcp",
		fmt.Sprintf("%s:%d", p.targetHost, p.targetPort), 5*time.Second)
}
