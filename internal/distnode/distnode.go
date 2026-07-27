// Package distnode provides a reusable distributed node runtime.
//
// Identity, membership, transport, and role — one self-contained package
// that any Go project can embed to get multi-node awareness.
//
// Usage:
//
//	cfg := distnode.Config{
//	    ID:    "node_a",
//	    Name:  "panel-1",
//	    Addr:  "10.0.0.1:7380",
//	    Secret: "cluster-shared-secret",
//	    Peers: []distnode.PeerConfig{
//	        {ID: "node_b", Addr: "10.0.0.2:7380"},
//	    },
//	}
//	dn := distnode.New(cfg)
//	dn.Start(ctx)
//	defer dn.Stop()
//
//	// Register a method other nodes can call
//	dn.Transport.Register("ListRoutes", handler)
//
//	// Call a method on another node
//	var result ListRoutesResponse
//	err := dn.Transport.Call(ctx, "node_b", "ListRoutes", req, &result)
package distnode

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Errors
// ──────────────────────────────────────────────────────────────────────────────

var (
	ErrPeerNotFound   = errors.New("distnode: peer not found")
	ErrPeerDead       = errors.New("distnode: peer is not alive")
	ErrMethodNotFound = errors.New("distnode: method not found")
	ErrCallFailed     = errors.New("distnode: call failed")
	ErrUnauthorized   = errors.New("distnode: unauthorized")
)

// CallError wraps a remote call failure with the target method name.
type CallError struct {
	Method string
	Err    error
}

func (e *CallError) Error() string { return fmt.Sprintf("distnode: call %s: %v", e.Method, e.Err) }
func (e *CallError) Unwrap() error { return e.Err }

// ──────────────────────────────────────────────────────────────────────────────
// Config
// ──────────────────────────────────────────────────────────────────────────────

// Config defines the static identity and peer list for a node.
// Designed to be loaded from YAML/JSON by the embedding project.
type Config struct {
	ID           string       `yaml:"id" json:"id"`                                             // unique node ID
	Name         string       `yaml:"name" json:"name"`                                         // human-readable name
	Addr         string       `yaml:"addr" json:"addr"`                                         // advertised address (for example "10.0.0.1:7380")
	Secret       string       `yaml:"secret" json:"secret"`                                     // cluster shared secret
	Peers        []PeerConfig `yaml:"peers" json:"peers"`                                       // statically known peers
	Scheme       string       `yaml:"scheme,omitempty" json:"scheme,omitempty"`                 // peer HTTP scheme
	HealthPath   string       `yaml:"health_path,omitempty" json:"health_path,omitempty"`       // peer health-check path
	CallPath     string       `yaml:"call_path,omitempty" json:"call_path,omitempty"`           // peer RPC path
	NodeIDHeader string       `yaml:"node_id_header,omitempty" json:"node_id_header,omitempty"` // caller identity header
	DefaultRole  string       `yaml:"default_role,omitempty" json:"default_role,omitempty"`     // initial self-declared role
}

const (
	defaultScheme       = "http"
	defaultHealthPath   = "/healthz"
	defaultCallPath     = "/distnode/call"
	defaultNodeIDHeader = "X-DistNode-ID"
	defaultRole         = "node"
)

func (cfg Config) withDefaults() Config {
	if cfg.Scheme == "" {
		cfg.Scheme = defaultScheme
	}
	cfg.Scheme = strings.TrimRight(cfg.Scheme, ":/")
	if cfg.HealthPath == "" {
		cfg.HealthPath = defaultHealthPath
	}
	cfg.HealthPath = ensureLeadingSlash(cfg.HealthPath)
	if cfg.CallPath == "" {
		cfg.CallPath = defaultCallPath
	}
	cfg.CallPath = ensureLeadingSlash(cfg.CallPath)
	if cfg.NodeIDHeader == "" {
		cfg.NodeIDHeader = defaultNodeIDHeader
	}
	if cfg.DefaultRole == "" {
		cfg.DefaultRole = defaultRole
	}
	return cfg
}

