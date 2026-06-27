package noderuntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const clientTimeout = 30 * time.Second

// Client communicates with the Aegis control plane.
type Client struct {
	baseURL    string
	nodeID     string
	nodeToken  string
	httpClient *http.Client
}

// HeartbeatResponse mirrors the control plane heartbeat API response.
type HeartbeatResponse struct {
	NodeID            string `json:"node_id"`
	Status            string `json:"status"`
	LatestRevision    int    `json:"latest_revision"`
	DesiredStateAvail bool   `json:"desired_state_available"`
	NodeIsOutdated    bool   `json:"node_is_outdated"`
}

// DesiredStateResponse mirrors the desired state API response.
type DesiredStateResponse struct {
	NodeID    string `json:"node_id"`
	Revision  int    `json:"revision"`
	StateHash string `json:"state_hash"`
	StateJSON string `json:"state_json,omitempty"`
	Status    string `json:"status"`
}

// ActualStateRequest mirrors the actual state API request.
type ActualStateRequest struct {
	NodeID            string `json:"node_id"`
	AppliedRevision   int    `json:"applied_revision"`
	StateHash         string `json:"state_hash"`
	Status            string `json:"status"`
	LastError         string `json:"last_error,omitempty"`
	ProviderStatus    string `json:"provider_status,omitempty"`
	GatewayStatus     string `json:"gateway_status,omitempty"`
	DiagnosticsStatus string `json:"diagnostics_status,omitempty"`
}

// NewClient creates a new control plane client.
func NewClient(baseURL, nodeID, nodeToken string) *Client {
	return &Client{
		baseURL:   baseURL,
		nodeID:    nodeID,
		nodeToken: nodeToken,
		httpClient: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

// SendHeartbeat sends a heartbeat to the control plane.
// gwInfo is optional local gateway status for inventory reporting.
func (c *Client) SendHeartbeat(status, agentVersion, hostname string, gwInfo *LocalGatewayInfo) (*HeartbeatResponse, error) {
	body := map[string]interface{}{
		"node_id":       c.nodeID,
		"status":        status,
		"agent_version": agentVersion,
		"hostname":      hostname,
	}

	// Include gateway status for heartbeat inventory upsert
	if gwInfo != nil {
		body["gateways"] = []map[string]interface{}{
			{
				"name":       gwInfo.Name,
				"type":       gwInfo.Type,
				"provider":   gwInfo.Provider,
				"bind_addr":  gwInfo.BindAddr,
				"host":       gwInfo.Host,
				"port":       gwInfo.Port,
				"scheme":     gwInfo.Scheme,
				"enabled":    gwInfo.Enabled,
				"status":     gwInfo.Status,
				"last_error": gwInfo.LastError,
			},
		}
		body["local_gateway_status"] = map[string]interface{}{
			"enabled":    gwInfo.Enabled,
			"bind_addr":  gwInfo.BindAddr,
			"port":       gwInfo.Port,
			"status":     gwInfo.Status,
			"last_error": gwInfo.LastError,
		}
	}
	resp, err := c.doPost("/api/node/v1/heartbeat", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeJSONResponse[HeartbeatResponse](resp, "/api/node/v1/heartbeat")
}

// PullDesiredState pulls the latest desired state from the control plane.
func (c *Client) PullDesiredState() (*DesiredStateResponse, error) {
	resp, err := c.doGet("/api/node/v1/desired-state")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return decodeJSONResponse[DesiredStateResponse](resp, "/api/node/v1/desired-state")
}

// ReportActualState reports the node's actual state to the control plane.
func (c *Client) ReportActualState(req ActualStateRequest) error {
	req.NodeID = c.nodeID
	resp, err := c.doPost("/api/node/v1/actual-state", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Read and discard body to ensure connection reuse
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{
			Path:       "/api/node/v1/actual-state",
			StatusCode: resp.StatusCode,
		}
	}
	return nil
}

// GetGatewayLinkToken fetches a decrypted GatewayLink token from the control plane.
// Returns the token for runtime injection into relay requests.
// Token is never cached to disk - only held in memory.
func (c *Client) GetGatewayLinkToken(gatewayLinkID string) (string, error) {
	resp, err := c.doGet("/api/node/v1/gateway-link-token/" + gatewayLinkID)
	if err != nil {
		return "", fmt.Errorf("fetch gateway link token: %w", err)
	}
	defer resp.Body.Close()
	result, err := decodeJSONResponse[GatewayLinkTokenResponse](resp, "/api/node/v1/gateway-link-token/"+gatewayLinkID)
	if err != nil {
		return "", err
	}
	return result.Token, nil
}

func (c *Client) doGet(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	c.setAuth(req)
	return c.httpClient.Do(req)
}

func (c *Client) doPost(path string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	return c.httpClient.Do(req)
}

func (c *Client) setAuth(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.nodeToken)
}

func decodeJSONResponse[T any](resp *http.Response, path string) (*T, error) {
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response %s: %w", path, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			Path:       path,
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	var result T
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decode response %s: %w", path, err)
	}
	return &result, nil
}

// APIError represents a non-2xx response from the control plane.
// GatewayLinkTokenResponse contains the decrypted GatewayLink secret.
type GatewayLinkTokenResponse struct {
	Token string `json:"token"`
}

type APIError struct {
	Path       string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %s: %d", e.Path, e.StatusCode)
}

// ErrorClassification returns a category for the error (no token leak).
func (e *APIError) ErrorClassification() string {
	switch {
	case e.StatusCode == 401:
		return "auth_failed"
	case e.StatusCode == 403:
		return "forbidden"
	case e.StatusCode >= 500:
		return "server_error"
	default:
		return "request_failed"
	}
}
