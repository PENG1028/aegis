package serviceauth_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	core "aegis/internal/serviceauth"
	sdk "aegis/pkg/serviceauth"
	_ "modernc.org/sqlite"
)

func newTestServer(t *testing.T) (*core.Service, *httptest.Server, *sql.DB) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	runTestMigrations(t, db)

	svc, err := core.NewService(core.Dependencies{
		Repo:        core.NewRepository(db),
		Secrets:     &memorySecretStore{secret: make([]byte, 32)},
		NodeChecker: &allowAllChecker{},
		LogWriter:   nil,
		IDGen:       core.DefaultIDGen,
		MasterKey:   nil,
	})
	if err != nil {
		db.Close()
		t.Fatalf("new service: %v", err)
	}

	mux := http.NewServeMux()
	registerTestRoutes(mux, svc)
	srv := httptest.NewServer(mux)
	return svc, srv, db
}

func newSDKClient(t *testing.T, serverURL, name string, port int, apis []sdk.APIDef) *sdk.Client {
	t.Helper()
	os.Setenv("AEGIS_URL", serverURL)
	os.Setenv("SERVICE_HOST", "127.0.0.1")

	client, err := sdk.New(sdk.Config{
		ServiceName: name,
		ServicePort: port,
		APIs:        apis,
		AegisURL:    serverURL,
	})
	if err != nil {
		t.Fatalf("new sdk client %s: %v", name, err)
	}
	return client
}

// ============================================================================
// E2E: Two services register and verify they see each other
// ============================================================================

func TestE2E_TwoServicesRegisterAndCall(t *testing.T) {
	_, srv, db := newTestServer(t)
	defer srv.Close()
	defer db.Close()
	ctx := context.Background()

	aAPIs := []sdk.APIDef{
		{Name: "validateAdmin", Path: "/api/v1/admin/validate", Method: "POST"},
	}
	clientA := newSDKClient(t, srv.URL, "admin-service", 3001, aAPIs)
	defer clientA.Close()
	if err := clientA.Register(ctx); err != nil {
		t.Fatalf("register A: %v", err)
	}

	bAPIs := []sdk.APIDef{
		{Name: "createProject", Path: "/api/v1/projects", Method: "POST"},
		{Name: "getUserList", Path: "/api/v1/users", Method: "GET"},
	}
	clientB := newSDKClient(t, srv.URL, "project-service", 3002, bAPIs)
	defer clientB.Close()
	if err := clientB.Register(ctx); err != nil {
		t.Fatalf("register B: %v", err)
	}

	if clientA.ServiceID() == "" || clientB.ServiceID() == "" {
		t.Error("empty service IDs after registration")
	}
	t.Logf("A ID=%s, B ID=%s", clientA.ServiceID(), clientB.ServiceID())

	// Verify DB has both.
	rows, _ := db.Query("SELECT name, host, port, status FROM svc_auth_services ORDER BY name")
	defer rows.Close()
	count := 0
	for rows.Next() {
		var name, host, status string
		var port int
		rows.Scan(&name, &host, &port, &status)
		t.Logf("  DB: %s @ %s:%d [%s]", name, host, port, status)
		count++
	}
	if count != 2 {
		t.Errorf("expected 2 services, got %d", count)
	}
}

// ============================================================================
// E2E: Guard rejects invalid tickets, accepts valid ones
// ============================================================================

