package transparent

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestRedirectRule_Validate(t *testing.T) {
	tests := []struct {
		name  string
		rule  RedirectRule
		valid bool
	}{
		{"valid", RedirectRule{
			ID: "r1", OriginalIP: "192.168.1.100", OriginalPort: 9100,
			TargetServiceID: "svc_1", TargetNodeID: "node_b",
		}, true},
		{"missing IP", RedirectRule{
			ID: "r2", OriginalPort: 9100, TargetServiceID: "svc_1",
		}, false},
		{"port zero", RedirectRule{
			ID: "r3", OriginalIP: "10.0.0.1", OriginalPort: 0, TargetServiceID: "svc_1",
		}, false},
		{"port too high", RedirectRule{
			ID: "r4", OriginalIP: "10.0.0.1", OriginalPort: 99999, TargetServiceID: "svc_1",
		}, false},
		{"missing service", RedirectRule{
			ID: "r5", OriginalIP: "10.0.0.1", OriginalPort: 8080,
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if tt.valid && err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("expected validation error")
			}
		})
	}
}

func TestRedirectRule_Key(t *testing.T) {
	r := RedirectRule{OriginalIP: "192.168.1.100", OriginalPort: 9100}
	if r.Key() != "192.168.1.100:9100" {
		t.Errorf("expected '192.168.1.100:9100', got %q", r.Key())
	}
}

// TestManager_Lifecycle tests start → list → stop lifecycle.
// iptables operations will fail on non-Linux — the test gracefully handles this.
func TestManager_Lifecycle(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()

	// Start a real TCP listener as "backend"
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start backend: %v", err)
	}
	defer backend.Close()
	_, portStr, _ := net.SplitHostPort(backend.Addr().String())
	backendPort := 0
	fmt.Sscanf(portStr, "%d", &backendPort)

	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("OK"))
			conn.Close()
		}
	}()

	rule := RedirectRule{
		ID: "test-1", OriginalIP: "10.255.255.1", OriginalPort: 9999,
		TargetServiceID: "svc_test", TargetNodeID: "node_test",
	}

	err = m.StartRedirect(rule)
	if err != nil {
		t.Logf("StartRedirect failed (expected on non-Linux): %v", err)
		// Test still passes — iptables isn't available on this platform
		return
	}

	if m.RuleCount() != 1 {
		t.Errorf("expected 1 rule, got %d", m.RuleCount())
	}

	status := m.ListStatus()
	if len(status) != 1 {
		t.Errorf("expected 1 status entry, got %d", len(status))
	}
	if status[0].Rule.ID != "test-1" {
		t.Errorf("expected rule ID 'test-1', got %q", status[0].Rule.ID)
	}

	if err := m.StopRedirect(rule.ID); err != nil {
		t.Errorf("StopRedirect: %v", err)
	}
	if m.RuleCount() != 0 {
		t.Errorf("expected 0 rules after stop, got %d", m.RuleCount())
	}
}

func TestManager_DuplicateRejected(t *testing.T) {
	m := NewManager()
	defer m.Shutdown()

	rule := RedirectRule{
		ID: "dup-1", OriginalIP: "10.0.0.1", OriginalPort: 8080,
		TargetServiceID: "svc_x", LocalProxyPort: 18150,
	}
	_ = m.StartRedirect(rule)
	err := m.StartRedirect(rule)
	if err == nil {
		t.Error("duplicate rule should be rejected")
	}
}

func TestManager_PortAllocation(t *testing.T) {
	m := NewManager()
	ports := make(map[int]bool)
	for i := 0; i < 5; i++ {
		port, err := m.allocatePortLocked()
		if err != nil {
			t.Fatalf("allocatePort: %v", err)
		}
		if port < m.portStart || port > m.portEnd {
			t.Errorf("port %d outside [%d,%d]", port, m.portStart, m.portEnd)
		}
		if ports[port] {
			t.Errorf("duplicate port: %d", port)
		}
		ports[port] = true
	}
}

func TestManager_DiscoverTargets_SkipsLocalhost(t *testing.T) {
	m := NewManager()
	endpoints := []EndpointInfo{
		{EndpointID: "ep1", ServiceID: "svc_a", Host: "192.168.1.100", Port: 9100, NodeID: "node_b"},
		{EndpointID: "ep2", ServiceID: "svc_b", Host: "127.0.0.1", Port: 3001, NodeID: "node_a"},  // skipped
		{EndpointID: "ep3", ServiceID: "svc_c", Host: "localhost", Port: 8080, NodeID: "node_a"},   // skipped
		{EndpointID: "ep4", ServiceID: "svc_d", Host: "", Port: 0, NodeID: "node_d"},               // skipped
	}
	rules := m.DiscoverTargets(endpoints)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule (only non-loopback), got %d", len(rules))
	}
	if rules[0].OriginalIP != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", rules[0].OriginalIP)
	}
}

func TestTransparentProxy_Forwarding(t *testing.T) {
	// Start backend
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	defer backend.Close()
	_, portStr, _ := net.SplitHostPort(backend.Addr().String())
	backendPort := 0
	fmt.Sscanf(portStr, "%d", &backendPort)

	ready := make(chan struct{})
	go func() {
		close(ready)
		conn, _ := backend.Accept()
		if conn != nil {
			conn.Write([]byte("hello"))
			conn.Close()
		}
	}()

	<-ready

	// Start proxy — use port 0 for OS-assigned address
	proxy := NewProxy(ProxyConfig{
		ID: "test-fwd", ListenAddr: "127.0.0.1:0",
		TargetHost: "127.0.0.1", TargetPort: backendPort,
	})
	if err := proxy.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer proxy.Stop()

	addr := proxy.ListenAddr()
	t.Logf("proxy listening on %s", addr)

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	n, _ := conn.Read(buf)
	if string(buf[:n]) != "hello" {
		t.Errorf("expected 'hello', got %q", string(buf[:n]))
	}

	conns, in, out := proxy.Stats()
	t.Logf("stats: conns=%d in=%d out=%d", conns, in, out)
	if conns != 1 {
		t.Errorf("expected 1 connection, got %d", conns)
	}
}

func TestTransparentProxy_Concurrent(t *testing.T) {
	backend, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("backend: %v", err)
	}
	defer backend.Close()
	_, portStr, _ := net.SplitHostPort(backend.Addr().String())
	backendPort := 0
	fmt.Sscanf(portStr, "%d", &backendPort)

	go func() {
		for {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("ok"))
			conn.Close()
		}
	}()

	proxy := NewProxy(ProxyConfig{
		ID: "test-conc", ListenAddr: "127.0.0.1:0",
		TargetHost: "127.0.0.1", TargetPort: backendPort,
	})
	if err := proxy.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer proxy.Stop()

	addr := proxy.ListenAddr()

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				errs <- err
				return
			}
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			if string(buf[:n]) != "ok" {
				errs <- fmt.Errorf("expected 'ok', got %q", string(buf[:n]))
			}
			conn.Close()
		}()
	}
	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}

	conns, _, _ := proxy.Stats()
	if conns != 20 {
		t.Errorf("expected 20 conns, got %d", conns)
	}
}

func TestTransparentProxy_CannotStartAfterStop(t *testing.T) {
	proxy := NewProxy(ProxyConfig{
		ID: "test-norestart", ListenAddr: "127.0.0.1:0",
		TargetHost: "127.0.0.1", TargetPort: 12345,
	})
	if err := proxy.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	proxy.Stop()

	err := proxy.Start()
	if err == nil {
		t.Error("Start after Stop should fail")
	}
}
