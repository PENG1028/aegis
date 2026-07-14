package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"aegis/internal/distnode"
)

// AdminDistNodeStatus handles GET /api/admin/v1/distnode/status
// Returns this node's cluster perspective: identity, role, peer list, liveness.
func (h *Handlers) AdminDistNodeStatus(w http.ResponseWriter, r *http.Request) {
	dn := h.DistNode
	if dn == nil {
		writeError(w, http.StatusNotImplemented, "distnode not enabled")
		return
	}

	allPeers := dn.Membership.AllPeers()
	alivePeers := dn.Membership.AlivePeers()

	// Enrich peer info with a ping test
	type peerStatus struct {
		ID     string `json:"id"`
		Addr   string `json:"addr"`
		Alive  bool   `json:"alive"`
		Since  string `json:"since,omitempty"`
	}

	peers := make([]peerStatus, 0, len(allPeers))
	for _, p := range allPeers {
		ps := peerStatus{ID: p.Info.ID, Addr: p.Info.Addr, Alive: p.Alive}
		if p.Alive && !p.AliveAt.IsZero() {
			ps.Since = p.AliveAt.Format(http.TimeFormat)
		}
		peers = append(peers, ps)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":     dn.ID,
		"name":        dn.Config.Name,
		"role":        dn.Role.Current(),
		"addr":        dn.Config.Addr,
		"peer_count":  len(allPeers),
		"alive_count": len(alivePeers),
		"peers":       peers,
		"enabled":     dn.Config.Secret != "",
	})
}

// AdminDistNodeCheck handles POST /api/admin/v1/distnode/check
// Runs a full cluster connectivity check: pings every peer, tries Transport.Call
// with an echo method, reports per-peer results.
func (h *Handlers) AdminDistNodeCheck(w http.ResponseWriter, r *http.Request) {
	dn := h.DistNode
	if dn == nil {
		writeError(w, http.StatusNotImplemented, "distnode not enabled")
		return
	}

	type checkResult struct {
		PeerID  string `json:"peer_id"`
		Addr    string `json:"addr"`
		Healthz string `json:"healthz"` // ok | timeout | error
		Echo    string `json:"echo"`    // ok | error_message
		Details string `json:"details,omitempty"`
	}

	results := make([]checkResult, 0)
	for _, p := range dn.Membership.AllPeers() {
		cr := checkResult{PeerID: p.Info.ID, Addr: p.Info.Addr}

		// 1. Healthz check
		healthzOK := false
		if p.Alive {
			cr.Healthz = "ok"
			healthzOK = true
		} else {
			cr.Healthz = "timeout"
		}

		// 2. Transport echo (round-trip auth + call)
		if healthzOK {
			var reply map[string]interface{}
			err := dn.Transport.Call(r.Context(), p.Info.ID, "DistNode.Ping", map[string]string{"from": dn.ID}, &reply)
			if err != nil {
				cr.Echo = "error"
				cr.Details = err.Error()
			} else {
				cr.Echo = "ok"
			}
		} else {
			cr.Echo = "skipped"
		}

		results = append(results, cr)
	}

	allOK := true
	for _, cr := range results {
		if cr.Healthz != "ok" || cr.Echo != "ok" {
			allOK = false
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_id":   dn.ID,
		"healthy":   allOK,
		"peer_count": len(results),
		"checks":    results,
	})
}

// AdminDistNodePingPeer handles POST /api/admin/v1/distnode/ping/{id}
// Pings a specific peer by node ID via transport echo.
func (h *Handlers) AdminDistNodePingPeer(w http.ResponseWriter, r *http.Request) {
	dn := h.DistNode
	if dn == nil {
		writeError(w, http.StatusNotImplemented, "distnode not enabled")
		return
	}

	targetID := r.PathValue("id")
	if targetID == "" {
		writeError(w, http.StatusBadRequest, "peer id required")
		return
	}

	peer := dn.Membership.GetPeer(targetID)
	if peer == nil {
		writeError(w, http.StatusNotFound, "peer not found")
		return
	}

	var reply map[string]interface{}
	err := dn.Transport.Call(r.Context(), targetID, "DistNode.Ping", map[string]string{"from": dn.ID}, &reply)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"peer_id": targetID,
			"alive":   false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peer_id": targetID,
		"alive":   true,
		"reply":   reply,
	})
}

