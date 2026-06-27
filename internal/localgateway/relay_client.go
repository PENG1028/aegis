package localgateway

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GatewayLinkSecretProvider provides GatewayLink tokens for relay auth.
type GatewayLinkSecretProvider interface {
	GetGatewayLinkToken(gatewayLinkID string) (string, error)
}

// RelayClient executes managed relay requests to remote gateways.
type RelayClient struct {
	client         *http.Client
	secretProvider GatewayLinkSecretProvider
}

// NewRelayClient creates a new relay client.
func NewRelayClient(secretProvider GatewayLinkSecretProvider, timeoutSec int) *RelayClient {
	return &RelayClient{
		client: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		secretProvider: secretProvider,
	}
}

// RelayRequest represents a relay request to execute.
type RelayRequest struct {
	Method          string
	GatewayURL      string // e.g. "http://43.x.x.x:80/__aegis/relay"
	Path            string
	Body             io.Reader
	Headers         map[string]string
	RouteID         string
	GatewayLinkID   string
}

// Execute sends a relay request and returns the response.
func (c *RelayClient) Execute(req *RelayRequest) (*http.Response, error) {
	if req.GatewayLinkID != "" {
		token, err := c.secretProvider.GetGatewayLinkToken(req.GatewayLinkID)
		if err != nil {
			return nil, fmt.Errorf("gateway link secret unavailable: %w", err)
		}
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["X-Aegis-Gateway-ID"] = req.GatewayLinkID
		req.Headers["X-Aegis-Gateway-Token"] = token
	}

	// v1.8C-8A: Use fixed relay endpoint — always POST to /__aegis/relay.
	// Original path/query/method are carried via controlled headers.
	relayURL := req.GatewayURL

	// Split the original path into path and query for header transport.
	if req.Path != "" {
		pathPart := req.Path
		queryPart := ""
		if idx := strings.IndexByte(pathPart, '?'); idx >= 0 {
			queryPart = pathPart[idx+1:]
			pathPart = pathPart[:idx]
		}
		if req.Headers == nil {
			req.Headers = make(map[string]string)
		}
		req.Headers["X-Aegis-Original-Path"] = pathPart
		if queryPart != "" {
			req.Headers["X-Aegis-Original-Query"] = queryPart
		}
	}

	// Always POST to the relay endpoint; carry original method as a header.
	// The relay handler requires POST at the route level.
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	req.Headers["X-Aegis-Original-Method"] = req.Method

	outReq, err := http.NewRequest("POST", relayURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("create relay request: %w", err)
	}

	// Set headers
	for key, value := range req.Headers {
		outReq.Header.Set(key, value)
	}
	outReq.Header.Set("X-Aegis-Hop", "1")

	return c.client.Do(outReq)
}