func ensureLeadingSlash(path string) string {
	if path == "" || strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func peerURL(scheme, addr, path string) string {
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return strings.TrimRight(addr, "/") + ensureLeadingSlash(path)
	}
	return strings.TrimRight(scheme, ":/") + "://" + strings.TrimRight(addr, "/") + ensureLeadingSlash(path)
}

// PeerConfig defines a known peer's identity.
type PeerConfig struct {
	ID   string `yaml:"id" json:"id"`
	Addr string `yaml:"addr" json:"addr"`
	// Secret is this peer's own credential (per-peer auth, Phase 1). When set,
	// calls FROM this peer are verified against it, and revoking the peer denies
	// only that node. Empty falls back to the cluster shared secret (Phase 0),
	// so existing single-secret deployments keep working byte-for-byte.
	Secret string `yaml:"secret,omitempty" json:"secret,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// NodeInfo & Peer
// ──────────────────────────────────────────────────────────────────────────────

// NodeInfo is the public identity of any node in the cluster.
type NodeInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Addr   string `json:"addr"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

const (
	StatusAlive = "alive"
	StatusDead  = "dead"
)

// Peer tracks the runtime state of a known cluster member.
type Peer struct {
	Info      NodeInfo  `json:"info"`
	Alive     bool      `json:"alive"`
	AliveAt   time.Time `json:"alive_at,omitempty"`
	DeadAt    time.Time `json:"dead_at,omitempty"`
	FailCount int       `json:"-"`
	// Secret is this peer's per-peer credential (Phase 1). Empty = use the
	// cluster shared secret to verify this peer's calls (Phase 0).
	Secret string `json:"-"`
	// Revoked denies this peer's cluster access regardless of a valid token.
	// Cryptographically enforced when Secret is set; cooperative otherwise.
	Revoked bool `json:"revoked,omitempty"`
}

// ──────────────────────────────────────────────────────────────────────────────
// Events
// ──────────────────────────────────────────────────────────────────────────────

// EventType describes a membership change.
type EventType int

const (
	EventPeerJoined EventType = iota
	EventPeerLeft
	EventPeerAlive
	EventPeerDead
)

func (e EventType) String() string {
	switch e {
	case EventPeerJoined:
		return "joined"
	case EventPeerLeft:
		return "left"
	case EventPeerAlive:
		return "alive"
	case EventPeerDead:
		return "dead"
	default:
		return "unknown"
	}
}

// PeerEvent is emitted when a peer's membership state changes.
type PeerEvent struct {
	Type EventType `json:"type"`
	Peer *Peer     `json:"peer"`
}

// PeerEventHandler is a callback for membership changes.
type PeerEventHandler func(PeerEvent)

// ──────────────────────────────────────────────────────────────────────────────
// Identity
// ──────────────────────────────────────────────────────────────────────────────

// Identity manages this node's authentication credentials.
// Uses HMAC-SHA256 for token verification (same scheme as Aegis nodeauth).
// The Secret is shared across the cluster for Phase 0 simplicity;
// projects can replace this with per-node credentials or mTLS later.
type Identity struct {
	nodeID string
	secret string
}

func newIdentity(cfg Config) *Identity {
	return &Identity{
		nodeID: cfg.ID,
		secret: cfg.Secret,
	}
}

// NodeID returns this node's identity.
func (id *Identity) NodeID() string { return id.nodeID }

// Token returns the bearer token this node uses when calling peers.
func (id *Identity) Token() string { return id.secret }

// Authenticate verifies a bearer token against the shared secret.
// Returns the authenticated node ID on success.
// In Phase 0 all nodes share a secret; in future this can verify
// per-node credentials stored in the receiver's DB.
func (id *Identity) Authenticate(token string) (string, error) {
	if token == "" {
		return "", ErrUnauthorized
	}
	// Phase 0: constant-time comparison against shared secret
	// Future: look up token hash in node_credentials table
	if !hmac.Equal([]byte(token), []byte(id.secret)) {
		return "", ErrUnauthorized
	}
	// We don't know the caller's ID from a shared secret alone.
	// For per-node auth, extract from the token lookup result.
	return "", nil
}

// authHeader builds the Authorization header value.
func (id *Identity) authHeader() string {
	return "Bearer " + id.secret
}

// ──────────────────────────────────────────────────────────────────────────────
// Membership
// ──────────────────────────────────────────────────────────────────────────────

// Membership tracks known cluster peers and monitors their liveness.
//
// Current implementation: static config + periodic HTTP health checks.
// Future: replace with SWIM/gossip protocol for auto-discovery.
//
// Thread-safe.
type Membership struct {
	self       string
	selfAddr   string
	scheme     string
	healthPath string
	peers      map[string]*Peer
	checkIntv  time.Duration
	onEvent    PeerEventHandler

	httpClient *http.Client
	mu         sync.RWMutex
	stopFn     context.CancelFunc
	logf       func(format string, args ...any)
}

const (
	healthCheckInterval = 15 * time.Second
	healthCheckTimeout  = 5 * time.Second
	maxFailCount        = 3
)

func newMembership(cfg Config) *Membership {
	cfg = cfg.withDefaults()
	m := &Membership{
		self:       cfg.ID,
		selfAddr:   cfg.Addr,
		scheme:     cfg.Scheme,
		healthPath: cfg.HealthPath,
		peers:      make(map[string]*Peer),
		checkIntv:  healthCheckInterval,
		httpClient: &http.Client{Timeout: healthCheckTimeout},
		logf:       log.Printf,
	}
	for _, pc := range cfg.Peers {
		if pc.ID == cfg.ID {
			continue // skip self
		}
		m.peers[pc.ID] = &Peer{
			Info: NodeInfo{
				ID:     pc.ID,
				Addr:   pc.Addr,
				Status: StatusDead,
			},
			Alive:  false,
			Secret: pc.Secret,
		}
	}
	return m
}

// Start begins the periodic health-check loop.
// Blocks until ctx is cancelled. Typically called in a goroutine.
func (m *Membership) Start(ctx context.Context) {
	ctx, m.stopFn = context.WithCancel(ctx)
	m.checkAll(ctx) // immediate first check

	ticker := time.NewTicker(m.checkIntv)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

// Stop terminates the health-check loop.
func (m *Membership) Stop() {
	if m.stopFn != nil {
		m.stopFn()
	}
}

// OnEvent registers a callback for membership state changes.
func (m *Membership) OnEvent(handler PeerEventHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onEvent = handler
}

// GetPeer returns a peer by ID, or nil if unknown.
func (m *Membership) GetPeer(id string) *Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.peers[id]
}

// AddPeer dynamically registers a peer at runtime (thread-safe). It is a no-op
// when the ID is empty, equals self, or is already known. The next health-check
// tick probes the new peer; the first successful probe fires EventPeerAlive
// through the registered OnEvent handler. This lets a running cluster grow
// without a restart — e.g. when an operator joins a node to the control plane.
func (m *Membership) AddPeer(pc PeerConfig) {
	if pc.ID == "" || pc.ID == m.self {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.peers[pc.ID]; exists {
		return
	}
	m.peers[pc.ID] = &Peer{
		Info: NodeInfo{
			ID:     pc.ID,
			Addr:   pc.Addr,
			Status: StatusDead,
		},
		Alive:  false,
		Secret: pc.Secret,
	}
	m.logf("[distnode] peer %s (%s) added dynamically", pc.ID, pc.Addr)
}

// PeerSecret returns the per-peer credential for id, or "" if the peer is
// unknown or has no per-peer secret (falls back to the cluster shared secret).
func (m *Membership) PeerSecret(id string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if p := m.peers[id]; p != nil {
		return p.Secret
	}
	return ""
}

// IsRevoked reports whether the peer's cluster access has been revoked.
func (m *Membership) IsRevoked(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p := m.peers[id]
	return p != nil && p.Revoked
}

// RevokePeer denies a peer's cluster access. Enforced cryptographically when the
// peer has a per-peer secret; cooperative (ID-based) otherwise. Returns false if
// the peer is unknown.
func (m *Membership) RevokePeer(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.peers[id]
	if p == nil {
		return false
	}
	p.Revoked = true
	m.logf("[distnode] peer %s revoked", id)
	return true
}

// AlivePeers returns all peers currently marked alive.
func (m *Membership) AlivePeers() []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Peer, 0, len(m.peers))
	for _, p := range m.peers {
		if p.Alive {
			out = append(out, p)
		}
	}
	return out
}

