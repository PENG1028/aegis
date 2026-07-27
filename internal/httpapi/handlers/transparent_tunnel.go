package handlers

import (
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"aegis/internal/transparent"
)

// TransparentTunnel upgrades an authenticated HTTP request into a raw TCP
// tunnel and connects it to the local endpoint port on this node.
func (h *Handlers) TransparentTunnel(w http.ResponseWriter, r *http.Request) {
	secret := ""
	if h.Config != nil {
		secret = h.Config.DistNode.Secret
	}
	if secret == "" {
		http.Error(w, "transparent tunnel is not configured", http.StatusServiceUnavailable)
		return
	}
	if !validTunnelAuth(r.Header.Get("Authorization"), secret) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !headerHasToken(r.Header, "Connection", "upgrade") ||
		!headerHasToken(r.Header, "Upgrade", transparent.TunnelUpgrade) {
		http.Error(w, "upgrade required", http.StatusUpgradeRequired)
		return
	}

	port, err := strconv.Atoi(r.Header.Get("X-Aegis-Original-Port"))
	if err != nil || port <= 0 || port > 65535 {
		http.Error(w, "invalid original port", http.StatusBadRequest)
		return
	}
	serviceID := strings.TrimSpace(r.Header.Get("X-Aegis-Target-Service"))
	targetNodeID := strings.TrimSpace(r.Header.Get("X-Aegis-Target-Node"))
	if serviceID == "" || targetNodeID == "" {
		http.Error(w, "missing target identity", http.StatusBadRequest)
		return
	}
	if !h.transparentTunnelAllowed(serviceID, targetNodeID, port) {
		http.Error(w, "target not allowed", http.StatusForbidden)
		return
	}

	backend, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		http.Error(w, "target unavailable", http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		backend.Close()
		http.Error(w, "hijacking unsupported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		backend.Close()
		return
	}
	defer conn.Close()
	defer backend.Close()

	fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: %s\r\n\r\n", transparent.TunnelUpgrade)
	if err := rw.Flush(); err != nil {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(backend, conn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(conn, backend)
	}()
	wg.Wait()
}

func (h *Handlers) transparentTunnelAllowed(serviceID, targetNodeID string, port int) bool {
	if h == nil || h.EndpointRepo == nil || h.NodeRepo == nil {
		return false
	}
	if serviceID == "__panel" || port == h.controlPlanePort() {
		return false
	}
	current, err := h.NodeRepo.FindCurrent()
	if err != nil || current == nil || current.NodeID == "" || targetNodeID != current.NodeID {
		return false
	}
	eps, err := h.EndpointRepo.FindAllEnabled()
	if err != nil {
		return false
	}
	for i := range eps {
		ep := eps[i]
		if ep.ServiceID != serviceID || ep.NodeID != current.NodeID {
			continue
		}
		host, epPort := ep.HostPort()
		if epPort != port {
			continue
		}
		if host == "127.0.0.1" || host == "localhost" || host == "::1" {
			return true
		}
	}
	return false
}

func (h *Handlers) controlPlanePort() int {
	addr := "127.0.0.1:7380"
	if h != nil && h.Config != nil && h.Config.Server.Addr != "" {
		addr = h.Config.Server.Addr
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return 7380
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 7380
	}
	return port
}

func validTunnelAuth(auth, secret string) bool {
	auth = strings.TrimSpace(auth)
	const prefix = transparent.TunnelAuthType + " "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	got := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return subtle.ConstantTimeCompare([]byte(got), []byte(secret)) == 1
}

func headerHasToken(h http.Header, key, want string) bool {
	for _, value := range h.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}
