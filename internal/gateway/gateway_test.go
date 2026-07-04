package gateway

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockResolver struct {
	decisions map[string]*RoutingDecision
}

func (m *mockResolver) Resolve(domain string) *RoutingDecision {
	if d, ok := m.decisions[domain]; ok {
		return d
	}
	return &RoutingDecision{
		Domain:            domain,
		Status:            "unavailable",
		UnavailableReason: "not found",
	}
}

type testSecretProvider struct {
	tokens map[string]string
}

func (p *testSecretProvider) GetGatewayLinkToken(id string) (string, error) {
	if t, ok := p.tokens[id]; ok {
		return t, nil
	}
	return "", fmt.Errorf("secret not found for id: %s", id)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestHandler creates a Handler wired with real forwarder/relay client and
// a testSecretProvider that knows no tokens by default.
func newTestHandler(t *testing.T, resolver DomainResolver, config *Config) *Handler {
	t.Helper()
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	return NewHandler(resolver, forwarder, relayClient, config)
}

// getBackendHostPort extracts the host and numeric port from an httptest.Server.
func getBackendHostPort(t *testing.T, s *httptest.Server) (string, int) {
	t.Helper()
	addr := s.Listener.Addr().String()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("failed to parse backend address %s: %v", addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to convert port %s: %v", portStr, err)
	}
	return host, port
}

// mustParsePort extracts the port from an httptest.Server URL string.
func mustParsePort(t *testing.T, s *httptest.Server) int {
	t.Helper()
	_, portStr, err := net.SplitHostPort(s.Listener.Addr().String())
	if err != nil {
		t.Fatalf("failed to parse addr: %v", err)
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("failed to parse port: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// 1. TestDefaultConfig — defaults are sensible
// ---------------------------------------------------------------------------

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if !cfg.Enabled {
		t.Error("DefaultConfig().Enabled should be true")
	}
	if cfg.BindAddr != "127.0.0.1" {
		t.Errorf("DefaultConfig().BindAddr = %q, want %q", cfg.BindAddr, "127.0.0.1")
	}
	if cfg.Port != 18080 {
		t.Errorf("DefaultConfig().Port = %d, want %d", cfg.Port, 18080)
	}
	if cfg.UnmanagedMode != UnmanagedReject {
		t.Errorf("DefaultConfig().UnmanagedMode = %q, want %q", cfg.UnmanagedMode, UnmanagedReject)
	}
	if !cfg.PreserveHost {
		t.Error("DefaultConfig().PreserveHost should be true")
	}
	if cfg.RequestTimeoutSec != 30 {
		t.Errorf("DefaultConfig().RequestTimeoutSec = %d, want %d", cfg.RequestTimeoutSec, 30)
	}
}

// ---------------------------------------------------------------------------
// 2. TestListenAddr — ListenAddr/ListenPort return correct values
// ---------------------------------------------------------------------------

func TestListenAddr(t *testing.T) {
	cfg := &Config{
		BindAddr: "0.0.0.0",
		Port:     9090,
	}
	if addr := cfg.ListenAddr(); addr != "0.0.0.0" {
		t.Errorf("ListenAddr() = %q, want %q", addr, "0.0.0.0")
	}
	if port := cfg.ListenPort(); port != 9090 {
		t.Errorf("ListenPort() = %d, want %d", port, 9090)
	}
}

// ---------------------------------------------------------------------------
// 3. TestHandlerManagedDomain — managed domain resolved and processed
// ---------------------------------------------------------------------------

func TestHandlerManagedDomain(t *testing.T) {
	// Backend server that receives forwarded requests
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("managed ok"))
	}))
	defer backend.Close()

	host, port := getBackendHostPort(t, backend)

	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"app.example.com": {
				Domain:    "app.example.com",
				Status:    "available",
				RouteID:   "route-1",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: host,
				TargetLocalPort: port,
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "app.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "managed ok" {
		t.Errorf("body = %q, want %q", string(body), "managed ok")
	}
}

// ---------------------------------------------------------------------------
// 4. TestHandlerUnmanagedDomain — unmanaged domain returns 421
// ---------------------------------------------------------------------------

