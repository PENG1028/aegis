package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aegis/internal/cluster"
	"aegis/internal/flowbridge"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// newFlowBridgeHandler builds a Handlers with a real DB, pending state and a
// live /health stub on the probe target.
func newFlowBridgeHandler(t *testing.T) (*Handlers, *httptest.Server, *sql.DB) {
	t.Helper()
	healthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(healthSrv.Close)

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	h := &Handlers{
		FlowBridgeSvc: flowbridge.NewService(flowbridge.NewRepository(db), nil),
		Route:         route.NewAppService(route.NewRepository(db), logs.NewAppService(logs.NewRepository(db)), nil),
		PendingState:  cluster.NewPendingState(db),
	}
	return h, healthSrv, db
}

func flowbridgeRequest(t *testing.T, h *Handlers, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	parts := strings.Split(strings.TrimPrefix(path, "/api/admin/v1/flowbridge/"), "/")
	if len(parts) == 1 && parts[0] != "" && parts[0] != "flowbridge" {
		req.SetPathValue("id", parts[0])
	} else if len(parts) == 2 {
		req.SetPathValue("id", parts[0])
	}
	res := httptest.NewRecorder()
	switch {
	case method == http.MethodPost && path == "/api/admin/v1/flowbridge":
		h.AdminCreateFlowBridge(res, req)
	case method == http.MethodGet && path == "/api/admin/v1/flowbridge":
		h.AdminListFlowBridge(res, req)
	case method == http.MethodGet && strings.HasSuffix(path, "/check") == false && strings.Count(path, "/") == 5:
		h.AdminGetFlowBridge(res, req)
	case method == http.MethodPatch && strings.Count(path, "/") == 5:
		h.AdminUpdateFlowBridge(res, req)
	case method == http.MethodPost && strings.HasSuffix(path, "/check"):
		h.AdminCheckFlowBridge(res, req)
	case method == http.MethodDelete && strings.Count(path, "/") == 5:
		h.AdminDeleteFlowBridge(res, req)
	default:
		t.Fatalf("unhandled request %s %s", method, path)
	}
	return res
}

func TestFlowBridgeLifecycle(t *testing.T) {
	h, healthSrv, _ := newFlowBridgeHandler(t)
	control := strings.TrimPrefix(healthSrv.URL, "http://")

	// FUN-1: create -> 201 + healthy (initial check against /health)
	res := flowbridgeRequest(t, h, http.MethodPost, "/api/admin/v1/flowbridge",
		`{"name":"edge-prod","machine_ip":"10.0.0.5","data_plane_port":8080,"control_address":"`+control+`"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d: %s", res.Code, res.Body.String())
	}
	var created map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("create returned no id")
	}
	if created["last_health_status"] != "healthy" {
		t.Fatalf("initial health check expected healthy, got %v", created["last_health_status"])
	}

	// SEC-2: create marks pending
	if !h.PendingState.Status().Pending {
		t.Fatal("create did not mark pending apply")
	}

	// FUN-2: explicit check endpoint
	res = flowbridgeRequest(t, h, http.MethodPost, "/api/admin/v1/flowbridge/"+id+"/check", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"healthy"`) {
		t.Fatalf("check: %d %s", res.Code, res.Body.String())
	}

	// list -> 1 item
	res = flowbridgeRequest(t, h, http.MethodGet, "/api/admin/v1/flowbridge", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "edge-prod") {
		t.Fatalf("list: %d %s", res.Code, res.Body.String())
	}

	// SEC-4: validation — bad IP rejected (400)
	res = flowbridgeRequest(t, h, http.MethodPatch, "/api/admin/v1/flowbridge/"+id,
		`{"machine_ip":"not-an-ip"}`)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("bad IP update: expected 400, got %d", res.Code)
	}

	// bind a route to the instance
	now := "2026-08-01T00:00:00Z"
	if err := h.Route.CreateRouteDirect(&route.Route{
		ID: "rt_fb", Domain: "a.test", ServiceID: "svc_test",
		Composition: "https_route", TLSEnabled: true, SourceProvider: "caddy",
		Status: "active", FlowBridgeID: &id, CreatedAt: parseTestTime(t, now), UpdatedAt: parseTestTime(t, now),
	}); err != nil {
		t.Fatal(err)
	}

	// FUN-4: delete while referenced -> 409
	res = flowbridgeRequest(t, h, http.MethodDelete, "/api/admin/v1/flowbridge/"+id, "")
	if res.Code != http.StatusConflict {
		t.Fatalf("referenced delete: expected 409, got %d: %s", res.Code, res.Body.String())
	}

	// FUN-3: disable via PATCH
	res = flowbridgeRequest(t, h, http.MethodPatch, "/api/admin/v1/flowbridge/"+id, `{"enabled":false}`)
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), `"enabled":false`) {
		t.Fatalf("disable: %d %s", res.Code, res.Body.String())
	}

	// unbind then delete succeeds
	if err := h.Route.DeleteRoute(context.Background(), "rt_fb"); err != nil {
		t.Fatal(err)
	}
	res = flowbridgeRequest(t, h, http.MethodDelete, "/api/admin/v1/flowbridge/"+id, "")
	if res.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", res.Code, res.Body.String())
	}
}

func TestFlowBridgeUnavailableWithoutWiring(t *testing.T) {
	h := &Handlers{}
	res := flowbridgeRequest(t, h, http.MethodGet, "/api/admin/v1/flowbridge", "")
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without wiring, got %d", res.Code)
	}
}

func parseTestTime(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
