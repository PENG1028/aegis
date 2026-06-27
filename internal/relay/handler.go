package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aegis/internal/endpoint"
	"aegis/internal/id"
	"aegis/internal/logs"
	"aegis/internal/secrets"
)

// Relay header names.
const (
	HeaderRouteID        = "X-Aegis-Route-ID"
	HeaderGatewayID      = "X-Aegis-Gateway-ID"
	HeaderGatewayToken   = "X-Aegis-Gateway-Token"
	HeaderSourceNode     = "X-Aegis-Source-Node"
	HeaderHop            = "X-Aegis-Hop"
	HeaderRequestID      = "X-Aegis-Request-ID"
	HeaderOriginalPath   = "X-Aegis-Original-Path"   // v1.8C-8A: original request path
	HeaderOriginalQuery  = "X-Aegis-Original-Query"  // v1.8C-8A: original request query
	HeaderOriginalMethod = "X-Aegis-Original-Method" // v1.8C-8A: original request method
)

// MaxHopLimit is the maximum allowed relay hops.
const MaxHopLimit = 1

// HandlerDependencies for the relay dispatch handler.
type HandlerDeps struct {
	RouteRepo     RouteRepo
	EndpointRepo  EndpointRepo
	NodeRepo      NodeRepo
	GWLinkRepo    GWLinkRepo
	LogSvc        *logs.AppService
	MasterKey     *secrets.MasterKey // v1.8B-5: for decrypting GatewayLink secrets
}

// RelayHandler handles incoming relay requests on /__aegis/relay.
type RelayHandler struct {
	deps HandlerDeps
}

// NewRelayHandler creates a new relay dispatch handler.
func NewRelayHandler(deps HandlerDeps) *RelayHandler {
	return &RelayHandler{deps: deps}
}

