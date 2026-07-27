// Package aegis contains adapter implementations that connect the
// serviceauth core to Aegis infrastructure (secrets, node, logs).
// (embedded in Aegis)
// provides alternative implementations backed by the filesystem and CIDR lists.
package aegis

import (
	"context"
	"fmt"
	"os"

	"aegis/internal/logs"
	"aegis/internal/node"
	"aegis/internal/secrets"
	"aegis/internal/serviceauth"
)

// ============================================================================
// SecretStore — AES-256-GCM encrypted persistence
// ============================================================================

// SecretStoreAdapter persists the cluster secret using Aegis's AES-256-GCM
// encryption to files on disk.
type SecretStoreAdapter struct {
	mk        *secrets.MasterKey
	encPath   string
	noncePath string
}

// NewSecretStoreAdapter creates an adapter that reads/writes the cluster
// secret from AES-256-GCM encrypted files.
func NewSecretStoreAdapter(mk *secrets.MasterKey, encPath, noncePath string) *SecretStoreAdapter {
	if encPath == "" {
		encPath = "./data/cluster_secret.enc"
	}
	if noncePath == "" {
		noncePath = "./data/cluster_secret.nonce"
	}
	return &SecretStoreAdapter{mk: mk, encPath: encPath, noncePath: noncePath}
}

func (a *SecretStoreAdapter) Load() ([]byte, error) {
	if a.mk == nil {
		return nil, fmt.Errorf("master key not available")
	}
	encrypted, err := os.ReadFile(a.encPath)
	if err != nil {
		return nil, err
	}
	nonce, err := os.ReadFile(a.noncePath)
	if err != nil {
		return nil, err
	}
	plaintext, err := secrets.Decrypt(a.mk, string(encrypted), string(nonce))
	if err != nil {
		return nil, err
	}
	return []byte(plaintext), nil
}

func (a *SecretStoreAdapter) Save(secret []byte) error {
	if a.mk == nil {
		return fmt.Errorf("master key not available")
	}
	encrypted, nonce, err := secrets.Encrypt(a.mk, string(secret))
	if err != nil {
		return err
	}
	if err := os.MkdirAll("./data", 0700); err != nil {
		return err
	}
	if err := os.WriteFile(a.encPath, []byte(encrypted), 0600); err != nil {
		return err
	}
	return os.WriteFile(a.noncePath, []byte(nonce), 0600)
}

// ============================================================================
// NodeChecker — delegates to Aegis node table
// ============================================================================

// NodeCheckerAdapter wraps Aegis's node repository.
type NodeCheckerAdapter struct {
	repo *node.Repository
}

// NewNodeCheckerAdapter creates an adapter backed by the Aegis nodes table.
func NewNodeCheckerAdapter(repo *node.Repository) *NodeCheckerAdapter {
	return &NodeCheckerAdapter{repo: repo}
}

func (a *NodeCheckerAdapter) FindByIP(ip string) (*serviceauth.NodeInfo, error) {
	if a.repo == nil {
		return nil, serviceauth.ErrNotInCluster
	}
	// Scan all known nodes — fine for small clusters (Aegis's target scale).
	all, err := a.repo.FindAll()
	if err != nil {
		return nil, serviceauth.ErrNotInCluster
	}
	for _, n := range all {
		if n.LocalIP == ip || n.PrivateIP == ip || n.PublicIP == ip {
			return &serviceauth.NodeInfo{
				NodeID:    n.NodeID,
				LocalIP:   n.LocalIP,
				PrivateIP: n.PrivateIP,
			}, nil
		}
	}
	return nil, serviceauth.ErrNotInCluster
}

// ============================================================================
// LogWriter — delegates to Aegis audit log
// ============================================================================

// LogWriterAdapter writes call records via Aegis's log service.
type LogWriterAdapter struct {
	logSvc logs.Logger
}

// NewLogWriterAdapter creates an adapter backed by Aegis audit logging.
func NewLogWriterAdapter(logSvc logs.Logger) *LogWriterAdapter {
	return &LogWriterAdapter{logSvc: logSvc}
}

func (a *LogWriterAdapter) WriteCallLog(caller, target, api, callerHost, targetHost string, allowed bool, latencyMs int, errMsg string) error {
	if a.logSvc == nil {
		return nil
	}
	details := fmt.Sprintf("caller=%s target=%s api=%s host=%s allowed=%v latency=%dms",
		caller, target, api, callerHost, allowed, latencyMs)
	if errMsg != "" {
		details += " error=" + errMsg
	}
	a.logSvc.Log(context.Background(), "service-auth.call", "service_auth", caller, "info", details, "system")
	return nil
}
