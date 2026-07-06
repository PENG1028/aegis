package serviceauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// Dependencies — everything the Service needs from the outside world.
// All fields are interfaces or concrete types defined in this package so
// the core has zero imports of Aegis (or any other project) packages.
// ============================================================================

// SecretStore persists the cluster-wide shared secret.
type SecretStore interface {
	// Load returns the stored secret, or an error if none exists yet.
	Load() ([]byte, error)
	// Save persists the secret. Called once on first startup.
	Save(secret []byte) error
}

// NodeChecker decides whether an IP address belongs to the trusted cluster.
type NodeChecker interface {
	// FindByIP returns node info if the IP is recognised, or ErrNotInCluster.
	FindByIP(ip string) (*NodeInfo, error)
}

// NodeInfo is minimal node identity returned by NodeChecker.
type NodeInfo struct {
	NodeID    string
	LocalIP   string
	PrivateIP string
}

// LogRecorder writes inter-service call records.
type LogRecorder interface {
	WriteCallLog(caller, target, api, callerHost, targetHost string, allowed bool, latencyMs int, errMsg string) error
}

// Dependencies holds every external dependency of the Service.
type Dependencies struct {
	Repo        *Repository
	Secrets     SecretStore
	NodeChecker NodeChecker // may be nil — falls back to private-IP check
	LogWriter   LogRecorder // may be nil — logs are silently dropped
	IDGen       func() string
	MasterKey   []byte // 32-byte key for ticket signing; if nil, cluster_secret is used
}

// ============================================================================
// Service
// ============================================================================

// Service is the core business-logic layer for serviceauth.
// It is transport-agnostic — HTTP handlers (in serviceauthd or Aegis)
// call its methods.
type Service struct {
	deps Dependencies

	clusterSecret []byte       // shared HMAC key, distributed to all services
	blVersion     atomic.Int64 // monotonic blocklist version
	catVersion    atomic.Int64 // monotonic catalog version
	mu            sync.RWMutex // guards clusterSecret lazy-init
}

// NewService creates a Service, loading or generating the cluster secret.
func NewService(deps Dependencies) (*Service, error) {
	if deps.Repo == nil {
		return nil, fmt.Errorf("serviceauth: Repo is required")
	}
	if deps.IDGen == nil {
		deps.IDGen = DefaultIDGen
	}
	if deps.Secrets == nil {
		return nil, fmt.Errorf("serviceauth: Secrets is required")
	}

	svc := &Service{deps: deps}

	// Load existing cluster secret or generate a new one.
	secret, err := deps.Secrets.Load()
	if err != nil || len(secret) == 0 {
		secret = make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, fmt.Errorf("generate cluster secret: %w", err)
		}
		if err := deps.Secrets.Save(secret); err != nil {
			return nil, fmt.Errorf("save cluster secret: %w", err)
		}
	}
	svc.clusterSecret = secret

	// Restore version counters from DB.
	if v, err := deps.Repo.GetBlocklistVersion(); err == nil {
		svc.blVersion.Store(v)
	}

	return svc, nil
}

// ClusterSecret returns a base64 copy of the shared HMAC key.
func (s *Service) ClusterSecretB64() string {
	return base64.StdEncoding.EncodeToString(s.clusterSecret)
}

// signingKey returns the key used for ticket signatures.
func (s *Service) signingKey() []byte {
	if len(s.deps.MasterKey) >= 32 {
		return s.deps.MasterKey
	}
	return s.clusterSecret
}

// ============================================================================
// Register — called by a service on startup
// ============================================================================

