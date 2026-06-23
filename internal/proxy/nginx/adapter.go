package nginx

import (
	"aegis/internal/proxy"
	"errors"
)

// Adapter implements proxy.ProxyAdapter for Nginx.
// This is a stub for future implementation.
type Adapter struct{}

// NewAdapter creates a new Nginx adapter stub.
func NewAdapter() *Adapter {
	return &Adapter{}
}

// Name returns the adapter name.
func (a *Adapter) Name() string {
	return "nginx"
}

// Render is not implemented yet.
func (a *Adapter) Render(cfg proxy.GatewayConfig) ([]byte, error) {
	return nil, errors.New("nginx adapter is not implemented yet")
}

// Validate is not implemented yet.
func (a *Adapter) Validate(configPath string) error {
	return errors.New("nginx adapter is not implemented yet")
}

// Reload is not implemented yet.
func (a *Adapter) Reload(command string) error {
	return errors.New("nginx adapter is not implemented yet")
}
