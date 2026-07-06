// Package serviceauth is the Go SDK for the Aegis service-auth cluster.
//
// Import this package in your Go service to automatically register with the
// cluster, call other services, and protect your own endpoints.
//
//	func main() {
//	    client, _ := serviceauth.New(serviceauth.Config{
//	        ServiceName: "my-service",
//	        ServicePort: 8080,
//	        APIs: []serviceauth.APIDef{
//	            {Name: "health", Path: "/health", Method: "GET"},
//	        },
//	    })
//	    client.Register(context.Background())
//	    defer client.Close()
//
//	    http.HandleFunc("GET /health", client.Guard(healthHandler))
//	    http.ListenAndServe(":8080", nil)
//	}
package serviceauth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Config is the SDK initialisation config. Only ServiceName and ServicePort
// are required; everything else has sensible defaults.
type Config struct {
	ServiceName string   // required — logical name in the cluster
	ServicePort int      // required — port this service listens on
	APIs        []APIDef // this service's exposed APIs
	AegisURL    string   // optional — auto-detected if empty
	HTTPClient  *http.Client
}

// Client manages the lifecycle of a service in the Aegis auth cluster.
// It is safe for concurrent use.
type Client struct {
	cfg        Config
	gatewayURL string
	serviceID  string

	privateKey string              // Ed25519 private key (base64, stored locally)
	publicKey  string              // Ed25519 public key (base64, sent to server)
	publicKeys map[string]string   // name → public_key (synced)
	instances  map[string][]ServiceInstance
	apis       map[string][]APIDef
	blocklist  []BlocklistEntry
	blVersion  int64
	catVersion int64
	localHost  string
	nodeHost   string
	keyDir     string // dir for storing private key

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
	if cfg.ServicePort <= 0 {
		return nil, fmt.Errorf("serviceauth: ServicePort is required")
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
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}

	nodeHost, _ := os.Hostname()

	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		cfg:        cfg,
		gatewayURL: gatewayURL,
		httpClient: httpClient,
		instances:  make(map[string][]ServiceInstance),
		apis:       make(map[string][]APIDef),
		publicKeys: make(map[string]string),
		localHost:  detectLocalIP(),
		nodeHost:   nodeHost,
		keyDir:     keyDir(),
		ctx:        ctx,
		cancel:     cancel,
	}

	return c, nil
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

	apis := c.cfg.APIs
	if apis == nil {
		apis = []APIDef{}
	}

	reqBody := RegisterRequest{
		ServiceName: c.cfg.ServiceName,
		Host:        c.localHost,
		Port:        c.cfg.ServicePort,
		NodeHost:    c.nodeHost,
		APIs:        apis,
		PublicKey:   pubKey,
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
	c.catVersion = regResp.CatVersion
	c.blocklist = regResp.Blocklist
	for k, v := range regResp.PublicKeys {
		c.publicKeys[k] = v
	}
	for _, inst := range regResp.Instances {
		c.instances[inst.Name] = append(c.instances[inst.Name], inst)
	}
	for _, api := range regResp.APIs {
		owner := c.findAPIOwner(api)
		c.apis[owner] = append(c.apis[owner], api)
	}
	c.mu.Unlock()

	syncInterval := time.Duration(regResp.SyncInterval) * time.Second
	if syncInterval < 10*time.Second {
		syncInterval = 30 * time.Second
	}
	c.wg.Add(1)
	go c.syncLoop(syncInterval)

	log.Printf("[serviceauth] registered as %s (%s) with %d peers",
		c.cfg.ServiceName, c.serviceID, len(regResp.Instances))
	return nil
}

// Call invokes an API on another service in the cluster.
// The SDK handles ticket generation and instance selection automatically.
// method is optional — if empty, the method from the API definition is used.
func (c *Client) Call(ctx context.Context, targetService, targetAPI string, body io.Reader) (*http.Response, error) {
	return c.callWithMethod(ctx, targetService, targetAPI, "", body)
}

// callWithMethod is the internal implementation that respects the API's
// declared HTTP method.
func (c *Client) callWithMethod(ctx context.Context, targetService, targetAPI, method string, body io.Reader) (*http.Response, error) {
	targetURL, effectiveMethod, err := c.resolveTarget(targetService, targetAPI, method)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}

	ticket := c.generateTicket(targetService, targetAPI)

	req, err := http.NewRequestWithContext(ctx, effectiveMethod, targetURL, body)
	if err != nil {
		return nil, fmt.Errorf("call: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", ticket)
	req.Header.Set("X-Caller-Service", c.cfg.ServiceName)
	req.Header.Set("X-Caller-Host", c.localHost)

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()

	go c.reportCall(targetService, targetAPI, err == nil, int(latency))

	return resp, err
}