// Register processes a service registration request.
func (s *Service) Register(ctx context.Context, req RegisterRequest, clientIP string) (*RegisterResponse, error) {
	if err := validateRegisterRequest(req); err != nil {
		return nil, err
	}

	if !s.isInCluster(clientIP) {
		return nil, ErrNotInCluster
	}

	apisJSON, _ := json.Marshal(req.APIs)
	now := time.Now()
	rec := &ServiceRecord{
		ID:        s.deps.IDGen(),
		Name:      req.ServiceName,
		Host:      req.Host,
		Port:      req.Port,
		NodeHost:  req.NodeHost,
		APIsJSON:  string(apisJSON),
		PublicKey: req.PublicKey,
		Status:    "active",
		LastSeen:  now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.deps.Repo.UpsertService(rec); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}

	s.catVersion.Add(1)

	allActive, _ := s.deps.Repo.ListActive()
	instances := make([]ServiceInstance, 0, len(allActive))
	allAPIs := make([]APIDef, 0)
	for _, svc := range allActive {
		instances = append(instances, ServiceInstance{
			Name:     svc.Name,
			Host:     svc.Host,
			Port:     svc.Port,
			NodeHost: svc.NodeHost,
		})
		var apis []APIDef
		if json.Unmarshal([]byte(svc.APIsJSON), &apis) == nil {
			allAPIs = append(allAPIs, apis...)
		}
	}

	publicKeys, _ := s.deps.Repo.ListPublicKeys()

	blocklist, _ := s.deps.Repo.GetBlocklist()
	if blocklist == nil {
		blocklist = []BlocklistEntry{}
	}

	return &RegisterResponse{
		ServiceID:    rec.ID,
		Instances:    instances,
		PublicKeys:   publicKeys,
		APIs:         allAPIs,
		Blocklist:    blocklist,
		BlVersion:    s.blVersion.Load(),
		CatVersion:   s.catVersion.Load(),
		SyncInterval: 30,
	}, nil
}

// ============================================================================
// Sync — lightweight delta poll
// ============================================================================

// Sync returns changes since the client's last known versions.
func (s *Service) Sync(ctx context.Context, blVersion, catVersion int64) (*SyncResponse, error) {
	currentBL := s.blVersion.Load()
	currentCat := s.catVersion.Load()

	if blVersion >= currentBL && catVersion >= currentCat {
		return &SyncResponse{
			BlVersion:   currentBL,
			CatVersion:  currentCat,
			NotModified: true,
		}, nil
	}

	resp := &SyncResponse{
		BlVersion:  currentBL,
		CatVersion: currentCat,
	}

	if blVersion < currentBL {
		if entries, err := s.deps.Repo.GetBlocklistSince(blVersion); err == nil {
			resp.Blocklist = entries
		}
	}

	if catVersion < currentCat {
		allActive, err := s.deps.Repo.ListActive()
		if err == nil {
			for _, svc := range allActive {
				resp.NewInstances = append(resp.NewInstances, ServiceInstance{
					Name:     svc.Name,
					Host:     svc.Host,
					Port:     svc.Port,
					NodeHost: svc.NodeHost,
				})
			}
		}
		if pks, err := s.deps.Repo.ListPublicKeys(); err == nil {
			resp.PublicKeys = pks
		}
	}

	return resp, nil
}

// ============================================================================
// Report — asynchronous call logging
// ============================================================================

// Report records an inter-service call.
func (s *Service) Report(ctx context.Context, req ReportRequest) error {
	log := &CallLog{
		ID:            s.deps.IDGen(),
		CallerService: req.CallerService,
		TargetService: req.TargetService,
		TargetAPI:     req.TargetAPI,
		CallerHost:    req.CallerHost,
		TargetHost:    req.TargetHost,
		Allowed:       req.Allowed,
		LatencyMs:     req.LatencyMs,
		ErrorMsg:      req.ErrorMsg,
		CalledAt:      time.Now(),
	}

	if err := s.deps.Repo.InsertCallLog(log); err != nil {
		return fmt.Errorf("report: %w", err)
	}

	if s.deps.LogWriter != nil {
		_ = s.deps.LogWriter.WriteCallLog(
			req.CallerService, req.TargetService, req.TargetAPI,
			req.CallerHost, req.TargetHost, req.Allowed, req.LatencyMs, req.ErrorMsg,
		)
	}

	return nil
}

// ============================================================================
// Block / Unblock — admin actions
// ============================================================================