// AllPeers returns all known peers regardless of liveness.
func (m *Membership) AllPeers() []*Peer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Peer, 0, len(m.peers))
	for _, p := range m.peers {
		out = append(out, p)
	}
	return out
}

// PeerCount returns the total number of known peers (excluding self).
func (m *Membership) PeerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.peers)
}

// SelfInfo returns this node's own identity.
func (m *Membership) SelfInfo() NodeInfo {
	return NodeInfo{
		ID:   m.self,
		Addr: m.selfAddr,
	}
}

// ── health check internals ──

func (m *Membership) checkAll(ctx context.Context) {
	m.mu.RLock()
	peers := make([]*Peer, 0, len(m.peers))
	for _, p := range m.peers {
		peers = append(peers, p)
	}
	m.mu.RUnlock()

	for _, p := range peers {
		m.checkOne(ctx, p)
	}
}

func (m *Membership) checkOne(ctx context.Context, p *Peer) {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	alive := false
	req, err := http.NewRequestWithContext(checkCtx, "GET",
		peerURL(m.scheme, p.Info.Addr, m.healthPath), nil)
	if err == nil {
		resp, herr := m.httpClient.Do(req)
		if herr == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				alive = true
			}
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	prevAlive := p.Alive
	if alive {
		p.FailCount = 0
		if !prevAlive {
			p.Alive = true
			p.AliveAt = time.Now()
			p.Info.Status = StatusAlive
			m.logf("[distnode] peer %s (%s) is alive", p.Info.ID, p.Info.Addr)
			m.fireEvent(PeerEvent{Type: EventPeerAlive, Peer: p})
		}
	} else {
		p.FailCount++
		if p.FailCount >= maxFailCount && prevAlive {
			p.Alive = false
			p.DeadAt = time.Now()
			p.Info.Status = StatusDead
			m.logf("[distnode] peer %s (%s) is dead (fail=%d)", p.Info.ID, p.Info.Addr, p.FailCount)
			m.fireEvent(PeerEvent{Type: EventPeerDead, Peer: p})
		}
	}
}

