// Package serviceauth is the Go SDK for the Aegis service-auth cluster.
//
// Import this package in your Go service to automatically register with the
// cluster, call other services by URL, and protect your own endpoints.
//
//	func main() {
//	    client, _ := serviceauth.New(serviceauth.Config{
//	        ServiceName: "my-service",
//	    })
//	    client.Register(context.Background())
//	    defer client.Close()
//
//	    // Call any service by URL — ticket is auto-signed
//	    resp, _ := client.Post("https://other-service.example.com/api/action", body)
//
//	    // Protect endpoints with Guard middleware
//	    http.Handle("GET /api/resource", client.Guard(handler))
//	}
package serviceauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config is the SDK initialisation config. Only ServiceName is required.
type Config struct {
	ServiceName string   // required — logical name in the cluster
	AegisURL    string   // optional — auto-detected if empty
	HTTPClient  *http.Client
	IPChecker   IPChecker // nil = cluster-only (default)
}

// Client manages the lifecycle of a service in the Aegis auth cluster.
// It is safe for concurrent use.
type Client struct {
	cfg        Config
	gatewayURL string
	serviceID  string
	instanceID string // unique per process lifetime, for heartbeat tracking

	privateKey string            // Ed25519 private key (base64, stored locally)
	publicKey  string            // Ed25519 public key (base64, sent to server)
	publicKeys map[string][]string // name → public_key (synced)
	groups     []ServiceGroup
	policies   []Policy
	blocklist  []BlocklistEntry
	blVersion  int64
	keyDir     string // dir for storing private key
	ipChecker  IPChecker // 调用方 IP 检查，默认允许内网

	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.RWMutex
}

// New creates a Client. Call Register() afterwards to join the cluster.
func New(cfg Config) (*Client, error) {
	if cfg.ServiceName == "" {
		return nil, fmt.Errorf("serviceauth: ServiceName is required")
	}

	gatewayURL := cfg.AegisURL
	if gatewayURL == "" {
		var err error
		gatewayURL, err = autoDetectAegis()
		if err != nil {
			return nil, fmt.Errorf("serviceauth: auto-detect aegis: %w", err)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	ctx, cancel := context.WithCancel(context.Background())

	ipChecker := cfg.IPChecker
	if ipChecker == nil {
		ipChecker = AllowCluster()
	}

	instanceID := generateInstanceID()

	c := &Client{
		cfg:        cfg,
		gatewayURL: gatewayURL,
		instanceID: instanceID,
		httpClient: httpClient,
		publicKeys: make(map[string][]string),
		keyDir:     keyDir(),
		ipChecker:  ipChecker,
		ctx:        ctx,
		cancel:     cancel,
	}

	return c, nil
}

func generateInstanceID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ins_%x", time.Now().UnixNano())
	}
	return "ins_" + hex.EncodeToString(b)
}

// Register joins the cluster and starts background sync.
func (c *Client) Register(ctx context.Context) error {
	// Load or generate Ed25519 key pair.
	pubKey, privKey, err := c.loadOrGenerateKeyPair()
	if err != nil {
		return fmt.Errorf("register: keys: %w", err)
	}
	c.publicKey = pubKey
	c.privateKey = privKey

	reqBody := RegisterRequest{
		ServiceName: c.cfg.ServiceName,
		PublicKey:   pubKey,
			InstanceID:  c.instanceID,
	}

	data, _ := json.Marshal(reqBody)
	resp, err := c.httpClient.Post(
		c.gatewayURL+"/api/service-auth/v1/register",
		"application/json",
		bytes.NewReader(data),
	)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register: %s: %s", resp.Status, string(body))
	}

	var regResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("register: decode: %w", err)
	}

	c.mu.Lock()
	c.serviceID = regResp.ServiceID
	c.blVersion = regResp.BlVersion
	c.blocklist = regResp.Blocklist
	if regResp.PublicKeys != nil {
		c.publicKeys = regResp.PublicKeys
	}
	c.mu.Unlock()

	syncInterval := time.Duration(regResp.SyncInterval) * time.Second
	if syncInterval < 10*time.Second {
		syncInterval = 30 * time.Second
	}
	c.wg.Add(1)
	go c.syncLoop(syncInterval)

	log.Printf("[serviceauth] registered as %s (%s)", c.cfg.ServiceName, c.serviceID)
	return nil
}

// ============================================================================
// URL-based HTTP methods — each request carries an auto-signed Ed25519 ticket.
// ============================================================================

// Post sends a POST request with an auto-signed service ticket.
func (c *Client) Post(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("post: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// Get sends a GET request with an auto-signed service ticket.
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}
	return c.Do(req)
}

// Put sends a PUT request with an auto-signed service ticket.
func (c *Client) Put(ctx context.Context, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return nil, fmt.Errorf("put: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

// Delete sends a DELETE request with an auto-signed service ticket.
func (c *Client) Delete(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("delete: %w", err)
	}
	return c.Do(req)
}

// Do sends an HTTP request with an auto-signed service ticket attached.
// The ticket proves the caller's identity via Ed25519 signature.
// After a successful (2xx) response, automatically reports the call
// to the ServiceAuth cluster so the target service can see who called it.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	c.mu.RLock()
	privKey := c.privateKey
	caller := c.cfg.ServiceName
	c.mu.RUnlock()

	ticket := SignTicket(NewTicket(caller), privKey)

	req.Header.Set("X-Service-Ticket", ticket)
	req.Header.Set("X-Caller-Service", caller)

	resp, err := c.httpClient.Do(req)
	if err == nil && resp.StatusCode < 300 {
		// Fire-and-forget report: the SDK records who it called.
		target := req.URL.Host
		api := req.URL.Path
		go func() {
			reportReq := ReportRequest{
				CallerService: caller,
				TargetService: target,
				TargetAPI:     api,
				Allowed:       true,
			}
			data, _ := json.Marshal(reportReq)
			c.httpClient.Post(c.gatewayURL+"/api/service-auth/v1/report", "application/json", bytes.NewReader(data))
		}()
	}
	return resp, err
}