func TestHandlerUnmanagedDomain(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"unknown.example.com": {
				Domain:            "unknown.example.com",
				Status:            "unavailable",
				UnavailableReason: "not in routing table",
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "unknown.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMisdirectedRequest)
	}
}

// ---------------------------------------------------------------------------
// 5. TestHandlerMissingHost — missing Host returns 400
// ---------------------------------------------------------------------------

func TestHandlerMissingHost(t *testing.T) {
	handler := newTestHandler(t, &mockResolver{}, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = ""
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ---------------------------------------------------------------------------
// 6. TestHandlerDisabledRoute — disabled route returns 421
// ---------------------------------------------------------------------------

func TestHandlerDisabledRoute(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"disabled.example.com": {
				Domain: "disabled.example.com",
				Status: "disabled",
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "disabled.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMisdirectedRequest)
	}
}

// ---------------------------------------------------------------------------
// 7. TestHandlerUnavailableRoute — unavailable route returns 421
// ---------------------------------------------------------------------------

func TestHandlerUnavailableRoute(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"unavailable.example.com": {
				Domain:            "unavailable.example.com",
				Status:            "unavailable",
				UnavailableReason: "service down",
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "unavailable.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMisdirectedRequest)
	}
}

// ---------------------------------------------------------------------------
// 8. TestHandlerUnknownDomain — unknown domain returns 421
// ---------------------------------------------------------------------------

func TestHandlerUnknownDomain(t *testing.T) {
	// Mock resolver with no matching domain — falls through to default
	// "unavailable" decision.
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "nonexistent.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMisdirectedRequest)
	}
}

// ---------------------------------------------------------------------------
// 9. TestLocalDispatchPreservesMethod — method preserved in forward
// ---------------------------------------------------------------------------

func TestLocalDispatchPreservesMethod(t *testing.T) {
	// Backend records the HTTP method it receives
	receivedMethod := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod <- r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	host, port := getBackendHostPort(t, backend)

	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"api.example.com": {
				Domain:    "api.example.com",
				Status:    "available",
				RouteID:   "route-api",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: host,
				TargetLocalPort: port,
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL, bytes.NewReader([]byte(`{"key":"value"}`)))
	req.Host = "api.example.com"
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	if method := <-receivedMethod; method != "POST" {
		t.Errorf("forwarded method = %q, want %q", method, "POST")
	}
}

// ---------------------------------------------------------------------------
// 10. TestLocalDispatchTargetFromRoutingTable — target cannot be overridden
// ---------------------------------------------------------------------------

func TestLocalDispatchTargetFromRoutingTable(t *testing.T) {
	// Backend that would not be reached if forwarding used the request's Host.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("from routing table"))
	}))
	defer backend.Close()

	host, port := getBackendHostPort(t, backend)

	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"myapp.example.com": {
				Domain:    "myapp.example.com",
				Status:    "available",
				RouteID:   "route-myapp",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: host,
				TargetLocalPort: port,
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Request with a Host that does NOT match the backend address.
	// The forward must use the routing-table target, not the request Host.
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "myapp.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body = %s — target was not from routing table",
			resp.StatusCode, string(body))
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "from routing table" {
		t.Errorf("body = %q, want %q", string(body), "from routing table")
	}
}

// ---------------------------------------------------------------------------
// 11. TestLocalDispatchMissingTargetPort — missing port returns 500
// ---------------------------------------------------------------------------