func TestE2E_GuardRejectsInvalidTicket(t *testing.T) {
	_, srv, db := newTestServer(t)
	defer srv.Close()
	defer db.Close()
	ctx := context.Background()

	apis := []sdk.APIDef{{Name: "create", Path: "/api/v1/create", Method: "POST"}}
	clientB := newSDKClient(t, srv.URL, "project-service", 3002, apis)
	defer clientB.Close()
	if err := clientB.Register(ctx); err != nil {
		t.Fatalf("register B: %v", err)
	}

	// Start a simulated project-service with Guard middleware.
	guarded := http.NewServeMux()
	guarded.Handle("POST /api/v1/create", clientB.Guard("create",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			caller := sdk.CallerFromContext(r.Context())
			json.NewEncoder(w).Encode(map[string]string{
				"status": "created", "caller": caller.ServiceName,
			})
		}),
	))
	testB := httptest.NewServer(guarded)
	defer testB.Close()

	clusterSecret := clientB.ClusterSecret()

	// Test 1: No ticket → 401
	resp, _ := http.Post(testB.URL+"/api/v1/create", "application/json", bytes.NewReader([]byte(`{}`)))
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("Test 1 (no ticket): expected 401, got %d: %s", resp.StatusCode, body)
	} else {
		t.Log("PASS: no ticket → 401")
	}

	// Test 2: Garbage ticket → 403
	req, _ := http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", "this-is-garbage")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("Test 2 (garbage ticket): expected 403, got %d: %s", resp.StatusCode, body)
	} else {
		t.Log("PASS: garbage ticket → 403")
	}

	// Test 3: Valid ticket → 200
	claims := core.NewTicket("admin-service", "project-service", "create")
	validTicket := core.SignTicket(claims, clusterSecret)
	req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", validTicket)
	req.Header.Set("X-Caller-Service", "admin-service")
	req.Header.Set("X-Caller-Host", "127.0.0.1")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("Test 3 (valid ticket): expected 200, got %d: %s", resp.StatusCode, body)
	} else {
		t.Logf("PASS: valid ticket → 200: %s", body)
	}

	// Test 4: Wrong key → 403
	fakeKey := make([]byte, 32)
	fakeKey[0] = 0xFF
	wrongTicket := core.SignTicket(claims, fakeKey)
	req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", wrongTicket)
	req.Header.Set("X-Caller-Service", "admin-service")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("Test 4 (wrong key): expected 403, got %d: %s", resp.StatusCode, body)
	} else {
		t.Log("PASS: wrong key → 403")
	}

	// Test 5: Expired ticket → 403
	expiredClaims := core.TicketClaims{
		CallerService: "admin-service",
		TargetService: "project-service",
		TargetAPI:     "create",
		ExpiresAt:     time.Now().Add(-1 * time.Hour).Unix(),
	}
	expiredTicket := core.SignTicket(expiredClaims, clusterSecret)
	req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", expiredTicket)
	req.Header.Set("X-Caller-Service", "admin-service")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("Test 5 (expired): expected 403, got %d: %s", resp.StatusCode, body)
	} else {
		t.Log("PASS: expired ticket → 403")
	}

	// Test 6: Ticket for wrong target service → 403
		otherClaims := core.NewTicket("admin-service", "other-service", "create")
		otherTicket := core.SignTicket(otherClaims, clusterSecret)
		req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Service-Ticket", otherTicket)
		req.Header.Set("X-Caller-Service", "admin-service")
		resp, _ = http.DefaultClient.Do(req)
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("Test 6 (wrong target): expected 403, got %d: %s", resp.StatusCode, body)
		} else {
			t.Log("PASS: wrong target service → 403")
		}

		// Test 7: Ticket for wrong API → 403
		wrongAPIClaims := core.NewTicket("admin-service", "project-service", "delete")
		wrongAPITicket := core.SignTicket(wrongAPIClaims, clusterSecret)
		req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Service-Ticket", wrongAPITicket)
		req.Header.Set("X-Caller-Service", "admin-service")
		resp, _ = http.DefaultClient.Do(req)
		body, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 403 {
			t.Errorf("Test 7 (wrong api): expected 403, got %d: %s", resp.StatusCode, body)
		} else {
			t.Log("PASS: wrong api → 403")
		}
}

// ============================================================================
// E2E: Block service, verify calls rejected
// ============================================================================