// BlockService blocks an entire service by its record ID.
func (s *Service) BlockService(ctx context.Context, id, reason string) error {
	rec, err := s.deps.Repo.FindByID(id)
	if err != nil || rec == nil {
		return ErrServiceNotFound
	}

	if err := s.deps.Repo.UpdateStatus(id, "blocked"); err != nil {
		return fmt.Errorf("block service: %w", err)
	}

	ver := s.blVersion.Add(1)
	entry := &BlocklistEntry{
		ID:        s.deps.IDGen(),
		ServiceID: rec.Name, // store service name so SDK's isBlocked can match by name
		APIName:   "*",
		Reason:    reason,
		Version:   ver,
	}
	if err := s.deps.Repo.AddBlock(entry); err != nil {
		return fmt.Errorf("block service: add entry: %w", err)
	}

	return nil
}

// BlockAPI blocks a specific API of a service.
func (s *Service) BlockAPI(ctx context.Context, serviceID, apiName, reason string) error {
	rec, err := s.deps.Repo.FindByID(serviceID)
	if err != nil || rec == nil {
		return ErrServiceNotFound
	}

	ver := s.blVersion.Add(1)
	entry := &BlocklistEntry{
		ID:        s.deps.IDGen(),
		ServiceID: rec.Name, // store service name so SDK's isBlocked can match
		APIName:   apiName,
		Reason:    reason,
		Version:   ver,
	}
	if err := s.deps.Repo.AddBlock(entry); err != nil {
		return fmt.Errorf("block api: %w", err)
	}

	return nil
}

// Unblock removes a blocklist entry.
func (s *Service) Unblock(ctx context.Context, blockID string) error {
	if err := s.deps.Repo.RemoveBlock(blockID); err != nil {
		return fmt.Errorf("unblock: %w", err)
	}
	s.blVersion.Add(1)
	s.catVersion.Add(1)
	return nil
}

// ============================================================================
// Queries — admin UI data
// ============================================================================

// ListServices returns all registered services.
func (s *Service) ListServices(ctx context.Context) ([]ServiceRecord, error) {
	return s.deps.Repo.ListAll()
}

// GetService returns a single service by ID.
func (s *Service) GetService(ctx context.Context, id string) (*ServiceRecord, error) {
	rec, err := s.deps.Repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, ErrServiceNotFound
	}
	return rec, nil
}

// GetTopology returns the service call topology.
func (s *Service) GetTopology(ctx context.Context, window time.Duration) (*TopologyData, error) {
	if window <= 0 {
		window = 1 * time.Hour
	}
	since := time.Now().Add(-window)

	edges, err := s.deps.Repo.TopologyEdges(since)
	if err != nil {
		return nil, err
	}

	allActive, _ := s.deps.Repo.ListActive()
	nodes := make([]TopologyNode, 0, len(allActive))
	for _, svc := range allActive {
		nodes = append(nodes, TopologyNode{
			Name:     svc.Name,
			Host:     svc.Host,
			Port:     svc.Port,
			NodeHost: svc.NodeHost,
			Status:   svc.Status,
		})
	}

	return &TopologyData{Nodes: nodes, Edges: edges}, nil
}

// GetCallLogs returns recent call records.
func (s *Service) GetCallLogs(ctx context.Context, since time.Time, limit int) ([]CallLog, error) {
	return s.deps.Repo.QueryCallLogs(since, limit)
}

// ============================================================================
// Cluster membership
// ============================================================================

// isInCluster returns true when clientIP is considered part of the trusted
// cluster. The check is layered:
//
//  1. localhost — always trusted
//  2. Private IP ranges (10.x, 172.16-31.x, 192.168.x) — trusted
//  3. NodeChecker — delegates to the injected implementation
//     (CIDR whitelist in standalone, node table in Aegis)
func (s *Service) isInCluster(clientIP string) bool {
	if clientIP == "" {
		return false
	}

	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}

	// Layer 1: localhost.
	if ip.IsLoopback() {
		return true
	}

	// Layer 2: private IPv4 ranges.
	if isPrivateIP(ip) {
		return true
	}

	// Layer 3: injected checker (CIDR whitelist or node table).
	if s.deps.NodeChecker != nil {
		if _, err := s.deps.NodeChecker.FindByIP(clientIP); err == nil {
			return true
		}
	}

	return false
}