func TestLocalDispatchMissingTargetPort(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"noport.example.com": {
				Domain:    "noport.example.com",
				Status:    "available",
				RouteID:   "route-noport",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: "127.0.0.1",
				TargetLocalPort: 0, // not configured
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/api", nil)
	r.Host = "noport.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// 12. TestRelayClientBuildsRequest — relay request uses /__aegis/relay
// ---------------------------------------------------------------------------

func TestRelayClientBuildsRequest(t *testing.T) {
	var receivedPath string
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	client := NewRelayClient(secretProvider, 30)

	req := &RelayRequest{
		Method:     "GET",
		GatewayURL: relayServer.URL + "/__aegis/relay",
		RouteID:    "route-1",
	}

	resp, err := client.Execute(req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resp.Body.Close()

	if !strings.Contains(receivedPath, "__aegis/relay") {
		t.Errorf("request path = %q, should contain %q", receivedPath, "__aegis/relay")
	}
}

// ---------------------------------------------------------------------------
// 13. TestRelayClientTokenInjected — GatewayLink token injected from secret
// ---------------------------------------------------------------------------

func TestRelayClientTokenInjected(t *testing.T) {
	var receivedToken string
	var receivedLinkID string
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("X-Aegis-Gateway-Token")
		receivedLinkID = r.Header.Get("X-Aegis-Gateway-ID")
		w.WriteHeader(http.StatusOK)
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{
		tokens: map[string]string{
			"link-1": "super-secret-token-value",
		},
	}
	client := NewRelayClient(secretProvider, 30)

	req := &RelayRequest{
		Method:       "POST",
		GatewayURL:   relayServer.URL + "/__aegis/relay",
		GatewayLinkID: "link-1",
		RouteID:      "route-1",
	}

	resp, err := client.Execute(req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	resp.Body.Close()

	if receivedToken != "super-secret-token-value" {
		t.Errorf("X-Aegis-Gateway-Token = %q, want %q", receivedToken, "super-secret-token-value")
	}
	if receivedLinkID != "link-1" {
		t.Errorf("X-Aegis-Gateway-ID = %q, want %q", receivedLinkID, "link-1")
	}
}

// ---------------------------------------------------------------------------
// 14. TestRelayClientMissingSecret — missing secret fails safely
// ---------------------------------------------------------------------------

func TestRelayClientMissingSecret(t *testing.T) {
	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	client := NewRelayClient(secretProvider, 30)

	req := &RelayRequest{
		Method:       "GET",
		GatewayURL:   "http://127.0.0.1:1/__aegis/relay",
		GatewayLinkID: "nonexistent-link",
		RouteID:      "route-1",
	}

	_, err := client.Execute(req)
	if err == nil {
		t.Fatal("expected error for missing secret, got nil")
	}
	if !strings.Contains(err.Error(), "secret") {
		t.Errorf("error message = %q, should contain 'secret'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// 15. TestRelayClientReturnsResponse — response returned to caller
// ---------------------------------------------------------------------------

func TestRelayClientReturnsResponse(t *testing.T) {
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "custom-val")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"created"}`))
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	client := NewRelayClient(secretProvider, 30)

	req := &RelayRequest{
		Method:     "PUT",
		GatewayURL: relayServer.URL + "/__aegis/relay",
		RouteID:    "route-1",
	}

	resp, err := client.Execute(req)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if cv := resp.Header.Get("X-Custom"); cv != "custom-val" {
		t.Errorf("X-Custom = %q, want %q", cv, "custom-val")
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"status":"created"}` {
		t.Errorf("body = %q, want %q", string(body), `{"status":"created"}`)
	}
}

// ---------------------------------------------------------------------------
// 16. TestNoDirectRemoteTarget — no direct_remote_target ever executed
// ---------------------------------------------------------------------------

func TestNoDirectRemoteTarget(t *testing.T) {
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"direct.example.com": {
				Domain:  "direct.example.com",
				Status:  "available",
				RouteID: "route-direct",
				SelectedCandidate: &CandidateEntry{
					Mode: "direct_remote_target",
				},
				TargetLocalHost: "127.0.0.1",
				TargetLocalPort: 9999,
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "direct.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotImplemented)
	}
}

// ---------------------------------------------------------------------------
// 17. TestUnmanagedDomainNotProxied — unmanaged domain not forwarded
// ---------------------------------------------------------------------------

func TestUnmanagedDomainNotProxied(t *testing.T) {
	// A backend that would be reached only if the handler erroneously forwarded.
	backendCalled := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	// Empty mock — every domain resolves to "unavailable".
	handler := newTestHandler(t, &mockResolver{}, DefaultConfig())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/", nil)
	r.Host = "unmanaged.example.com"
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusMisdirectedRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusMisdirectedRequest)
	}
	if backendCalled {
		t.Error("backend was called — handler forwarded an unmanaged domain")
	}
}

// ---------------------------------------------------------------------------
// 18. TestHandlerNoTokenLeak — error responses don't contain raw tokens
// ---------------------------------------------------------------------------

func TestHandlerNoTokenLeak(t *testing.T) {
	token := "super-secret-gateway-link-token-do-not-leak"

	// Relay server that echoes headers back.
	var sentToken string
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sentToken = r.Header.Get("X-Aegis-Gateway-Token")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{
		tokens: map[string]string{"link-1": token},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"relay.example.com": {
				Domain:    "relay.example.com",
				Status:    "available",
				RouteID:   "route-relay",
				SelectedCandidate: &CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    relayServer.URL,
					GatewayLinkID: "link-1",
				},
			},
		},
	}

	config := DefaultConfig()
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	// Full server round-trip so the relay request is actually sent.
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "relay.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// 1. Token must be present in the relay request header (not leaked elsewhere).
	if sentToken != token {
		t.Errorf("X-Aegis-Gateway-Token header = %q, want %q", sentToken, token)
	}

	// 2. Response body must NOT contain the raw token.
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), token) {
		t.Errorf("response body contains raw token: %q", string(body))
	}
}

