package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"aegis/internal/distnode"
)

// ──────────────────────────────────────────────────────────────────────────────
// View Proxy — cross-node perspective switching
//
// Instead of registering every API endpoint on Transport individually,
// Aegis.ProxyRequest receives a generic HTTP request specification and executes
// it against the local mux, capturing the response. This lets any API endpoint
// be called cross-node without per-endpoint registration.
//
// Flow:
//   Panel A (X-Aegis-View-As: node_b) → Transport.Call("node_b", "Aegis.ProxyRequest", req)
//   → Node B executes against its local mux → returns response
//   → Panel A returns response to browser
//
// The transport authentication (cluster secret) is trusted; the proxied request
// does not re-validate admin session — the caller already proved it's a cluster member.
// ──────────────────────────────────────────────────────────────────────────────

// ProxyRequest is sent from one node to another to execute an API call remotely.
type ProxyRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Query   string            `json:"query"`
	Body    json.RawMessage   `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ProxyResponse is returned after executing the request on the target node.
type ProxyResponse struct {
	StatusCode int               `json:"status_code"`
	Body       json.RawMessage   `json:"body"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// recordingWriter captures http.ResponseWriter output for transport proxying.
type recordingWriter struct {
	code    int
	headers http.Header
	body    bytes.Buffer
}

func (w *recordingWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(b)
}

func (w *recordingWriter) WriteHeader(code int) {
	w.code = code
}

// NewProxyHandler creates a distnode Transport handler that proxies HTTP requests
// through the local mux. This is the generic cross-node API mechanism.
//
// Usage in RegisterAegisTransportHandlers:
//
//	h.proxyMux = mux  // store mux on Handlers
//	dn.Transport.Register("Aegis.ProxyRequest", newProxyHandler(h))
func newProxyHandler(h *Handlers) func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
	return func(ctx context.Context, callerID string, args json.RawMessage) (interface{}, error) {
		var req ProxyRequest
		if err := json.Unmarshal(args, &req); err != nil {
			return nil, err
		}

		// Reconstruct HTTP request
		var bodyReader io.Reader
		if len(req.Body) > 0 {
			bodyReader = bytes.NewReader(req.Body)
		}

		httpReq, err := http.NewRequestWithContext(ctx, req.Method, req.Path, bodyReader)
		if err != nil {
			return ProxyResponse{StatusCode: 500, Body: json.RawMessage(`{"error":"bad request"}`)}, nil
		}

		if req.Query != "" {
			httpReq.URL.RawQuery = req.Query
		}
		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}
		httpReq.Header.Set("Content-Type", "application/json")

		// Execute against local mux
		rw := &recordingWriter{code: 200}
		h.proxyMux.ServeHTTP(rw, httpReq)

		// Build response headers (filter sensitive ones)
		respHeaders := make(map[string]string)
		for k, vals := range rw.headers {
			if strings.ToLower(k) == "content-length" || strings.ToLower(k) == "transfer-encoding" {
				continue
			}
			if len(vals) > 0 {
				respHeaders[k] = vals[0]
			}
		}

		return ProxyResponse{
			StatusCode: rw.code,
			Body:       json.RawMessage(rw.body.Bytes()),
			Headers:    respHeaders,
		}, nil
	}
}

// NewViewProxyHandler creates an HTTP middleware that intercepts requests with
// X-Aegis-View-As header and forwards them to the target node via Transport.
//
// Must be placed BEFORE auth middleware so proxied requests skip local auth.
// The target node's proxy handler will authenticate via X-Aegis-Proxy header.
//
// Usage in cli/serve.go:
//
//	if svcs.DistNode != nil {
//	    handler = handlers.NewViewProxyHandler(svcs.DistNode)(handler)
//	}
func NewViewProxyHandler(dn *distnode.DistNode) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			targetID := r.Header.Get("X-Aegis-View-As")
			if targetID == "" || dn == nil || targetID == dn.ID {
				// Local request — pass through
				next.ServeHTTP(w, r)
				return
			}

			// Read body
			body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB max
			if err != nil {
				http.Error(w, `{"error":"cannot read request body"}`, http.StatusBadRequest)
				return
			}

			var bodyRaw json.RawMessage
			if len(body) > 0 {
				bodyRaw = body
			}

			// Build proxy request (no headers needed — remote sets its own)
			req := ProxyRequest{
				Method:  r.Method,
				Path:    r.URL.Path,
				Query:   r.URL.RawQuery,
				Body:    bodyRaw,
				Headers: nil,
			}

			// Forward to target node
			var resp ProxyResponse
			err = dn.Transport.Call(r.Context(), targetID, "Aegis.ProxyRequest", req, &resp)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				json.NewEncoder(w).Encode(map[string]string{
					"error":  "proxy: target unreachable",
					"target": targetID,
				})
				return
			}

			// Write response headers from remote
			for k, v := range resp.Headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(resp.StatusCode)
			if len(resp.Body) > 0 {
				w.Write(resp.Body)
			}
		})
	}
}