func TestE2E_BlockServicePreventsCalls(t *testing.T) {
	svc, srv, db := newTestServer(t)
	defer srv.Close()
	defer db.Close()
	ctx := context.Background()

	clientA := newSDKClient(t, srv.URL, "admin-service", 3001, []sdk.APIDef{
		{Name: "ping", Path: "/ping", Method: "GET"},
	})
	defer clientA.Close()
	if err := clientA.Register(ctx); err != nil {
		t.Fatalf("register A: %v", err)
	}

	clientB := newSDKClient(t, srv.URL, "project-service", 3002, []sdk.APIDef{
		{Name: "create", Path: "/api/v1/create", Method: "POST"},
	})
	defer clientB.Close()
	if err := clientB.Register(ctx); err != nil {
		t.Fatalf("register B: %v", err)
	}

	// Start guarded endpoint.
	guarded := http.NewServeMux()
	guarded.Handle("POST /api/v1/create", clientB.Guard("create",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(200)
			w.Write([]byte(`{"status":"ok"}`))
		}),
	))
	testB := httptest.NewServer(guarded)
	defer testB.Close()

	clusterSecret := clientB.ClusterSecret()
	claims := core.NewTicket("admin-service", "project-service", "create")
	validTicket := core.SignTicket(claims, clusterSecret)

	// Pre-block: should pass.
	req, _ := http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", validTicket)
	req.Header.Set("X-Caller-Service", "admin-service")
	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("pre-block: expected 200, got %d: %s", resp.StatusCode, body)
	}
	t.Log("PASS: valid ticket before block → 200")

	// Block admin-service.
	all, _ := svc.ListServices(ctx)
	var targetID string
	for _, s := range all {
		if s.Name == "admin-service" {
			targetID = s.ID
			break
		}
	}
	if targetID == "" {
		t.Fatal("admin-service not found")
	}
	if err := svc.BlockService(ctx, targetID, "e2e test"); err != nil {
		t.Fatalf("block: %v", err)
	}
	t.Logf("Blocked admin-service (id=%s)", targetID)

	// Re-register B to pull fresh blocklist.
	if err := clientB.Register(ctx); err != nil {
		t.Logf("re-register B (expected duplicate, ok): %v", err)
	}

	// Post-block: should be rejected.
	req, _ = http.NewRequest("POST", testB.URL+"/api/v1/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Ticket", validTicket)
	req.Header.Set("X-Caller-Service", "admin-service")
	resp, _ = http.DefaultClient.Do(req)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("post-block: expected 403, got %d: %s", resp.StatusCode, body)
	} else {
		t.Log("PASS: blocked service → 403")
	}
}

// ============================================================================
// Helpers
// ============================================================================

type memorySecretStore struct{ secret []byte }

func (s *memorySecretStore) Load() ([]byte, error) {
	if len(s.secret) == 0 {
		return nil, os.ErrNotExist
	}
	return s.secret, nil
}
func (s *memorySecretStore) Save(secret []byte) error {
	s.secret = secret
	return nil
}

type allowAllChecker struct{}

func (c *allowAllChecker) FindByIP(ip string) (*core.NodeInfo, error) {
	return &core.NodeInfo{NodeID: "test"}, nil
}

func runTestMigrations(t *testing.T, db *sql.DB) {
	t.Helper()
	for i, m := range []string{
		`CREATE TABLE IF NOT EXISTS svc_auth_services (id TEXT PRIMARY KEY, name TEXT NOT NULL, host TEXT NOT NULL, port INTEGER NOT NULL DEFAULT 0, node_host TEXT NOT NULL DEFAULT '', apis_json TEXT NOT NULL DEFAULT '[]', status TEXT NOT NULL DEFAULT 'active', last_seen TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS svc_auth_call_logs (id TEXT PRIMARY KEY, caller_service TEXT NOT NULL DEFAULT '', target_service TEXT NOT NULL DEFAULT '', target_api TEXT NOT NULL DEFAULT '', caller_host TEXT NOT NULL DEFAULT '', target_host TEXT NOT NULL DEFAULT '', allowed INTEGER NOT NULL DEFAULT 1, latency_ms INTEGER NOT NULL DEFAULT 0, error_msg TEXT NOT NULL DEFAULT '', called_at TEXT NOT NULL DEFAULT '')`,
		`CREATE TABLE IF NOT EXISTS svc_auth_blocklist (id TEXT PRIMARY KEY, service_id TEXT, api_name TEXT NOT NULL DEFAULT '*', reason TEXT NOT NULL DEFAULT '', version INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT '')`,
	} {
		if _, err := db.Exec(m); err != nil {
			t.Fatalf("migration %d: %v", i, err)
		}
	}
}

func registerTestRoutes(mux *http.ServeMux, svc *core.Service) {
	mux.HandleFunc("POST /api/service-auth/v1/register", func(w http.ResponseWriter, r *http.Request) {
		var req core.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = "127.0.0.1"
		}
		resp, err := svc.Register(r.Context(), req, ip)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("GET /api/service-auth/v1/sync", func(w http.ResponseWriter, r *http.Request) {
		blVer, _ := parseIntParam(r, "bl_version")
		catVer, _ := parseIntParam(r, "cat_version")
		resp, _ := svc.Sync(r.Context(), blVer, catVer)
		if resp.NotModified {
			w.WriteHeader(304)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("POST /api/service-auth/v1/report", func(w http.ResponseWriter, r *http.Request) {
		var req core.ReportRequest
		json.NewDecoder(r.Body).Decode(&req)
		svc.Report(r.Context(), req)
		w.WriteHeader(200)
	})
}

func parseIntParam(r *http.Request, name string) (int64, error) {
	var v int64
	_, err := fmt.Sscanf(r.URL.Query().Get(name), "%d", &v)
	return v, err
}