// ---------------------------------------------------------------------------
// 19. TestSelfLoopPrevention — self-loop candidate rejected (check hop = 1)
// ---------------------------------------------------------------------------

func TestSelfLoopPrevention(t *testing.T) {
	// Relay server that simulates self-loop detection (returns 403)
	// and records the X-Aegis-Hop header.
	var hopValue string
	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hopValue = r.Header.Get("X-Aegis-Hop")
		http.Error(w, "self-loop detected", http.StatusForbidden)
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{
		tokens: map[string]string{"link-loop": "loop-token"},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"loop.example.com": {
				Domain:    "loop.example.com",
				Status:    "available",
				RouteID:   "route-loop",
				SelectedCandidate: &CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    relayServer.URL,
					GatewayLinkID: "link-loop",
				},
			},
		},
	}

	config := DefaultConfig()
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "loop.example.com"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// The relay request must carry X-Aegis-Hop: 1
	if hopValue != "1" {
		t.Errorf("X-Aegis-Hop = %q, want %q", hopValue, "1")
	}

	// Remote gateway returning 403 → handler maps to 502 Bad Gateway
	if resp.StatusCode != http.StatusBadGateway {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("status = %d, want 502, body = %q", resp.StatusCode, string(body))
	}
}

// ---------------------------------------------------------------------------
// 20. TestGatewayStatusOnline — status online when bound
// ---------------------------------------------------------------------------

func TestGatewayStatusOnline(t *testing.T) {
	config := DefaultConfig()
	config.Port = 0 // OS-assigned port

	resolver := &mockResolver{decisions: map[string]*RoutingDecision{}}
	secretProvider := &testSecretProvider{tokens: map[string]string{}}

	gateway := NewGateway(config, resolver, secretProvider)
	err := gateway.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	defer gateway.Stop()

	status := gateway.Status()
	if status.Status != "online" {
		t.Errorf("status = %q, want %q", status.Status, "online")
	}
}

// ---------------------------------------------------------------------------
// 21. TestGatewayStatusDegraded — status degraded on bind failure
// ---------------------------------------------------------------------------

func TestGatewayStatusDegraded(t *testing.T) {
	config := DefaultConfig()
	config.Port = 99999 // invalid port (>65535), guaranteed bind failure

	resolver := &mockResolver{decisions: map[string]*RoutingDecision{}}
	secretProvider := &testSecretProvider{tokens: map[string]string{}}

	gateway := NewGateway(config, resolver, secretProvider)
	err := gateway.Start()
	if err == nil {
		t.Fatal("Start() should have failed with invalid port")
	}

	status := gateway.Status()
	if status.Status != "failed" {
		t.Errorf("status = %q, want %q", status.Status, "failed")
	}
	if status.LastError == "" {
		t.Error("status.LastError should not be empty after bind failure")
	}
}

// 22. TestRelayRequestWithCorrectHeaders — full round-trip validates headers
// ---------------------------------------------------------------------------

