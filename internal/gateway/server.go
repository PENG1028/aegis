package gateway

import (
	"fmt"
	"net/http"
	"time"

)

// Gateway is the local HTTP gateway runtime.
type Gateway struct {
	config  *Config
	server  *http.Server
	handler *Handler
	status  *GatewayStatus
}

// NewGateway creates a new local HTTP gateway.
func NewGateway(config *Config, resolver DomainResolver, secretProvider GatewayLinkSecretProvider) *Gateway {
	forwarder := NewLocalForwarder(config.RequestTimeoutSec)
	relayClient := NewRelayClient(secretProvider, config.RequestTimeoutSec)
	handler := NewHandler(resolver, forwarder, relayClient, config)

	return &Gateway{
		config:  config,
		handler: handler,
		status:  NewGatewayStatusFromConfig(config),
	}
}

// Start starts the local HTTP gateway.
func (g *Gateway) Start() error {
	if !g.config.Enabled {
		g.status.SetStatus("disabled", "gateway is disabled in config")
		return fmt.Errorf("local gateway is disabled")
	}

	// Wire internal handler with gateway status provider
	g.WireGatewayStatus()

	addr := fmt.Sprintf("%s:%d", g.config.ListenAddr(), g.config.ListenPort())
	g.server = &http.Server{
		Addr:         addr,
		Handler:      g.handler,
		ReadTimeout:  time.Duration(g.config.RequestTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(g.config.RequestTimeoutSec) * time.Second,
	}

	g.status.SetStatus("starting", fmt.Sprintf("binding to %s", addr))

	listener, err := listen(addr)
	if err != nil {
		g.status.SetStatus("failed", fmt.Sprintf("bind failed: %v", err))
		return fmt.Errorf("local gateway bind: %w", err)
	}

	g.status.SetStatus("online", "")
	go func() {
		if err := g.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			g.status.SetStatus("degraded", fmt.Sprintf("serve error: %v", err))
		}
	}()

	return nil
}

// Stop stops the local HTTP gateway.
func (g *Gateway) Stop() error {
	if g.server != nil {
		return g.server.Close()
	}
	return nil
}

// Status returns the current gateway status.
func (g *Gateway) Status() GatewayStatusInfo {
	return g.status.Get()
}

// SetRoutingTableStatusProvider sets the routing table status provider on the handler.
func (g *Gateway) SetRoutingTableStatusProvider(p RoutingTableStatusProvider) {
	g.handler.SetRoutingTableStatusProvider(p)
}

// WireGatewayStatus sets the gateway status tracker on the internal handler.
func (g *Gateway) WireGatewayStatus() {
	g.handler.SetGatewayStatus(g.status)
}
