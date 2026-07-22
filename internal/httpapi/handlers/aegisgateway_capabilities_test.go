package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"aegis/internal/action"
	"aegis/internal/node"
	"aegis/internal/store"
)

func TestServiceCapabilitiesRequiresServiceAuth(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/api/service-auth/v1/capabilities", nil)
	rec := httptest.NewRecorder()

	h.ServiceCapabilities(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServiceCapabilitiesListsNodeCapability(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := serviceRequest(http.MethodGet, "/api/service-auth/v1/capabilities", nil)
	rec := httptest.NewRecorder()

	h.ServiceCapabilities(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Capabilities []struct {
			Name     string `json:"name"`
			ReadOnly bool   `json:"read_only"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Capabilities) != 1 || body.Capabilities[0].Name != "node.list" || !body.Capabilities[0].ReadOnly {
		t.Fatalf("capabilities = %+v", body.Capabilities)
	}
}

func TestServiceCapabilityCallNodeList(t *testing.T) {
	nodeSvc := newTestNodeService(t)
	if _, err := nodeSvc.CreateNode("alpha", node.RoleWorker, "alpha-host", "43.159.34.11", "10.0.0.11", "linux", "amd64", "test"); err != nil {
		t.Fatalf("create node: %v", err)
	}
	h := &Handlers{NodeSvc: nodeSvc}
	req := serviceRequest(http.MethodPost, "/api/service-auth/v1/capabilities/node.list/call", nil)
	req.SetPathValue("name", "node.list")
	rec := httptest.NewRecorder()

	h.ServiceCapabilityCall(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Capability string `json:"capability"`
		Result     struct {
			Count int `json:"count"`
			Nodes []struct {
				NodeID string `json:"node_id"`
				Name   string `json:"name"`
			} `json:"nodes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Capability != "node.list" || body.Result.Count != 1 {
		t.Fatalf("response = %+v", body)
	}
	if body.Result.Nodes[0].Name != "alpha" {
		t.Fatalf("nodes = %+v", body.Result.Nodes)
	}
}

func TestServiceCapabilityCallMissingCapability(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := serviceRequest(http.MethodPost, "/api/service-auth/v1/capabilities/missing/call", nil)
	req.SetPathValue("name", "missing")
	rec := httptest.NewRecorder()

	h.ServiceCapabilityCall(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func serviceRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req = req.WithContext(action.WithActionContext(req.Context(), &action.ActionContext{
		SpaceID:   "caller-service",
		TokenType: "service",
		TokenID:   "ticket",
		Actor:     "api",
	}))
	return req
}

func newTestNodeService(t *testing.T) *node.Service {
	t.Helper()
	db, err := store.OpenSQLite(filepath.Join(t.TempDir(), "aegis.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Initialize(db); err != nil {
		t.Fatal(err)
	}
	repo := node.NewRepository(db)
	return node.NewService(repo)
}
