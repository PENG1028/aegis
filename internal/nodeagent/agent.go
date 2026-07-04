package nodeagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"aegis/internal/config"
	"aegis/internal/noderuntime"
	"aegis/internal/provider"
)

const joinTimeout = 30 * time.Second

// Agent is the node agent daemon.
type Agent struct {
	cfg          *noderuntime.Config
	proxyCfg     *config.Config
	client       *noderuntime.Client
	reconciler   *noderuntime.Reconciler
	cache        *noderuntime.CacheManager
	caddyApplier noderuntime.CaddyfileApplier

	// Provider registry — populated in Run() with built-in providers.
	// Used by the gateway status provider to report installed gateway programs.
	provReg *provider.Registry

	stopCh chan struct{}
	doneCh chan struct{}
}

// New creates a new node agent from the given configuration.
func New(nodeCfg *noderuntime.Config, proxyCfg *config.Config) (*Agent, error) {
	if err := nodeCfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &Agent{
		cfg:      nodeCfg,
		proxyCfg: proxyCfg,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}, nil
}

// ensureJoined checks if the node is already registered.
// If not, it uses a join token to register with the control plane.
func (a *Agent) ensureJoined() error {
	if a.cfg.NodeID != "" && a.cfg.NodeToken != "" {
		return nil
	}

	// Try to read join token from env or file
	joinToken := os.Getenv("AEGIS_JOIN_TOKEN")
	if joinToken == "" {
		joinTokenFile := filepath.Join(filepath.Dir(a.cfg.NodeTokenFile), "join.token")
		if data, err := os.ReadFile(joinTokenFile); err == nil {
			joinToken = strings.TrimSpace(string(data))
		}
	}
	if joinToken == "" {
		return fmt.Errorf("no node credential — set AEGIS_JOIN_TOKEN or place join.token beside node_token_file")
	}

	// Call join API
	hostname, _ := os.Hostname()
	nodeID, nodeToken, err := a.callJoin(joinToken, hostname)
	if err != nil {
		return fmt.Errorf("join failed: %w", err)
	}

	// Save credentials
	a.cfg.NodeID = nodeID
	a.cfg.NodeToken = nodeToken

	// Persist token
	if err := os.MkdirAll(filepath.Dir(a.cfg.NodeTokenFile), 0700); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	if err := os.WriteFile(a.cfg.NodeTokenFile, []byte(nodeToken+"\n"), 0600); err != nil {
		return fmt.Errorf("save node token: %w", err)
	}

	log.Printf("[agent] joined control plane as node %s", nodeID)
	return nil
}

// callJoin calls POST /api/node/v1/join and returns the node credentials.
func (a *Agent) callJoin(joinToken, hostname string) (nodeID, nodeToken string, err error) {
	body, _ := json.Marshal(map[string]string{
		"join_token": joinToken,
		"hostname":   hostname,
		"node_name":  hostname,
	})

	ctx, cancel := context.WithTimeout(context.Background(), joinTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST",
		a.cfg.ControlPlaneURL+"/api/node/v1/join", bytes.NewReader(body))
	if err != nil {
		return "", "", fmt.Errorf("create join request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("join request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", "", fmt.Errorf("join returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		NodeID    string `json:"node_id"`
		NodeToken string `json:"node_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decode join response: %w", err)
	}
	if result.NodeID == "" || result.NodeToken == "" {
		return "", "", fmt.Errorf("join response missing node_id or node_token")
	}

	return result.NodeID, result.NodeToken, nil
}

// Run starts the agent and blocks until interrupted.
func (a *Agent) Run() error {
	// Ensure joined
	if err := a.ensureJoined(); err != nil {
		return err
	}

	// Wire up components
	a.client = noderuntime.NewClient(a.cfg.ControlPlaneURL, a.cfg.NodeID, a.cfg.NodeToken)
	a.cache = noderuntime.NewCacheManager(a.cfg.CacheDir)
	// Wire provider discovery → gateway status for heartbeat
	// This populates the gateway inventory table on the control plane with
	// all detected gateway programs (Caddy, HAProxy, etc.) and their status.
	a.provReg = provider.NewRegistry()
	a.provReg.Register(provider.NewCaddyProvider(a.proxyCfg))
	a.provReg.Register(provider.NewHAProxyProvider("", "", a.proxyCfg.Proxy.BackupDir))
	a.reconciler = noderuntime.NewReconciler(a.cfg, a.client, a.cache)
	a.caddyApplier = noderuntime.NewCaddyApplier(a.provReg)
	gwStatusProvider := noderuntime.NewProviderGatewayStatusProvider(a.provReg)
	a.reconciler.SetGatewayStatusProvider(gwStatusProvider)

	log.Printf("[agent] node %s starting — cp=%s heartbeat=%ds sync=%ds",
		a.cfg.NodeID, a.cfg.ControlPlaneURL,
		a.cfg.HeartbeatIntervalSec, a.cfg.SyncIntervalSec)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[nodeagent] panic in agent loop: %v", r)
			}
		}()
		a.loop()
	}()

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
		log.Printf("[agent] received shutdown signal")
	case <-a.stopCh:
	}

	close(a.stopCh)
	<-a.doneCh
	log.Printf("[agent] node %s stopped", a.cfg.NodeID)
	return nil
}

// loop runs the periodic sync loop.
func (a *Agent) loop() {
	defer close(a.doneCh)

	a.syncOnce()

	interval := time.Duration(a.cfg.HeartbeatIntervalSec) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.syncOnce()
		}
	}
}

func (a *Agent) syncOnce() {
	state, err := a.reconciler.SyncOnce()
	if err != nil {
		log.Printf("[agent] sync error: %v", err)
		return
	}
	log.Printf("[agent] sync ok — status=%s rev=%d", state.Status, state.AppliedRevision)
}

// Stop gracefully stops the agent.
func (a *Agent) Stop() {
	select {
	case <-a.stopCh:
	default:
		close(a.stopCh)
	}
	<-a.doneCh
}

// NodeID returns the configured node ID.
func (a *Agent) NodeID() string {
	return a.cfg.NodeID
}