func (m *Membership) fireEvent(evt PeerEvent) {
	if m.onEvent != nil {
		m.onEvent(evt)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Transport
// ──────────────────────────────────────────────────────────────────────────────

// Handler processes a remote method call.
// Args arrive as raw JSON; the handler unmarshals them itself.
type Handler func(ctx context.Context, callerID string, args json.RawMessage) (any, error)

// Transport provides cross-node method calls.
// Each node registers methods it exposes and can call methods on any peer.
type Transport struct {
	selfID       string
	secret       string
	scheme       string
	callPath     string
	nodeIDHeader string
	handlers     map[string]Handler
	members      *Membership
	client       *http.Client
	mu           sync.RWMutex
}

func newTransport(cfg Config, members *Membership) *Transport {
	cfg = cfg.withDefaults()
	return &Transport{
		selfID:       cfg.ID,
		secret:       cfg.Secret,
		scheme:       cfg.Scheme,
		callPath:     cfg.CallPath,
		nodeIDHeader: cfg.NodeIDHeader,
		handlers:     make(map[string]Handler),
		members:      members,
		client:       &http.Client{Timeout: 30 * time.Second},
	}
}

// Register registers a method handler that remote nodes can call.
// Method names should follow "Service.Method" convention, e.g. "Aegis.ListRoutes".
func (t *Transport) Register(method string, handler Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[method] = handler
}

// Call invokes a method on a remote node.
//
//   - targetID: the peer's node ID (must be in membership)
//   - method:   name registered by Register on the target
//   - args:     request payload (will be JSON-serialized)
//   - reply:    response will be JSON-deserialized into this (pass a pointer)
//
// Returns ErrPeerDead if the target is known but not currently alive.
// Returns CallError wrapping the underlying error on transport failure.
func (t *Transport) Call(ctx context.Context, targetID, method string, args, reply any) error {
	peer := t.members.GetPeer(targetID)
	if peer == nil {
		return fmt.Errorf("%w: %s", ErrPeerNotFound, targetID)
	}
	if !peer.Alive {
		return fmt.Errorf("%w: %s (%s)", ErrPeerDead, targetID, peer.Info.Addr)
	}

	argsRaw, err := json.Marshal(args)
	if err != nil {
		return &CallError{Method: method, Err: fmt.Errorf("marshal args: %w", err)}
	}
	body, err := json.Marshal(callRequest{Method: method, Args: argsRaw})
	if err != nil {
		return &CallError{Method: method, Err: fmt.Errorf("marshal request: %w", err)}
	}

	url := peerURL(t.scheme, peer.Info.Addr, t.callPath)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return &CallError{Method: method, Err: err}
	}
	req.Header.Set("Authorization", "Bearer "+t.secret)
	req.Header.Set(t.nodeIDHeader, t.selfID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		t.members.checkOne(ctx, peer) // provoke failure detection
		return &CallError{Method: method, Err: fmt.Errorf("http: %w", err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return &CallError{Method: method, Err: ErrMethodNotFound}
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &CallError{Method: method, Err: ErrUnauthorized}
	}
	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return &CallError{Method: method, Err: fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(errBody))}
	}

	var cr callResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return &CallError{Method: method, Err: fmt.Errorf("decode response: %w", err)}
	}

	if reply != nil {
		if err := json.Unmarshal(cr.Result, reply); err != nil {
			return &CallError{Method: method, Err: fmt.Errorf("unmarshal reply: %w", err)}
		}
	}
	return nil
}