func TestRelayRequestWithCorrectHeaders(t *testing.T) {
    var (
        receivedRouteID      string
        receivedGatewayID    string
        receivedGatewayToken string
        receivedSourceNode   string
        receivedHop          string
    )
    relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        receivedRouteID = r.Header.Get("X-Aegis-Route-ID")
        receivedGatewayID = r.Header.Get("X-Aegis-Gateway-ID")
        receivedGatewayToken = r.Header.Get("X-Aegis-Gateway-Token")
        receivedSourceNode = r.Header.Get("X-Aegis-Source-Node")
        receivedHop = r.Header.Get("X-Aegis-Hop")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("relay ok"))
    }))
    defer relayServer.Close()

    secretProvider := &testSecretProvider{
        tokens: map[string]string{"link-1": "test-token"},
    }
    resolver := &mockResolver{
        decisions: map[string]*RoutingDecision{
            "app.example.com": {
                Domain:  "app.example.com",
                Status:  "available",
                RouteID: "route-1",
                SelectedCandidate: &CandidateEntry{
                    Mode:          "private_gateway",
                    GatewayURL:    relayServer.URL,
                    GatewayLinkID: "link-1",
                },
            },
        },
    }

    config := DefaultConfig()
    config.NodeID = "test-node-a"
    forwarder := NewLocalForwarder(config.RequestTimeoutSec)
    relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
    handler := NewHandler(resolver, forwarder, relayClient, config)

    ts := httptest.NewServer(handler)
    defer ts.Close()

    req, _ := http.NewRequest("GET", ts.URL, nil)
    req.Host = "app.example.com"
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(body))
    }

    if receivedRouteID != "route-1" {
        t.Errorf("X-Aegis-Route-ID = %q, want %q", receivedRouteID, "route-1")
    }
    if receivedGatewayID != "link-1" {
        t.Errorf("X-Aegis-Gateway-ID = %q, want %q", receivedGatewayID, "link-1")
    }
    if receivedGatewayToken != "test-token" {
        t.Errorf("X-Aegis-Gateway-Token = %q, want %q", receivedGatewayToken, "test-token")
    }
    if receivedSourceNode != "test-node-a" {
        t.Errorf("X-Aegis-Source-Node = %q, want %q", receivedSourceNode, "test-node-a")
    }
    if receivedHop != "1" {
        t.Errorf("X-Aegis-Hop = %q, want %q", receivedHop, "1")
    }
}

// ---------------------------------------------------------------------------
// 23. TestRelayWithWrongToken — 403 from relay mapped to 502
// ---------------------------------------------------------------------------

func TestRelayWithWrongToken(t *testing.T) {
    relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        http.Error(w, "invalid token", http.StatusForbidden)
    }))
    defer relayServer.Close()

    secretProvider := &testSecretProvider{
        tokens: map[string]string{"link-bad": "bad-token"},
    }
    resolver := &mockResolver{
        decisions: map[string]*RoutingDecision{
            "badrelay.example.com": {
                Domain:  "badrelay.example.com",
                Status:  "available",
                RouteID: "route-bad",
                SelectedCandidate: &CandidateEntry{
                    Mode:          "private_gateway",
                    GatewayURL:    relayServer.URL,
                    GatewayLinkID: "link-bad",
                },
            },
        },
    }

    config := DefaultConfig()
    forwarder := NewLocalForwarder(config.RequestTimeoutSec)
    relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
    handler := NewHandler(resolver, forwarder, relayClient, config)

    ts := httptest.NewServer(handler)
    defer ts.Close()

    req, _ := http.NewRequest("GET", ts.URL, nil)
    req.Host = "badrelay.example.com"
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusBadGateway {
        body, _ := io.ReadAll(resp.Body)
        t.Errorf("status = %d, want 502, body = %q", resp.StatusCode, string(body))
    }
}

// ---------------------------------------------------------------------------
// 24. TestLocalGatewayHeartbeatStatus — GatewayStatusProvider interface
// ---------------------------------------------------------------------------

