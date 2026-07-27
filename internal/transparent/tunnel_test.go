package transparent

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
)

func TestDialTunnelUpgradeStreamsBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()

		req, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			done <- err
			return
		}
		if req.Header.Get("Upgrade") != TunnelUpgrade {
			done <- fmt.Errorf("upgrade = %q", req.Header.Get("Upgrade"))
			return
		}
		fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n", TunnelUpgrade)
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err != nil {
			done <- err
			return
		}
		if string(buf) != "ping" {
			done <- fmt.Errorf("payload = %q", string(buf))
			return
		}
		_, err = conn.Write([]byte("pong"))
		done <- err
	}()

	conn, err := dialTunnel(TunnelConfig{
		EdgeAddr: ln.Addr().String(),
		Secret:   "secret",
		Rule: RedirectRule{
			ID:              "r1",
			OriginalIP:      "203.0.113.10",
			OriginalPort:    9100,
			TargetServiceID: "svc",
			TargetNodeID:    "node-b",
		},
	})
	if err != nil {
		t.Fatalf("dialTunnel: %v", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(buf) != "pong" {
		t.Fatalf("response = %q", string(buf))
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