// AdminDistNodeOverview handles GET /api/admin/v1/nodes/{id}/distnode-overview
// Returns the target node's system overview via distnode transport.
// This is the cross-node perspective primitive — it lets one panel
// see another node's data as if it were local.
func (h *Handlers) AdminDistNodeOverview(w http.ResponseWriter, r *http.Request) {
	if h.DistNode == nil {
		writeError(w, http.StatusNotImplemented, "distnode not enabled")
		return
	}
	targetID := r.PathValue("id")
	if targetID == "" {
		writeError(w, http.StatusBadRequest, "node id required")
		return
	}

	// If target is self, return local data directly
	if targetID == h.DistNode.ID {
		nodes, _ := h.NodeRepo.FindAll()
		routes, _ := h.Route.ListRoutes(r.Context())
		services, _ := h.Service.ListServices(r.Context())
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"node_id":       h.DistNode.ID,
			"node_count":    len(nodes),
			"route_count":   len(routes),
			"service_count": len(services),
			"source":        "local",
		})
		return
	}

	// Cross-node: call via transport
	var result map[string]interface{}
	err := h.DistNode.Transport.Call(r.Context(), targetID, "Aegis.SystemOverview", nil, &result)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"node_id":  targetID,
			"error":    err.Error(),
			"source":   "remote",
			"reachable": false,
		})
		return
	}
	result["source"] = "remote"
	result["reachable"] = true
	writeJSON(w, http.StatusOK, result)
}


// callLocal executes a request against the local mux and returns the response.
// Used by AdminDistNodeAggregate to collect local node data in the same format
// as remote proxy responses.
func (h *Handlers) callLocal(ctx context.Context, method, path string, body io.Reader) (json.RawMessage, int, error) {
	httpReq, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("X-Aegis-Proxy", "true") // bypass auth

	rw := &recordingWriter{code: 200}
	h.proxyMux.ServeHTTP(rw, httpReq)

	status := rw.code
	if status >= 400 {
		return nil, status, fmt.Errorf("local call returned %d", status)
	}

	return json.RawMessage(rw.body.Bytes()), status, nil
}

// PeerResult is one peer's response from a fan-out call.
type PeerResult struct {
	NodeID     string
	StatusCode int
	Body       json.RawMessage
	Err        error
}

// fanOutToPeers sends the same ProxyRequest to every alive peer concurrently via
// Aegis.ProxyRequest and collects each peer's response. This is the single peer
// fan-out primitive: AdminDistNodeAggregate and locateServiceNode both use it, so
// parallelism (and any future hop-guard / caching) lives in exactly one place.
// The local node is NOT included — callers that need it add local separately.
func (h *Handlers) fanOutToPeers(ctx context.Context, req ProxyRequest) []PeerResult {
	dn := h.DistNode
	if dn == nil {
		return nil
	}
	peers := dn.Membership.AlivePeers()
	results := make([]PeerResult, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func(idx int, nodeID string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					results[idx] = PeerResult{NodeID: nodeID, StatusCode: 500, Err: fmt.Errorf("panic: %v", rec)}
				}
			}()
			var resp ProxyResponse
			if err := dn.Transport.Call(ctx, nodeID, "Aegis.ProxyRequest", req, &resp); err != nil {
				results[idx] = PeerResult{NodeID: nodeID, StatusCode: 502, Err: err}
				return
			}
			results[idx] = PeerResult{NodeID: nodeID, StatusCode: resp.StatusCode, Body: resp.Body}
		}(i, p.Info.ID)
	}
	wg.Wait()
	return results
}

