package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"aegis/internal/aegisgateway"
	"aegis/internal/serviceauth"
)

func (h *Handlers) newAegisGateway() *aegisgateway.Gateway {
	opts := aegisgateway.Options{Registry: h.ServiceAuthSvc}
	if h.DistNode != nil {
		opts.SelfNodeID = h.DistNode.ID
		opts.Locator = distNodeServiceLocator{h: h}
		opts.RemoteCaller = distNodeServiceCaller{h: h}
	}
	return aegisgateway.New(opts)
}

type distNodeServiceCaller struct {
	h *Handlers
}

func (c distNodeServiceCaller) CallService(ctx context.Context, nodeID string, req aegisgateway.ServiceCallRequest, headers aegisgateway.ServiceCallHeaders) (*aegisgateway.HTTPResponse, error) {
	if c.h == nil || c.h.DistNode == nil {
		return nil, fmt.Errorf("distnode is not available")
	}
	if c.h.DistNode.Membership.GetPeer(nodeID) == nil {
		return nil, fmt.Errorf("target service on unknown/unreachable node: %s", nodeID)
	}
	callBody, _ := json.Marshal(req)
	proxyReq := ProxyRequest{
		Method: "POST",
		Path:   aegisgateway.ServiceCallPath,
		Body:   callBody,
		Headers: map[string]string{
			"X-Service-Ticket": headers.ServiceTicket,
			"X-Caller-Service": headers.CallerService,
		},
	}
	var resp ProxyResponse
	if err := c.h.DistNode.Transport.Call(ctx, nodeID, "Aegis.ProxyRequest", proxyReq, &resp); err != nil {
		return nil, fmt.Errorf("cross-node call to %s: %w", nodeID, err)
	}
	return &aegisgateway.HTTPResponse{
		StatusCode: resp.StatusCode,
		Headers:    resp.Headers,
		Body:       resp.Body,
	}, nil
}

type distNodeServiceLocator struct {
	h *Handlers
}

func (l distNodeServiceLocator) LocateServiceNode(ctx context.Context, name string) (string, error) {
	if l.h == nil || l.h.DistNode == nil {
		return "", nil
	}
	req := ProxyRequest{Method: "GET", Path: "/api/admin/v1/service-auth/services"}
	for _, pr := range l.h.fanOutToPeers(ctx, req) {
		if pr.Err != nil || pr.StatusCode != 200 || len(pr.Body) == 0 {
			continue
		}
		var listResp struct {
			Services []serviceauth.ServiceRecord `json:"services"`
		}
		if err := json.Unmarshal(pr.Body, &listResp); err != nil {
			continue
		}
		for _, s := range listResp.Services {
			if s.Name == name && s.Status == "active" && s.ListenPort > 0 {
				return pr.NodeID, nil
			}
		}
	}
	return "", nil
}
