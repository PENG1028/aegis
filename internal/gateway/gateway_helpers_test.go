package gateway

import (
	"fmt"
	"net"
	"net/http/httptest"
	"strconv"
	"testing"
)

// ── Shared test mocks and helpers ───────────────────────────────────────────
// Extracted from the old gateway_test.go after removing tests that referenced
// deleted subsystems (noderuntime, old Repository/Service constructors).
// Used by gateway_v18c7_test.go.

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

func newTestHandler(t *testing.T, resolver DomainResolver, config *Config) *Handler {
	t.Helper()
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	secretProvider := &testSecretProvider{tokens: map[string]string{}}
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	return NewHandler(resolver, forwarder, relayClient, config)
}

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