// AdminDistNodeAggregate handles GET /api/admin/v1/distnode/aggregate?path=/api/...
//
// Calls the same API path on every known node (local + all alive peers) and returns
// all results aggregated. This is the multi-node data primitive — it lets UI components
// fetch any API from all nodes at once without knowing about the cluster.
//
// Usage:
//
//	GET /api/admin/v1/distnode/aggregate?path=/api/admin/v1/routes
//	→ [{node_id:"node_a", status:200, body:[...]}, {node_id:"node_b", status:200, body:[...]}]
//
//	GET /api/admin/v1/distnode/aggregate?path=/api/system/status
//	→ [{node_id:"node_a", status:200, body:{version:"1.9A"}}, {node_id:"node_b", status:502, error:"..."}]
func (h *Handlers) AdminDistNodeAggregate(w http.ResponseWriter, r *http.Request) {
	dn := h.DistNode
	if dn == nil {
		writeError(w, http.StatusNotImplemented, "distnode not enabled")
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter required")
		return
	}

	type aggResult struct {
		NodeID string          `json:"node_id"`
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body,omitempty"`
		Error  string          `json:"error,omitempty"`
	}

	results := []aggResult{}

	// 1. Local execution
	localData, localStatus, localErr := h.callLocal(r.Context(), "GET", path, nil)
	if localErr != nil {
		results = append(results, aggResult{NodeID: dn.ID, Status: 500, Error: localErr.Error()})
	} else {
		results = append(results, aggResult{NodeID: dn.ID, Status: localStatus, Body: localData})
	}

	// 2. Remote execution on all alive peers (shared fan-out primitive)
	for _, pr := range h.fanOutToPeers(r.Context(), ProxyRequest{Method: "GET", Path: path}) {
		if pr.Err != nil {
			results = append(results, aggResult{NodeID: pr.NodeID, Status: pr.StatusCode, Error: pr.Err.Error()})
		} else {
			results = append(results, aggResult{NodeID: pr.NodeID, Status: pr.StatusCode, Body: pr.Body})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

// RegisterDistNodeHandlers registers the built-in distnode transport handlers.
// Called from main.go after creating the DistNode.
func RegisterDistNodeHandlers(dn *distnode.DistNode) {
	dn.Transport.Register("DistNode.Ping", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		return map[string]string{"pong": dn.ID, "role": dn.Role.Current()}, nil
	})
	dn.Transport.Register("DistNode.Info", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		return dn.Info(), nil
	})
	dn.Transport.Register("DistNode.Echo", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		return args, nil
	})
}

// RegisterAegisTransportHandlers registers Aegis-specific business handlers
// on the distnode transport, enabling cross-node method calls.
// Called from routes.go after creating the handlers.Handlers.
func RegisterAegisTransportHandlers(dn *distnode.DistNode, h *Handlers, mux *http.ServeMux) {
	if dn == nil || h == nil {
		return
	}
	dn.Transport.Register("Aegis.SystemOverview", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		data := h.getSystemOverview(ctx)
		data["node_id"] = dn.ID
		data["source"] = "remote"
		return data, nil
	})
	// NOTE: calls same h.Route.ListRoutes() as HTTP AdminListRoutes.
	// HTTP handler adds pagination; transport returns raw list.
	// Service call is shared — not a fork.
	dn.Transport.Register("Aegis.ListRoutes", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		routes, err := h.Route.ListRoutes(ctx)
		if err != nil {
			return nil, err
		}
		return routes, nil
	})
	// NOTE: calls same h.Service.ListServices() as HTTP AdminListServices.
	dn.Transport.Register("Aegis.ListServices", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		services, err := h.Service.ListServices(ctx)
		if err != nil {
			return nil, err
		}
		return services, nil
	})
	// Aegis.ProxyRequest is the generic cross-node API mechanism.
	// It executes any request against the local mux — no per-endpoint registration needed.
	h.proxyMux = mux
	dn.Transport.Register("Aegis.ProxyRequest", newProxyHandler(h))
	dn.Transport.Register("Aegis.NodeInfo", func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		nodes, _ := h.NodeRepo.FindAll()
		current, _ := h.NodeRepo.FindCurrent()
		return map[string]interface{}{
			"nodes":   nodes,
			"current": current,
		}, nil
	})
}
