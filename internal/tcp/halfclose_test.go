package tcp

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TestProxyHalfClosePropagation is a regression test: the old copy loop
// returned after the FIRST direction finished and then closed both
// connections, truncating the other direction's in-flight data. A client
// that sends its request and CloseWrite()'s (HTTP/1.0 POST, SMTP, protocols
// with request framing) must still receive the full response.
func TestProxyHalfClosePropagation(t *testing.T) {
	// Backend: read the request until EOF (client half-close), then send a
	// response derived from the request body.
	backendLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer backendLn.Close()
	backendAddr := backendLn.Addr().String()

	backendDone := make(chan error, 1)
	go func() {
		conn, err := backendLn.Accept()
		if err != nil {
			backendDone <- err
			return
		}
		defer conn.Close()
		body, err := io.ReadAll(conn) // returns when the client half-closes
		if err != nil {
			backendDone <- err
			return
		}
		_, err = conn.Write([]byte("response:" + string(body)))
		backendDone <- err
	}()

	// Proxy: entry -> backend
	proxy := NewProxy("halfclose-test", "127.0.0.1", 0, backendAddr, 0)
	if err := proxy.Start(); err != nil {
		t.Fatalf("proxy start: %v", err)
	}
	defer proxy.Stop()

	entryAddr := proxy.listener.Addr().String()

	client, err := net.DialTimeout("tcp", entryAddr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if _, err := client.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	// Half-close the write side: the request is complete.
	if tcp, ok := client.(*net.TCPConn); ok {
		if err := tcp.CloseWrite(); err != nil {
			t.Fatal(err)
		}
	}

	// The full response must arrive even though our write side is closed.
	client.SetReadDeadline(time.Now().Add(5 * time.Second))
	resp, err := io.ReadAll(client)
	if err != nil {
		t.Fatalf("read response after half-close: %v", err)
	}
	if got := string(resp); !strings.HasPrefix(got, "response:hello") {
		t.Fatalf("response = %q, want prefix response:hello (truncated by premature close?)", got)
	}

	select {
	case err := <-backendDone:
		if err != nil {
			t.Fatalf("backend: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("backend did not finish")
	}
}