// ServeHTTP implements http.Handler for the relay endpoint.
func (h *RelayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeRelayError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "only POST is accepted for relay")
		return
	}

	reqID := r.Header.Get(HeaderRequestID)
	routeID := r.Header.Get(HeaderRouteID)
	gatewayID := r.Header.Get(HeaderGatewayID)
	gatewayToken := r.Header.Get(HeaderGatewayToken)
	sourceNodeID := r.Header.Get(HeaderSourceNode)
	hopStr := r.Header.Get(HeaderHop)

	if routeID == "" {
		writeRelayError(w, http.StatusBadRequest, "MISSING_ROUTE_ID", "X-Aegis-Route-ID is required")
		return
	}
	if gatewayID == "" {
		writeRelayError(w, http.StatusBadRequest, "MISSING_GATEWAY_ID", "X-Aegis-Gateway-ID is required")
		return
	}
	if gatewayToken == "" {
		writeRelayError(w, http.StatusBadRequest, "MISSING_GATEWAY_TOKEN", "X-Aegis-Gateway-Token is required")
		return
	}
	if sourceNodeID == "" {
		writeRelayError(w, http.StatusBadRequest, "MISSING_SOURCE_NODE", "X-Aegis-Source-Node is required")
		return
	}

	// Validate hop count
	hop := 0
	if hopStr != "" {
		hop, _ = strconv.Atoi(hopStr)
	}
	if hop > MaxHopLimit {
		writeRelayError(w, http.StatusLoopDetected, "MAX_HOPS_EXCEEDED",
			fmt.Sprintf("hop count %d exceeds limit of %d", hop, MaxHopLimit))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, "", "max_hops_exceeded", reqID)
		return
	}

	// 1. Look up route
	rt, err := h.deps.RouteRepo.FindByID(routeID)
	if err != nil || rt == nil {
		writeRelayError(w, http.StatusNotFound, "ROUTE_NOT_FOUND",
			fmt.Sprintf("route %s not found", routeID))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, "", "route_not_found", reqID)
		return
	}

	// 2. Validate GatewayLink — FindByID already includes auth_value
	gwLink, err := h.deps.GWLinkRepo.FindByID(gatewayID)
	if err != nil || gwLink == nil {
		writeRelayError(w, http.StatusUnauthorized, "INVALID_GATEWAY",
			fmt.Sprintf("gateway %s not found", gatewayID))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "gateway_not_found", reqID)
		return
	}
	if !gwLink.CheckAuthEncrypted(gatewayToken, h.deps.MasterKey) {
		writeRelayError(w, http.StatusForbidden, "INVALID_GATEWAY_TOKEN",
			"gateway token verification failed")
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "invalid_token", reqID)
		return
	}

	// 3. Verify source node exists
	srcNode, err := h.deps.NodeRepo.FindByNodeID(sourceNodeID)
	if err != nil || srcNode == nil {
		writeRelayError(w, http.StatusForbidden, "UNKNOWN_SOURCE_NODE",
			fmt.Sprintf("source node %s not found", sourceNodeID))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "unknown_source", reqID)
		return
	}

	// 4. Get current node
	currentNode, err := h.deps.NodeRepo.FindCurrent()
	if err != nil || currentNode == nil {
		writeRelayError(w, http.StatusInternalServerError, "NODE_IDENTITY_ERROR",
			"cannot determine current node identity")
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "no_identity", reqID)
		return
	}

	// 5. Look up endpoints
	eps, err := h.deps.EndpointRepo.FindEnabledByServiceID(rt.ServiceID)
	if err != nil || len(eps) == 0 {
		writeRelayError(w, http.StatusNotFound, "NO_ENDPOINTS",
			fmt.Sprintf("service %s has no enabled endpoints", rt.ServiceID))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "no_endpoints", reqID)
		return
	}

	// 6. Find local endpoint with strict node_id enforcement
	var localEP *endpoint.Endpoint
	for i := range eps {
		if eps[i].NodeID != "" && (eps[i].NodeID == currentNode.NodeID || eps[i].NodeID == currentNode.ID) {
			localEP = &eps[i]
			break
		}
	}
	if localEP == nil {
		// Check if there's an endpoint with empty node_id or wrong node_id
		for i := range eps {
			if eps[i].NodeID == "" {
				writeRelayError(w, http.StatusConflict, "ENDPOINT_NODE_UNKNOWN",
					fmt.Sprintf("endpoint %s has empty node_id — must be set to %s for relay", eps[i].ID, currentNode.NodeID))
				h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "endpoint_node_unknown", reqID)
				return
			}
		}
		writeRelayError(w, http.StatusConflict, "ENDPOINT_NOT_LOCAL",
			fmt.Sprintf("no endpoint for route %s belongs to this node", routeID))
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "endpoint_not_local", reqID)
		return
	}

	// 7. Prevent forwarding to remote IPs
	targetHost, targetPort := localEP.HostPort()
	if targetHost != "127.0.0.1" && targetHost != "localhost" {
		if localEP.NodeID != currentNode.NodeID && localEP.NodeID != currentNode.ID {
			writeRelayError(w, http.StatusConflict, "ENDPOINT_NOT_LOCAL",
				fmt.Sprintf("endpoint %s target %s is not local to node %s", localEP.ID, targetHost, currentNode.NodeID))
			h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "endpoint_remote", reqID)
			return
		}
	}

	// Reject target headers (open proxy prevention)
	if r.Header.Get("X-Aegis-Target-Host") != "" || r.Header.Get("X-Aegis-Target-Port") != "" {
		writeRelayError(w, http.StatusBadRequest, "TARGET_HEADER_REJECTED",
			"X-Aegis-Target-Host and X-Aegis-Target-Port headers are not allowed")
		h.logRelayEvent("relay_rejected", routeID, sourceNodeID, gatewayID, "target_header_rejected", reqID)
		return
	}

	// 8. Forward to 127.0.0.1:target_port
	targetAddr := fmt.Sprintf("127.0.0.1:%d", targetPort)

	h.logRelayEvent("relay_forward", routeID, sourceNodeID, gatewayID, "forwarding", reqID)

	// Determine target path: use X-Aegis-Original-Path if provided (v1.8C-8A),
	// otherwise fall back to r.URL.Path for backward compatibility.
	targetPath := r.Header.Get(HeaderOriginalPath)
	if targetPath == "" {
		targetPath = r.URL.Path
	}
	targetQuery := r.Header.Get(HeaderOriginalQuery)
	targetURL := fmt.Sprintf("http://%s%s", targetAddr, targetPath)
	if targetQuery != "" {
		targetURL += "?" + targetQuery
	}

	// Use original method if provided (v1.8C-8A), fall back to r.Method for backward compat
	targetMethod := r.Header.Get(HeaderOriginalMethod)
	if targetMethod == "" {
		targetMethod = r.Method
	}

	proxyReq, err := http.NewRequestWithContext(r.Context(), targetMethod, targetURL, r.Body)
	if err != nil {
		writeRelayError(w, http.StatusInternalServerError, "PROXY_ERROR", err.Error())
		return
	}

	// Copy headers (strip Aegis internal ones)
	for key, vals := range r.Header {
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "X-AEGIS-") {
			continue
		}
		for _, v := range vals {
			proxyReq.Header.Add(key, v)
		}
	}
	proxyReq.Header.Set("X-Forwarded-For", srcNode.PublicIP)
	proxyReq.Header.Set("X-Forwarded-Host", rt.Domain)
	proxyReq.Header.Set("X-Relay-Node", currentNode.NodeID)

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
		},
	}

	resp, err := client.Do(proxyReq)
	if err != nil {
		writeRelayError(w, http.StatusBadGateway, "TARGET_UNREACHABLE",
			fmt.Sprintf("local target %s unreachable: %v", targetAddr, err))
		h.logRelayEvent("relay_failed", routeID, sourceNodeID, gatewayID, "target_unreachable", reqID)
		return
	}
	defer resp.Body.Close()

	for key, vals := range resp.Header {
		for _, v := range vals {
			w.Header().Add(key, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)

	h.logRelayEvent("relay_success", routeID, sourceNodeID, gatewayID,
		fmt.Sprintf("forwarded to 127.0.0.1:%d (status %d)", targetPort, resp.StatusCode), reqID)
}

func (h *RelayHandler) logRelayEvent(eventType, routeID, sourceNode, gatewayID, detail, reqID string) {
	if h.deps.LogSvc == nil {
		return
	}
	msg := fmt.Sprintf("relay %s: route=%s source=%s gateway=%s detail=%s",
		eventType, routeID, sourceNode, gatewayID, detail)
	if reqID != "" {
		msg = fmt.Sprintf("%s req=%s", msg, reqID)
	}

	severity := "info"
	if strings.HasPrefix(eventType, "relay_rejected") || strings.HasPrefix(eventType, "relay_failed") {
		severity = "warning"
	}

	h.deps.LogSvc.LogNodeEvent(&logs.NodeEvent{
		ID:        id.New("evt"),
		NodeID:    sourceNode,
		EventType: eventType,
		Severity:  severity,
		Message:   msg,
		CreatedAt: time.Now(),
	})
}

func writeRelayError(w http.ResponseWriter, statusCode int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// RelayHandlerForMux returns an http.Handler that wraps RelayHandler for use in a mux.
func RelayHandlerForMux(h *RelayHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	})
}