func TestLocalGatewayHeartbeatStatus(t *testing.T) {
    config := DefaultConfig()
    config.Port = 0 // OS-assigned

    resolver := &mockResolver{decisions: map[string]*RoutingDecision{}}
    secretProvider := &testSecretProvider{tokens: map[string]string{}}

    gateway := NewGateway(config, resolver, secretProvider)
    err := gateway.Start()
    if err != nil {
        t.Fatalf("Start() failed: %v", err)
    }
    defer gateway.Stop()

    // LocalGatewayStatuses() returns []*noderuntime.LocalGatewayInfo with Name/Type/Provider.
    infos := gateway.LocalGatewayStatuses()
    if len(infos) != 1 {
        t.Fatalf("LocalGatewayStatuses() returned %d infos, want 1", len(infos))
    }
    info := infos[0]
    // Name/Type/Provider are hard-coded constants from LocalGatewayStatuses().
    if info.Name != "local-gateway" {
        t.Errorf("Name = %q, want %q", info.Name, "local-gateway")
    }
    if info.Type != "local" {
        t.Errorf("Type = %q, want %q", info.Type, "local")
    }
    if info.Provider != "aegis" {
        t.Errorf("Provider = %q, want %q", info.Provider, "aegis")
    }
    // Status should be "online" after Start() succeeds.
    // ("starting" is accepted as a transient state.)
    if info.Status != "starting" && info.Status != "online" {
        t.Errorf("Status = %q, want 'starting' or 'online'", info.Status)
    }
}

// ---------------------------------------------------------------------------
// 25. TestConfigNodeID — NodeID passed through to relay headers
// ---------------------------------------------------------------------------

func TestConfigNodeID(t *testing.T) {
    var receivedSourceNode string
    relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        receivedSourceNode = r.Header.Get("X-Aegis-Source-Node")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("ok"))
    }))
    defer relayServer.Close()

    secretProvider := &testSecretProvider{
        tokens: map[string]string{"link-node": "node-token"},
    }
    resolver := &mockResolver{
        decisions: map[string]*RoutingDecision{
            "node.example.com": {
                Domain:  "node.example.com",
                Status:  "available",
                RouteID: "route-node",
                SelectedCandidate: &CandidateEntry{
                    Mode:          "private_gateway",
                    GatewayURL:    relayServer.URL,
                    GatewayLinkID: "link-node",
                },
            },
        },
    }

    config := DefaultConfig()
    config.NodeID = "my-node"
    forwarder := NewLocalForwarder(config.RequestTimeoutSec)
    relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
    handler := NewHandler(resolver, forwarder, relayClient, config)

    ts := httptest.NewServer(handler)
    defer ts.Close()

    req, _ := http.NewRequest("GET", ts.URL, nil)
    req.Host = "node.example.com"
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        t.Fatalf("request failed: %v", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(body))
    }

    if receivedSourceNode != "my-node" {
        t.Errorf("X-Aegis-Source-Node = %q, want %q", receivedSourceNode, "my-node")
    }
}

// ---------------------------------------------------------------------------
// 26. TestRelayClientHeaderNamesAreCorrect — direct relay client header check
// ---------------------------------------------------------------------------

func TestRelayClientHeaderNamesAreCorrect(t *testing.T) {
    var (
        hGatewayID    string
        hGatewayToken string
        hHop          string
    )
    relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        hGatewayID = r.Header.Get("X-Aegis-Gateway-ID")
        hGatewayToken = r.Header.Get("X-Aegis-Gateway-Token")
        hHop = r.Header.Get("X-Aegis-Hop")
        w.WriteHeader(http.StatusOK)
    }))
    defer relayServer.Close()

    secretProvider := &testSecretProvider{
        tokens: map[string]string{"link-header": "header-token"},
    }
    client := NewRelayClient(secretProvider, 30)

    req := &RelayRequest{
        Method:       "GET",
        GatewayURL:   relayServer.URL + "/__aegis/relay",
        GatewayLinkID: "link-header",
        RouteID:      "route-header",
    }

    resp, err := client.Execute(req)
    if err != nil {
        t.Fatalf("Execute failed: %v", err)
    }
    resp.Body.Close()

    if hGatewayID != "link-header" {
        t.Errorf("X-Aegis-Gateway-ID = %q, want %q", hGatewayID, "link-header")
    }
    if hGatewayToken != "header-token" {
        t.Errorf("X-Aegis-Gateway-Token = %q, want %q", hGatewayToken, "header-token")
    }
    if hHop != "1" {
        t.Errorf("X-Aegis-Hop = %q, want %q", hHop, "1")
    }
}