// Handler returns an http.Handler that serves the configured RPC endpoint.
// Mount it in the embedding project's HTTP mux:
//
//	mux.Handle("POST "+dn.Config.CallPath, dn.Transport.Handler())
func (t *Transport) Handler() http.Handler {
	return http.HandlerFunc(t.serveCall)
}

// ── transport internals ──

type callRequest struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args,omitempty"`
}

type callResponse struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func (t *Transport) serveCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate the caller
	auth := r.Header.Get("Authorization")
	callerID, err := t.authenticateCaller(r.Header.Get(t.nodeIDHeader), auth)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Parse request
	var req callRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Route to registered handler
	t.mu.RLock()
	handler, ok := t.handlers[req.Method]
	t.mu.RUnlock()

	if !ok {
		http.Error(w, "unknown method: "+req.Method, http.StatusNotFound)
		return
	}

	// Call the handler
	result, herr := handler(r.Context(), callerID, req.Args)
	if herr != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(callResponse{Error: herr.Error()})
		return
	}

	// Return result
	json.NewEncoder(w).Encode(callResponse{Result: mustMarshalJSON(result)})
}

// authenticateCaller verifies an incoming call. callerID comes from the
// configured node ID header (may be empty for legacy callers), auth from the
// Authorization header.
//
// Resolution (Phase 1, backward-compatible):
//   - Known + revoked peer            → reject
//   - Known peer with per-peer secret → verify against THAT secret (strict)
//   - Otherwise                       → verify against the cluster shared secret
//
// So pure Phase-0 clusters (no per-peer secrets, no revocations) behave exactly
// as before; per-peer secrets enable real per-node revocation.
func (t *Transport) authenticateCaller(callerID, auth string) (string, error) {
	if auth == "" {
		return "", ErrUnauthorized
	}
	var token string
	if len(auth) > 7 && auth[:7] == "Bearer " {
		token = auth[7:]
	}
	if token == "" {
		return "", ErrUnauthorized
	}

	// Per-peer path: only when the caller is identified and known.
	if callerID != "" && t.members != nil {
		if t.members.IsRevoked(callerID) {
			return "", ErrUnauthorized
		}
		if peerSecret := t.members.PeerSecret(callerID); peerSecret != "" {
			if !hmac.Equal([]byte(token), []byte(peerSecret)) {
				return "", ErrUnauthorized
			}
			return callerID, nil
		}
	}

	// Fallback: cluster shared secret (Phase 0).
	if !hmac.Equal([]byte(token), []byte(t.secret)) {
		return "", ErrUnauthorized
	}
	return callerID, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Role
// ──────────────────────────────────────────────────────────────────────────────

// Role manages this node's self-declared role.
//
// Roles are project-defined strings, not externally assigned by DistNode.
// The embedding system can confirm, reject, or override the value according to
// its own rules.
type Role struct {
	self    string
	current string
	mu      sync.RWMutex
}

func newRole(cfg Config) *Role {
	cfg = cfg.withDefaults()
	return &Role{
		self:    cfg.ID,
		current: cfg.DefaultRole,
	}
}

// Declare sets the node's self-declared role.
// This does NOT ask permission — the node decides.
// The system can reject or override via its own logic.
func (r *Role) Declare(role string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.current = role
}

// Is reports whether the node currently has the given role.
func (r *Role) Is(role string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current == role
}

// Current returns the current role string.
func (r *Role) Current() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.current
}