// CallAegis invokes an Aegis management API (e.g. bind-http-domain).
// The target service is always "aegis" and the request is sent to the
// gateway URL rather than a service instance.
func (c *Client) CallAegis(ctx context.Context, apiPath, method string, body io.Reader) (*http.Response, error) {
	c.mu.RLock()
	gateway := c.gatewayURL
	c.mu.RUnlock()

	ticket := c.generateTicket("aegis", apiPath)

	url := gateway + apiPath
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("call aegis: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", ticket)
	req.Header.Set("X-Caller-Service", c.cfg.ServiceName)
	req.Header.Set("X-Caller-Host", c.localHost)

	return c.httpClient.Do(req)
}

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

// PublicKey returns this service's Ed25519 public key.
func (c *Client) PublicKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publicKey
}

// ============================================================================
// Internal
// ============================================================================

func (c *Client) resolveTarget(targetService, targetAPI, method string) (url, effectiveMethod string, err error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	instances, ok := c.instances[targetService]
	if !ok || len(instances) == 0 {
		return "", "", fmt.Errorf("service %q not found", targetService)
	}

	// Prefer same-host instance.
	inst := instances[0]
	for _, i := range instances {
		if i.Host == c.localHost {
			inst = i
			break
		}
	}

	// Look up the API definition to get path and method.
	path := "/"
	effectiveMethod = method
	apis, ok := c.apis[targetService]
	if !ok {
		// Fallback: try empty-key (unknown owner) APIs.
		apis = c.apis[""]
	}
	for _, a := range apis {
		if a.Name == targetAPI {
			path = a.Path
			if effectiveMethod == "" {
				effectiveMethod = a.Method
			}
			break
		}
	}
	if effectiveMethod == "" {
		effectiveMethod = "POST" // last-resort default
	}

	return fmt.Sprintf("http://%s:%d%s", inst.Host, inst.Port, path), effectiveMethod, nil
}

func (c *Client) generateTicket(targetService, targetAPI string) string {
	c.mu.RLock()
	privKey := c.privateKey
	c.mu.RUnlock()

	claims := NewTicket(c.cfg.ServiceName, targetService, targetAPI)
	return SignTicket(claims, privKey)
}

func (c *Client) reportCall(targetService, targetAPI string, allowed bool, latencyMs int) {
	req := ReportRequest{
		CallerService: c.cfg.ServiceName,
		TargetService: targetService,
		TargetAPI:     targetAPI,
		CallerHost:    c.localHost,
		Allowed:       allowed,
		LatencyMs:     latencyMs,
	}
	data, _ := json.Marshal(req)

	c.mu.RLock()
	gateway := c.gatewayURL
	c.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", gateway+"/api/service-auth/v1/report", bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err == nil {
		resp.Body.Close()
	}
}

// findAPIOwner attempts to determine which service owns an API definition.
// In the Register response, APIs come back flat. We heuristically match
// by path prefix against known instances.
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

// ─── Internal helpers ───

func (c *Client) findAPIOwner(api APIDef) string {
	// Fallback: most APIs belong to the registering service.
	if api.Path != "" {
		return c.cfg.ServiceName
	}
	return ""
}

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
	catVer := c.catVersion
	gateway := c.gatewayURL
	c.mu.RUnlock()

	url := fmt.Sprintf("%s/api/service-auth/v1/sync?bl_version=%d&cat_version=%d", gateway, blVer, catVer)
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
	c.catVersion = syncResp.CatVersion

	// Merge blocklist + public keys.
	if len(syncResp.Blocklist) > 0 {
		c.blocklist = syncResp.Blocklist
	}
	for k, v := range syncResp.PublicKeys {
		c.publicKeys[k] = v
	}

	// Deduplicate new instances by (name, host, port).
	seen := make(map[string]bool)
	for _, inst := range syncResp.NewInstances {
		key := fmt.Sprintf("%s:%s:%d", inst.Name, inst.Host, inst.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		c.instances[inst.Name] = append(c.instances[inst.Name], inst)
	}
	for _, id := range syncResp.RemovedIDs {
		delete(c.instances, id)
	}
}