// ---------------------------------------------------------------------------
// 27. TestUnmanagedDomainPassthroughDeferred — no open proxy
// ---------------------------------------------------------------------------

func TestUnmanagedDomainPassthroughDeferred(t *testing.T) {
    // A backend that should never be reached.
    backendCalled := false
    backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        backendCalled = true
        w.WriteHeader(http.StatusOK)
    }))
    defer backend.Close()

    config := DefaultConfig()
    config.UnmanagedMode = UnmanagedPassthroughDefer

    resolver := &mockResolver{}
    handler := newTestHandler(t, resolver, config)
    w := httptest.NewRecorder()
    r := httptest.NewRequest("GET", "/", nil)
    r.Host = "unmanaged-deferred.example.com"
    handler.ServeHTTP(w, r)

    if w.Code != http.StatusMisdirectedRequest {
        t.Errorf("status = %d, want %d", w.Code, http.StatusMisdirectedRequest)
    }
    if backendCalled {
        t.Error("backend was called — passthrough_deferred mode created an open proxy")
    }
}


// ---------------------------------------------------------------------------
// 28-31: Header injection hardening
// ---------------------------------------------------------------------------

func TestExternalAegisHeadersStripped(t *testing.T) {
	// External request carries spoofed X-Aegis headers.
	// The gateway must strip them before processing.
	var receivedGatewayID string
	var receivedGatewayToken string
	var receivedHop string

	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedGatewayID = r.Header.Get("X-Aegis-Gateway-ID")
		receivedGatewayToken = r.Header.Get("X-Aegis-Gateway-Token")
		receivedHop = r.Header.Get("X-Aegis-Hop")
		w.WriteHeader(http.StatusOK)
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{
		tokens: map[string]string{"link-1": "real-token"},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"app.example.com": {
				Domain:  "app.example.com",
				Status:  "available",
				RouteID: "route-1",
				SelectedCandidate: &CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    relayServer.URL,
					GatewayLinkID: "link-1",
				},
			},
		},
	}

	config := DefaultConfig()
	config.NodeID = "test-node"
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// External request with spoofed headers
	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "app.example.com"
	req.Header.Set("X-Aegis-Gateway-ID", "evil-link")
	req.Header.Set("X-Aegis-Gateway-Token", "evil-token")
	req.Header.Set("X-Aegis-Hop", "99")
	req.Header.Set("X-Aegis-Source-Node", "evil-node")
	req.Header.Set("X-Aegis-Route-ID", "evil-route")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(body))
	}

	// The relay server must receive the TRUSTED headers, not the evil ones
	if receivedGatewayID == "evil-link" {
		t.Error("X-Aegis-Gateway-ID was not stripped - evil value reached relay")
	}
	if receivedGatewayID != "link-1" {
		t.Errorf("X-Aegis-Gateway-ID = %q, want %q (trusted value)", receivedGatewayID, "link-1")
	}
	if receivedGatewayToken == "evil-token" {
		t.Error("X-Aegis-Gateway-Token was not stripped - evil value reached relay")
	}
	if receivedGatewayToken != "real-token" {
		t.Errorf("X-Aegis-Gateway-Token = %q, want %q (real token from provider)", receivedGatewayToken, "real-token")
	}
	if receivedHop == "99" {
		t.Error("X-Aegis-Hop was not stripped - evil hop 99 reached relay")
	}
	if receivedHop != "1" {
		t.Errorf("X-Aegis-Hop = %q, want %q", receivedHop, "1")
	}
}

