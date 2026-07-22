// Package aegisgateway exposes Aegis capabilities to authenticated services.
package aegisgateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"aegis/internal/serviceauth"
)

const ServiceCallPath = "/api/service-auth/v1/call"

type ServiceRegistry interface {
	FindByName(ctx context.Context, name string) ([]serviceauth.ServiceRecord, error)
}

type NodeLocator interface {
	LocateServiceNode(ctx context.Context, name string) (string, error)
}

type RemoteCaller interface {
	CallService(ctx context.Context, nodeID string, req ServiceCallRequest, headers ServiceCallHeaders) (*HTTPResponse, error)
}

type ServiceCallRequest struct {
	Target  string            `json:"target"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Body    json.RawMessage   `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type ServiceCallHeaders struct {
	ServiceTicket string
	CallerService string
}

type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       json.RawMessage
}

type Gateway struct {
	registry     ServiceRegistry
	locator      NodeLocator
	remoteCaller RemoteCaller
	selfNodeID   string
	httpClient   *http.Client
}

type Options struct {
	Registry     ServiceRegistry
	Locator      NodeLocator
	RemoteCaller RemoteCaller
	SelfNodeID   string
	HTTPClient   *http.Client
}

func New(opts Options) *Gateway {
	g := &Gateway{
		registry:     opts.Registry,
		locator:      opts.Locator,
		remoteCaller: opts.RemoteCaller,
		selfNodeID:   opts.SelfNodeID,
		httpClient:   opts.HTTPClient,
	}
	if g.httpClient == nil {
		g.httpClient = http.DefaultClient
	}
	return g
}

func (g *Gateway) CallService(ctx context.Context, req ServiceCallRequest, headers ServiceCallHeaders) (*HTTPResponse, error) {
	if g == nil || g.registry == nil {
		return nil, statusError{code: http.StatusNotImplemented, message: "aegis gateway not available"}
	}
	if req.Target == "" || req.Method == "" || req.Path == "" {
		return nil, statusError{code: http.StatusBadRequest, message: "target, method, and path are required"}
	}

	target, err := g.findLocalTarget(ctx, req.Target)
	if err != nil {
		return nil, statusError{code: http.StatusInternalServerError, message: err.Error()}
	}

	targetNode := ""
	if target != nil {
		targetNode = target.NodeHost
	} else if g.locator != nil {
		targetNode, err = g.locator.LocateServiceNode(ctx, req.Target)
		if err != nil {
			return nil, statusError{code: http.StatusBadGateway, message: err.Error()}
		}
		if targetNode == "" {
			return nil, statusError{code: http.StatusNotFound, message: "target service not found in cluster: " + req.Target}
		}
	} else {
		return nil, statusError{code: http.StatusNotFound, message: "target service not found or has no listen_port: " + req.Target}
	}

	if g.remoteCaller != nil && targetNode != "" && targetNode != g.selfNodeID {
		return g.remoteCaller.CallService(ctx, targetNode, req, headers)
	}
	if target == nil {
		return nil, statusError{code: http.StatusNotFound, message: "target service not found or has no listen_port: " + req.Target}
	}
	return g.forwardLocal(ctx, target, req, headers)
}

func (g *Gateway) findLocalTarget(ctx context.Context, name string) (*serviceauth.ServiceRecord, error) {
	records, err := g.registry.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	for i := range records {
		if records[i].Status == "active" && records[i].ListenPort > 0 {
			return &records[i], nil
		}
	}
	return nil, nil
}

func (g *Gateway) forwardLocal(ctx context.Context, target *serviceauth.ServiceRecord, req ServiceCallRequest, headers ServiceCallHeaders) (*HTTPResponse, error) {
	forwardURL := fmt.Sprintf("http://%s:%d%s", target.Host, target.ListenPort, req.Path)
	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}
	outReq, err := http.NewRequestWithContext(ctx, req.Method, forwardURL, bodyReader)
	if err != nil {
		return nil, statusError{code: http.StatusInternalServerError, message: "build forward request: " + err.Error()}
	}
	outReq.Header.Set("Content-Type", "application/json")
	if headers.ServiceTicket != "" {
		outReq.Header.Set("X-Service-Ticket", headers.ServiceTicket)
	}
	if headers.CallerService != "" {
		outReq.Header.Set("X-Caller-Service", headers.CallerService)
	}

	resp, err := g.httpClient.Do(outReq)
	if err != nil {
		return nil, statusError{code: http.StatusBadGateway, message: "call " + req.Target + ": " + err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, statusError{code: http.StatusBadGateway, message: "read response from " + req.Target + ": " + err.Error()}
	}
	return &HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    flattenHeaders(resp.Header),
		Body:       json.RawMessage(body),
	}, nil
}

func flattenHeaders(headers http.Header) map[string]string {
	out := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			out[key] = values[0]
		}
	}
	return out
}

type statusError struct {
	code    int
	message string
}

func (e statusError) Error() string { return e.message }

func ErrorStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var se statusError
	if errors.As(err, &se) {
		return se.code
	}
	return http.StatusInternalServerError
}