// ──────────────────────────────────────────────────────────────────────────────
// Storage — optional abstraction for data persistence
// ──────────────────────────────────────────────────────────────────────────────

// StorageDriver is the persistence interface for distributed node data.
// The base distnode layer does not require storage — it handles identity,
// membership, and transport purely in memory. Storage becomes relevant
// when your project needs to store per-node state that survives restarts
// or syncs across the cluster.
//
// Implementations:
//   - SQLiteStorage: local file (single-writer, simple)
//   - EtcdStorage:   distributed KV (multi-writer, consensus)
//   - PostgreSQL:    centralized (external dependency)
//
// The embedding project chooses the implementation. distnode does not
// enforce one. Use it like:
//
//	type MyNode struct {
//	    *distnode.DistNode
//	    db   *sql.DB  // or etcd client, or Postgres pool
//	}
//
// SEE ALSO: distnode.Config carries no storage config; add your own in
// the embedding project's config and pass it to your storage driver.
type StorageDriver interface {
	// Get retrieves a value by key. Returns (nil, nil) if not found.
	Get(key string) ([]byte, error)
	// Set stores a value by key.
	Set(key string, value []byte) error
	// Delete removes a key. No error if not found.
	Delete(key string) error
	// List returns all keys with the given prefix.
	List(prefix string) ([]string, error)
}

// ──────────────────────────────────────────────────────────────────────────────
// DistNode — top-level facade
// ──────────────────────────────────────────────────────────────────────────────

// DistNode is the top-level distributed node runtime.
// Embed it in your project's node struct to get identity, membership,
// transport, and role self-declaration.
//
//	type MyNode struct {
//	    *distnode.DistNode  // embed
//	    // business fields...
//	}
type DistNode struct {
	ID         string
	Config     Config
	Identity   *Identity
	Membership *Membership
	Transport  *Transport
	Role       *Role

	startOnce sync.Once
	stopOnce  sync.Once
	cancel    context.CancelFunc
}

// New creates a DistNode from config.
// Does not start any goroutines — call Start() to begin.
func New(cfg Config) *DistNode {
	cfg = cfg.withDefaults()
	id := newIdentity(cfg)
	members := newMembership(cfg)
	transport := newTransport(cfg, members)
	role := newRole(cfg)

	return &DistNode{
		ID:         cfg.ID,
		Config:     cfg,
		Identity:   id,
		Membership: members,
		Transport:  transport,
		Role:       role,
	}
}

// Info returns this node's public NodeInfo.
func (dn *DistNode) Info() NodeInfo {
	return NodeInfo{
		ID:   dn.Config.ID,
		Name: dn.Config.Name,
		Addr: dn.Config.Addr,
		Role: dn.Role.Current(),
	}
}

// Start begins the membership health-check loop.
// Call this after the application has initialized its configured health
// endpoint.
//
// Blocks until ctx is cancelled. Run in a goroutine:
//
//	go dn.Start(ctx)
func (dn *DistNode) Start(ctx context.Context) {
	dn.startOnce.Do(func() {
		cctx, cancel := context.WithCancel(ctx)
		dn.cancel = cancel
		dn.Membership.Start(cctx)
	})
}

// Stop terminates the membership loop and releases resources.
func (dn *DistNode) Stop() {
	dn.stopOnce.Do(func() {
		if dn.cancel != nil {
			dn.cancel()
		}
		dn.Membership.Stop()
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func mustMarshalJSON(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}
