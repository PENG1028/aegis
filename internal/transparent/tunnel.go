package transparent

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	TunnelPath     = "/api/transparent/v1/tunnel"
	TunnelUpgrade  = "aegis-transparent"
	TunnelAuthType = "Bearer"
)

// TunnelConfig describes a remote Aegis transparent tunnel endpoint.
type TunnelConfig struct {
	EdgeAddr string
	Secret   string
	Rule     RedirectRule
}

func dialTunnel(cfg TunnelConfig) (net.Conn, error) {
	if cfg.EdgeAddr == "" {
		return nil, fmt.Errorf("tunnel edge address is required")
	}
	if cfg.Secret == "" {
		return nil, fmt.Errorf("tunnel secret is required")
	}

	conn, err := net.DialTimeout("tcp", cfg.EdgeAddr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial tunnel edge %s: %w", cfg.EdgeAddr, err)
	}

	req, err := http.NewRequest("POST", "http://"+cfg.EdgeAddr+TunnelPath, nil)
	if err != nil {
		conn.Close()
		return nil, err
	}
	req.Host = cfg.EdgeAddr
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", TunnelUpgrade)
	req.Header.Set("Authorization", TunnelAuthType+" "+cfg.Secret)
	req.Header.Set("X-Aegis-Original-IP", cfg.Rule.OriginalIP)
	req.Header.Set("X-Aegis-Original-Port", strconv.Itoa(cfg.Rule.OriginalPort))
	req.Header.Set("X-Aegis-Target-Service", cfg.Rule.TargetServiceID)
	req.Header.Set("X-Aegis-Target-Node", cfg.Rule.TargetNodeID)

	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write tunnel request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read tunnel response: %w", err)
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, fmt.Errorf("tunnel rejected: HTTP %d", resp.StatusCode)
	}
	if !headerContainsToken(resp.Header, "Upgrade", TunnelUpgrade) {
		conn.Close()
		return nil, fmt.Errorf("tunnel rejected: missing upgrade acknowledgement")
	}
	return conn, nil
}

func headerContainsToken(h http.Header, key, want string) bool {
	for _, value := range h.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), want) {
				return true
			}
		}
	}
	return false
}