func TestExternalAegisHeadersDontReachLocalTarget(t *testing.T) {
	// External request with X-Aegis-* headers.
	// When dispatched locally, these headers must NOT reach the target.
	var receivedGatewayToken string
	var receivedTargetHost string
	var receivedTargetPort string

	targetSvc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedGatewayToken = r.Header.Get("X-Aegis-Gateway-Token")
		receivedTargetHost = r.Header.Get("X-Aegis-Target-Host")
		receivedTargetPort = r.Header.Get("X-Aegis-Target-Port")
		w.WriteHeader(http.StatusOK)
	}))
	defer targetSvc.Close()

	host, port := getBackendHostPort(t, targetSvc)

	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"local.example.com": {
				Domain:  "local.example.com",
				Status:  "available",
				RouteID: "route-local",
				SelectedCandidate: &CandidateEntry{
					Mode: "local_gateway",
				},
				TargetLocalHost: host,
				TargetLocalPort: port,
			},
		},
	}

	handler := newTestHandler(t, resolver, DefaultConfig())
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "local.example.com"
	req.Header.Set("X-Aegis-Gateway-Token", "should-not-reach-target")
	req.Header.Set("X-Aegis-Target-Host", "1.2.3.4")
	req.Header.Set("X-Aegis-Target-Port", "9999")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(body))
	}

	if receivedGatewayToken != "" {
		t.Errorf("X-Aegis-Gateway-Token reached target: %q", receivedGatewayToken)
	}
	if receivedTargetHost != "" {
		t.Errorf("X-Aegis-Target-Host reached target: %q", receivedTargetHost)
	}
	if receivedTargetPort != "" {
		t.Errorf("X-Aegis-Target-Port reached target: %q", receivedTargetPort)
	}
}

func TestExternalHostHeaderNotUsedAsRelaySource(t *testing.T) {
	// The X-Aegis-Source-Node must come from Config.NodeID.
	var receivedSourceNode string

	relayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSourceNode = r.Header.Get("X-Aegis-Source-Node")
		w.WriteHeader(http.StatusOK)
	}))
	defer relayServer.Close()

	secretProvider := &testSecretProvider{
		tokens: map[string]string{"link-1": "token"},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"relay.example.com": {
				Domain:  "relay.example.com",
				Status:  "available",
				RouteID: "route-1",
				SelectedCandidate: &CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    relayServer.URL,
					GatewayLinkID: "link-1",
				},
			},
		},
	}

	config := DefaultConfig()
	config.NodeID = "my-official-node-id"
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL, nil)
	req.Host = "relay.example.com"
	req.Header.Set("X-Aegis-Source-Node", "spoofed-node")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if receivedSourceNode != "my-official-node-id" {
		t.Errorf("X-Aegis-Source-Node = %q, want %q",
			receivedSourceNode, "my-official-node-id")
	}
}

func TestSpoofedHeadersDontCausePanic(t *testing.T) {
	// All possible spoofed X-Aegis-* headers must not cause errors/panics.
	secretProvider := &testSecretProvider{
		tokens: map[string]string{"link-1": "token"},
	}
	resolver := &mockResolver{
		decisions: map[string]*RoutingDecision{
			"test.example.com": {
				Domain:  "test.example.com",
				Status:  "available",
				RouteID: "route-1",
				SelectedCandidate: &CandidateEntry{
					Mode:          "private_gateway",
					GatewayURL:    "http://127.0.0.1:1",
					GatewayLinkID: "link-1",
				},
			},
		},
	}

	config := DefaultConfig()
	config.NodeID = "node"
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/health", nil)
	r.Host = "test.example.com"
	// Spoof all possible internal headers
	r.Header.Set("X-Aegis-Gateway-ID", "evil")
	r.Header.Set("X-Aegis-Gateway-Token", "evil-token")
	r.Header.Set("X-Aegis-Source-Node", "evil")
	r.Header.Set("X-Aegis-Hop", "99")
	r.Header.Set("X-Aegis-Route-ID", "evil-route")
	r.Header.Set("X-Aegis-Request-ID", "evil-req")
	r.Header.Set("X-Aegis-From-Node", "evil")

	handler.ServeHTTP(w, r)

	// Must not return 500 due to header processing errors.
	// 502/500 is expected (relay to 127.0.0.1:1 fails with connection error)
	if w.Code == 500 {
		body := w.Body.String()
		if !strings.Contains(body, "relay execution failed") && !strings.Contains(body, "unavailable") {
			t.Errorf("status = %d with unexpected body: %s", w.Code, body)
		}
	}
}
