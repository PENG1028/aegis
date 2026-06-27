import sys

with open('internal/localgateway/gateway_test.go', 'r', encoding='utf-8') as f:
    content = f.read()

new_tests = '''

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
	// 502 is expected (relay to 127.0.0.1:1 fails)
	if w.Code == http.StatusInternalServerError {
		t.Errorf("status = %d, headers caused internal error", w.Code)
	}
}
'''

content = content.rstrip() + '\n' + new_tests

with open('internal/localgateway/gateway_test.go', 'w', encoding='utf-8') as f:
    f.write(content)
print('Done')