// ============================================================================
// Health check
// ============================================================================

// Healthy returns true if the given URL responds with 200 within 5 seconds.
func (c *Client) Healthy(url string) bool {
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// ============================================================================
// Close
// ============================================================================

// Close stops the background sync loop.
func (c *Client) Close() error {
	c.cancel()
	c.wg.Wait()
	return nil
}

// ServiceID returns the ID assigned by the cluster on registration.
func (c *Client) ServiceID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serviceID
}

// PublicKey returns this service's Ed25519 public key (base64).
func (c *Client) PublicKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publicKey
}

// PrivateKey returns this service's Ed25519 private key (base64). Test use only.
func (c *Client) PrivateKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.privateKey
}

// SetIPChecker replaces the IP checker used by Guard.
// WhitelistChecker 的条目最长 24h，硬编码不可绕过。
func (c *Client) SetIPChecker(checker IPChecker) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ipChecker = checker
}

// Groups returns the current service groups (for policy evaluation).

// Policies returns the current policies.

// ─── Scoped service discovery ───────────────────────────────────────────

// CallerEdge is one caller relationship.
type CallerEdge struct {
	Service  string `json:"service"`
	API      string `json:"api"`
	Count    int64  `json:"count"`
	LastSeen string `json:"last_seen"`
}

// DepEdge is one dependency relationship.
type DepEdge struct {
	Service  string `json:"service"`
	API      string `json:"api"`
	Count    int64  `json:"count"`
	LastSeen string `json:"last_seen"`
}

// Callers returns services that have called this service.
func (c *Client) Callers(ctx context.Context, window string) ([]CallerEdge, error) {
	url := c.gatewayURL + "/api/service-auth/v1/services?window=" + window
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("callers: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Service string       `json:"service"`
		Callers []CallerEdge `json:"callers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("callers: decode: %w", err)
	}
	return result.Callers, nil
}

// Deps returns services that this service depends on (has called).
func (c *Client) Deps(ctx context.Context, window string) ([]DepEdge, error) {
	url := c.gatewayURL + "/api/service-auth/v1/services?window=" + window
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("deps: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		Service string    `json:"service"`
		Deps    []DepEdge `json:"deps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("deps: decode: %w", err)
	}
	return result.Deps, nil
}

// InGroup returns true if the named service belongs to the group. Local lookup.

// ListGroupMembers returns all service names in a group. Local lookup.

// ============================================================================
// Internal
// ============================================================================

func (c *Client) syncLoop(interval time.Duration) {
	defer c.wg.Done()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.doSync()
		case <-c.ctx.Done():
			return
		}
	}
}

func (c *Client) doSync() {
	c.mu.RLock()
	blVer := c.blVersion
	gateway := c.gatewayURL
	c.mu.RUnlock()

	url := fmt.Sprintf("%s/api/service-auth/v1/sync?bl_version=%d", gateway, blVer)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 304 {
		return
	}
	if resp.StatusCode != 200 {
		return
	}

	var syncResp SyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return
	}
	if syncResp.NotModified {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.blVersion = syncResp.BlVersion

	if len(syncResp.Blocklist) > 0 {
		c.blocklist = syncResp.Blocklist
	}
	if syncResp.PublicKeys != nil {
		c.publicKeys = syncResp.PublicKeys
	}
	if len(syncResp.Groups) > 0 {
	}
	if len(syncResp.Policies) > 0 {
	}
}

// ─── Keypair management ───

func keyDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		return ".aegis/keys"
	}
	return dir + "/.aegis/keys"
}

func (c *Client) keyPath() string {
	return c.keyDir + "/" + c.cfg.ServiceName + ".key"
}

func (c *Client) loadOrGenerateKeyPair() (pubKey, privKey string, err error) {
	os.MkdirAll(c.keyDir, 0700)

	// Try loading existing key.
	if data, err := os.ReadFile(c.keyPath()); err == nil && len(data) > 0 {
		privKey = string(data)
		pub, genErr := ed25519PrivateToPublic(privKey)
		if genErr == nil {
			return pub, privKey, nil
		}
		log.Printf("[serviceauth] corrupt key, regenerating: %v", genErr)
	}

	// Generate new key pair.
	pub, priv, err := GenerateKeyPair()
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(c.keyPath(), []byte(priv), 0600); err != nil {
		return "", "", fmt.Errorf("save key: %w", err)
	}
	log.Printf("[serviceauth] generated new Ed25519 key for %s", c.cfg.ServiceName)
	return pub, priv, nil
}

func ed25519PrivateToPublic(privKeyB64 string) (string, error) {
	privBytes, err := base64.StdEncoding.DecodeString(privKeyB64)
	if err != nil {
		return "", err
	}
	priv := ed25519.PrivateKey(privBytes)
	pub := priv.Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub), nil
}