// isPrivateIP returns true for RFC 1918 private IPv4 and RFC 4193 private IPv6.
func isPrivateIP(ip net.IP) bool {
	// IPv4 private ranges.
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}
	// IPv6 private ranges: fd00::/8 (unique local) and fe80::/10 (link-local).
	if ip.IsPrivate() {
		return true
	}
	return false
}

// ============================================================================
// Input validation
// ============================================================================

const (
	maxServiceNameLen = 128
	maxAPINameLen     = 128
	maxPathLen        = 512
)

var reservedChars = []byte{':', '\n', '\r', '\x00'}

// validateRegisterRequest checks that all fields in a registration request are
// well-formed. Rejects empty names, forbidden characters (colons break ticket
// parsing), and excessive lengths.
func validateRegisterRequest(req RegisterRequest) error {
	if req.ServiceName == "" {
		return fmt.Errorf("%w: service_name is required", ErrInvalidInput)
	}
	if len(req.ServiceName) > maxServiceNameLen {
		return fmt.Errorf("%w: service_name too long (max %d)", ErrInvalidInput, maxServiceNameLen)
	}
	if containsReserved(req.ServiceName) {
		return fmt.Errorf("%w: service_name contains reserved characters", ErrInvalidInput)
	}

	if req.Host == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidInput)
	}
	if req.Port <= 0 || req.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalidInput)
	}

	for i, api := range req.APIs {
		if api.Name == "" {
			return fmt.Errorf("%w: apis[%d].name is required", ErrInvalidInput, i)
		}
		if len(api.Name) > maxAPINameLen {
			return fmt.Errorf("%w: apis[%d].name too long (max %d)", ErrInvalidInput, i, maxAPINameLen)
		}
		if containsReserved(api.Name) {
			return fmt.Errorf("%w: apis[%d].name contains reserved characters", ErrInvalidInput, i)
		}
		if len(api.Path) > maxPathLen {
			return fmt.Errorf("%w: apis[%d].path too long (max %d)", ErrInvalidInput, i, maxPathLen)
		}
	}

	return nil
}

func containsReserved(s string) bool {
	for i := 0; i < len(s); i++ {
		for _, c := range reservedChars {
			if s[i] == c {
				return true
			}
		}
	}
	return false
}

// ============================================================================
// Bridge: Ticket → ActionContext (for Aegis integration)
// ============================================================================

// VerifyTicketAndGetSpace validates a service ticket using Ed25519 and the
// caller's public key from the repository. Returns the caller's service name.
func (s *Service) VerifyTicketAndGetSpace(ticketStr string) (serviceName string, err error) {
	// Quick decode to get caller name before full verification.
	claims, err := VerifyTicket(ticketStr, "") // won't pass; we just need caller name
	// Full verification with public key lookup.
	allKeys, keyErr := s.deps.Repo.ListPublicKeys()
	if keyErr != nil {
		return "", fmt.Errorf("verify ticket: lookup public keys: %w", keyErr)
	}
	// Decode again properly
	ticketDecoded, decodeErr := base64.StdEncoding.DecodeString(ticketStr)
	if decodeErr != nil {
		return "", fmt.Errorf("verify ticket: %w", ErrTicketInvalid)
	}
	parts := strings.SplitN(string(ticketDecoded), ":", 5)
	if len(parts) < 1 {
		return "", fmt.Errorf("verify ticket: %w", ErrTicketInvalid)
	}
	callerName := parts[0]

	pubKey, ok := allKeys[callerName]
	if !ok {
		return "", ErrServiceNotFound
	}

	claims, verifyErr := VerifyTicket(ticketStr, pubKey)
	if verifyErr != nil {
		return "", fmt.Errorf("verify ticket: %w", verifyErr)
	}

	instances, _ := s.deps.Repo.FindByName(callerName)
	for _, inst := range instances {
		if inst.Status == "blocked" {
			return "", ErrServiceBlocked
		}
	}
	_ = claims
	return callerName, nil
}

// LookupServiceByName returns the first active instance of a named service.
func (s *Service) LookupServiceByName(ctx context.Context, name string) (*ServiceRecord, error) {
	instances, err := s.deps.Repo.FindByName(name)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, ErrServiceNotFound
	}
	return &instances[0], nil
}
