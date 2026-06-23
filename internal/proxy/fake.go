package proxy

import (
	"fmt"
)

// FakeProxyAdapter is a mock adapter for testing.
// Use it to test apply flow without a real Caddy installation.
type FakeProxyAdapter struct {
	ValidateShouldFail bool
	ReloadShouldFail   bool
	LastRendered       []byte
	LastValidated      string
	ReloadCallCount    int
}

// NewFakeAdapter creates a new fake proxy adapter.
func NewFakeAdapter() *FakeProxyAdapter {
	return &FakeProxyAdapter{}
}

// Name returns the adapter name.
func (a *FakeProxyAdapter) Name() string {
	return "fake"
}

// Render generates config output and stores it.
func (a *FakeProxyAdapter) Render(cfg GatewayConfig) ([]byte, error) {
	var output string
	for _, r := range cfg.Routes {
		if r.MaintenanceEnabled {
			msg := r.MaintenanceMessage
			if msg == "" {
				msg = "Service temporarily unavailable"
			}
			output += fmt.Sprintf("%s {\n    respond \"%s\" 503\n}\n", r.Domain, msg)
		} else {
			output += fmt.Sprintf("%s {\n    encode gzip\n    reverse_proxy %s\n}\n", r.Domain, r.UpstreamURL)
		}
	}
	a.LastRendered = []byte(output)
	return a.LastRendered, nil
}

// Validate simulates config validation.
func (a *FakeProxyAdapter) Validate(configPath string) error {
	a.LastValidated = configPath
	if a.ValidateShouldFail {
		return fmt.Errorf("fake validate failed")
	}
	return nil
}

// Reload simulates a proxy reload.
func (a *FakeProxyAdapter) Reload(command string) error {
	a.ReloadCallCount++
	if a.ReloadShouldFail {
		return fmt.Errorf("fake reload failed")
	}
	return nil
}
