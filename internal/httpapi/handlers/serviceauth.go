package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aegis/internal/action"
	"aegis/internal/serviceauth"
)

// clientIP extracts the real client IP from a request.
// Only trusts X-Forwarded-For when the direct TCP peer is localhost (Caddy reverse proxy).
// Otherwise uses the unspoofable RemoteAddr to prevent isInCluster bypass.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	// Only trust X-Forwarded-For from the local reverse proxy.
	if host == "127.0.0.1" || host == "::1" {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			parts := strings.Split(fwd, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	return host
}

// ============================================================================
// Service-side API (called by SDK, no admin auth required)
// ============================================================================

// ServiceAuthRegister handles POST /api/service-auth/v1/register
func (h *Handlers) ServiceAuthRegister(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var req serviceauth.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if req.ServiceName == "" || req.PublicKey == "" {
		writeError(w, http.StatusBadRequest, "service_name and public_key are required")
		return
	}

	resp, err := h.ServiceAuthSvc.Register(r.Context(), req, clientIP(r))
	if err != nil {
		if errors.Is(err, serviceauth.ErrNotInCluster) {
			writeError(w, http.StatusForbidden, "not in cluster")
			return
		}
		if errors.Is(err, serviceauth.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// ServiceAuthSync handles GET /api/service-auth/v1/sync
func (h *Handlers) ServiceAuthSync(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	blVer, _ := strconv.ParseInt(r.URL.Query().Get("bl_version"), 10, 64)
	catVer, _ := strconv.ParseInt(r.URL.Query().Get("cat_version"), 10, 64)

	resp, err := h.ServiceAuthSvc.Sync(r.Context(), blVer, catVer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if resp.NotModified {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// ServiceAuthReport handles POST /api/service-auth/v1/report
func (h *Handlers) ServiceAuthReport(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var req serviceauth.ReportRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.ServiceAuthSvc.Report(r.Context(), req); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
// ServiceAuthHeartbeat handles POST /api/service-auth/v1/heartbeat
func (h *Handlers) ServiceAuthHeartbeat(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}
	var body struct {
		Name       string `json:"name"`
		InstanceID string `json:"instance_id"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.ServiceAuthSvc.Heartbeat(r.Context(), body.Name, body.InstanceID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

// ServiceAuthScopedServices handles GET /api/service-auth/v1/services
// Returns per-service scoped view (callers + deps), identified via X-Service-Ticket.
func (h *Handlers) ServiceAuthScopedServices(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}
	ac := action.GetActionContext(r.Context())
	if ac == nil || !ac.IsService() {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	serviceName := ac.SpaceID
	if serviceName == "" {
		writeError(w, http.StatusBadRequest, "unknown caller")
		return
	}
	window := 1 * time.Hour
	if w := r.URL.Query().Get("window"); w != "" {
		if d, err := time.ParseDuration(w); err == nil {
			window = d
		}
	}
	callers, _ := h.ServiceAuthSvc.CallersOf(r.Context(), serviceName, window)
	deps, _ := h.ServiceAuthSvc.DepsOf(r.Context(), serviceName, window)
	type relEntry struct {
		Service  string `json:"service"`
		API      string `json:"api"`
		Count    int64  `json:"count"`
		LastSeen string `json:"last_seen"`
	}
	callerList := make([]relEntry, 0, len(callers))
	for _, e := range callers {
		callerList = append(callerList, relEntry{Service: e.Caller, API: e.API, Count: e.Count, LastSeen: e.LastSeen})
	}
	depList := make([]relEntry, 0, len(deps))
	for _, e := range deps {
		depList = append(depList, relEntry{Service: e.Target, API: e.API, Count: e.Count, LastSeen: e.LastSeen})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": serviceName,
		"callers": callerList,
		"deps":    depList,
	})
}
// AdminListServiceAuthServices handles GET /api/admin/v1/service-auth/services
// Returns all registered services for the admin panel.
func (h *Handlers) AdminListServiceAuthServices(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	services, err := h.ServiceAuthSvc.ListServices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if services == nil { services = []serviceauth.ServiceRecord{} }
	writeJSON(w, http.StatusOK, map[string]interface{}{"services": services, "count": len(services)})
}

// AdminGetServiceAuthService handles GET /api/admin/v1/service-auth/services/{id}
func (h *Handlers) AdminGetServiceAuthService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	s, err := h.ServiceAuthSvc.GetService(r.Context(), id)
	if err != nil {
		if errors.Is(err, serviceauth.ErrServiceNotFound) {
			writeError(w, http.StatusNotFound, "service not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s)
}

// AdminBlockServiceAuthService handles POST /api/admin/v1/service-auth/services/{id}/block
func (h *Handlers) AdminBlockServiceAuthService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	var body struct {
		Reason string `json:"reason"`
	}
	decodeJSON(r, &body)

	if err := h.ServiceAuthSvc.BlockService(r.Context(), id, body.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if h.PendingState != nil {
		h.PendingState.MarkPending("service auth block: " + id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "blocked"})
}

// AdminUnblockServiceAuth handles POST /api/admin/v1/service-auth/blocklist/{id}/unblock
func (h *Handlers) AdminUnblockServiceAuth(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	if err := h.ServiceAuthSvc.Unblock(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unblocked"})
}

// AdminDeleteServiceAuthService handles POST /api/admin/v1/service-auth/services/{id}/delete
func (h *Handlers) AdminDeleteServiceAuthService(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	id := r.PathValue("id")
	if err := h.ServiceAuthSvc.DeleteService(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// AdminServiceAuthTopology handles GET /api/admin/v1/service-auth/topology
func (h *Handlers) AdminServiceAuthTopology(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	window := 1 * time.Hour
	if w := r.URL.Query().Get("window"); w != "" {
		if d, err := time.ParseDuration(w); err == nil {
			window = d
		}
	}

	data, err := h.ServiceAuthSvc.GetTopology(r.Context(), window)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// AdminServiceAuthCallLogs handles GET /api/admin/v1/service-auth/call-logs
func (h *Handlers) AdminServiceAuthCallLogs(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	since := time.Now().Add(-1 * time.Hour)
	if s := r.URL.Query().Get("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 1000 {
			limit = n
		}
	}

	logs, err := h.ServiceAuthSvc.GetCallLogs(r.Context(), since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logs)
}

// ============================================================================
// Service-to-service call routing (v1.9B)
// ============================================================================

// ServiceAuthCall handles POST /api/service-auth/v1/call
// Routes a service-to-service call by service name instead of URL.
// The caller only needs to know the target service name and API path —
// Aegis resolves the backend host:port from the registration table.
//
// Request:  {"target": "project-service", "method": "POST", "path": "/api/v1/create", "body": {...}}
// Response: the target service's HTTP response, relayed as-is.
func (h *Handlers) ServiceAuthCall(w http.ResponseWriter, r *http.Request) {
	if h.ServiceAuthSvc == nil {
		writeError(w, http.StatusNotImplemented, "service auth not available")
		return
	}

	var reqBody struct {
		Target  string            `json:"target"`
		Method  string            `json:"method"`
		Path    string            `json:"path"`
		Body    json.RawMessage   `json:"body,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}
	if err := decodeJSON(r, &reqBody); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}
	if reqBody.Target == "" || reqBody.Method == "" || reqBody.Path == "" {
		writeError(w, http.StatusBadRequest, "target, method, and path are required")
		return
	}
	if reqBody.Method == "" {
		reqBody.Method = "POST"
	}

	// Look up target service by name.
	// FindByName may return multiple instances; pick the first active one.
	records, err := h.ServiceAuthSvc.FindByName(r.Context(), reqBody.Target)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *serviceauth.ServiceRecord
	for i := range records {
		if records[i].Status == "active" && records[i].ListenPort > 0 {
			target = &records[i]
			break
		}
	}

	// Resolve which node hosts the target.
	//   - Local record with remote NodeHost -> that node
	//   - Local record with local/empty NodeHost -> local forward
	//   - No local record -> fan out to peers to locate it (reuses ProxyRequest,
	//     no new Transport method — same骨架 as AdminDistNodeAggregate)
	targetNode := ""
	if target != nil {
		targetNode = target.NodeHost
	} else if h.DistNode != nil {
		targetNode = h.locateServiceNode(r.Context(), reqBody.Target)
		if targetNode == "" {
			writeError(w, http.StatusNotFound, "target service not found in cluster: "+reqBody.Target)
			return
		}
	} else {
		writeError(w, http.StatusNotFound, "target service not found or has no listen_port: "+reqBody.Target)
		return
	}

	// Cross-node: forward the whole call to the hosting node via ProxyRequest.
	// The remote node runs this same handler and resolves host:listen_port locally.
	if h.DistNode != nil && targetNode != "" && targetNode != h.DistNode.ID {
		callBody, _ := json.Marshal(reqBody)
		h.forwardServiceCall(w, r, targetNode, callBody)
		return
	}

	if target == nil {
		writeError(w, http.StatusNotFound, "target service not found or has no listen_port: "+reqBody.Target)
		return
	}

	// Build the forward URL: http://<host>:<listen_port><path>
	forwardURL := fmt.Sprintf("http://%s:%d%s", target.Host, target.ListenPort, reqBody.Path)

	// Build outgoing request — copy the ticket and caller headers so the
	// target's Guard middleware can verify the original caller's identity.
	var bodyReader io.Reader
	if len(reqBody.Body) > 0 {
		bodyReader = bytes.NewReader(reqBody.Body)
	}
	outReq, err := http.NewRequestWithContext(r.Context(), reqBody.Method, forwardURL, bodyReader)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build forward request: "+err.Error())
		return
	}
	outReq.Header.Set("Content-Type", "application/json")
	// Forward the original caller's identity headers so Guard can verify.
	if ticket := r.Header.Get("X-Service-Ticket"); ticket != "" {
		outReq.Header.Set("X-Service-Ticket", ticket)
	}
	if caller := r.Header.Get("X-Caller-Service"); caller != "" {
		outReq.Header.Set("X-Caller-Service", caller)
	}

	resp, err := http.DefaultClient.Do(outReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, "call "+reqBody.Target+": "+err.Error())
		return
	}
	defer resp.Body.Close()

	// Relay the response back to the caller.
	for k, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

// forwardServiceCall proxies a service-auth call to the node that hosts the
// target service, via the generic Aegis.ProxyRequest transport (no per-endpoint
// registration — same mechanism as AdminDistNodeAggregate). The remote node runs
// ServiceAuthCall against its own registry and does the final local forward.
func (h *Handlers) forwardServiceCall(w http.ResponseWriter, r *http.Request, nodeID string, callBody json.RawMessage) {
	if h.DistNode.Membership.GetPeer(nodeID) == nil {
		writeError(w, http.StatusBadGateway, "target service on unknown/unreachable node: "+nodeID)
		return
	}
	proxyReq := ProxyRequest{
		Method: "POST",
		Path:   "/api/service-auth/v1/call",
		Body:   callBody,
		Headers: map[string]string{
			"X-Service-Ticket": r.Header.Get("X-Service-Ticket"),
			"X-Caller-Service": r.Header.Get("X-Caller-Service"),
		},
	}
	var resp ProxyResponse
	if err := h.DistNode.Transport.Call(r.Context(), nodeID, "Aegis.ProxyRequest", proxyReq, &resp); err != nil {
		writeError(w, http.StatusBadGateway, "cross-node call to "+nodeID+": "+err.Error())
		return
	}
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

// locateServiceNode fans out to alive peers and returns the node ID of the first
// peer whose local registry has an active registration for name (with a
// listen_port). It reuses Aegis.ProxyRequest against the existing service list
// endpoint — no new transport method, same fan-out骨架 as AdminDistNodeAggregate.
// Returns "" when no peer hosts the service.
func (h *Handlers) locateServiceNode(ctx context.Context, name string) string {
	if h.DistNode == nil {
		return ""
	}
	for _, p := range h.DistNode.Membership.AlivePeers() {
		req := ProxyRequest{Method: "GET", Path: "/api/admin/v1/service-auth/services"}
		var resp ProxyResponse
		if err := h.DistNode.Transport.Call(ctx, p.Info.ID, "Aegis.ProxyRequest", req, &resp); err != nil {
			continue
		}
		if resp.StatusCode != 200 || len(resp.Body) == 0 {
			continue
		}
		var listResp struct {
			Services []serviceauth.ServiceRecord `json:"services"`
		}
		if err := json.Unmarshal(resp.Body, &listResp); err != nil {
			continue
		}
		for _, s := range listResp.Services {
			if s.Name == name && s.Status == "active" && s.ListenPort > 0 {
				return p.Info.ID
			}
		}
	}
	return ""
}

