package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"

	"aegis/internal/serviceauth"
)

// ============================================================================
// SecretStore — file-based, 0600 permissions
// ============================================================================

type fileSecretStore struct {
	path string
}

func newFileSecretStore(path string) *fileSecretStore {
	return &fileSecretStore{path: path}
}

func (s *fileSecretStore) Load() ([]byte, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (s *fileSecretStore) Save(secret []byte) error {
	return os.WriteFile(s.path, secret, 0600)
}

// ============================================================================
// NodeChecker — CIDR whitelist + localhost
// ============================================================================

type cidrChecker struct {
	networks []*net.IPNet
}

func newCIDRChecker(networks []*net.IPNet) *cidrChecker {
	return &cidrChecker{networks: networks}
}

func (c *cidrChecker) FindByIP(ipStr string) (*serviceauth.NodeInfo, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, serviceauth.ErrNotInCluster
	}

	// Localhost.
	if ip.IsLoopback() {
		return &serviceauth.NodeInfo{NodeID: "localhost"}, nil
	}

	// CIDR whitelist.
	for _, n := range c.networks {
		if n.Contains(ip) {
			return &serviceauth.NodeInfo{NodeID: ipStr}, nil
		}
	}

	return nil, serviceauth.ErrNotInCluster
}

func parseCIDRs(raw string) []*net.IPNet {
	if raw == "" {
		return nil
	}
	var out []*net.IPNet
	for _, s := range strings.Split(raw, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid CIDR %q: %v\n", s, err)
			continue
		}
		out = append(out, n)
	}
	return out
}

// ============================================================================
// LogWriter — direct SQLite insert
// ============================================================================

type dbLogWriter struct {
	db *sql.DB
}

func newDBLogWriter(db *sql.DB) *dbLogWriter {
	return &dbLogWriter{db: db}
}

func (w *dbLogWriter) WriteCallLog(caller, target, api, callerHost, targetHost string, allowed bool, latencyMs int, errMsg string) error {
	allowedInt := 0
	if allowed {
		allowedInt = 1
	}
	_, err := w.db.Exec(
		`INSERT INTO svc_auth_call_logs (id, caller_service, target_service, target_api, caller_host, target_host, allowed, latency_ms, error_msg, called_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, datetime('now'))`,
		serviceauth.DefaultIDGen(), caller, target, api, callerHost, targetHost, allowedInt, latencyMs, errMsg,
	)
	return err
}

// ============================================================================
// Master key — read or generate 32-byte key file
// ============================================================================

func loadOrGenerateKey(path string) []byte {
	data, err := os.ReadFile(path)
	if err == nil && len(data) >= 32 {
		return data[:32]
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		fmt.Fprintf(os.Stderr, "warning: crypto/rand failed, using insecure fallback: %v\n", err)
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not persist master key: %v\n", err)
	}
	return key
}
