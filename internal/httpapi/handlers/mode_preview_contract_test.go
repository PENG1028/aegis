package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"aegis/internal/hostdep/provider"
	"aegis/internal/logs"
	"aegis/internal/route"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// previewHarness wires ModePreview against a real route store and a registry
// whose installed providers decide which target modes are reachable.
type previewHarness struct {
	h  *Handlers
	db *sql.DB
}

func newPreviewHarness(t *testing.T, installed ...string) *previewHarness {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}

	registry := provider.NewRegistry()
	for _, id := range installed {
		registry.Register(&modeTLSProvider{state: providerStateFor(id)})
	}

	return &previewHarness{
		h: &Handlers{
			Route:   route.NewAppService(route.NewRepository(db), logs.NewAppService(logs.NewRepository(db)), nil),
			ProvReg: registry,
		},
		db: db,
	}
}

func providerStateFor(id string) provider.ProviderState {
	base := provider.ProviderState{
		ID: id, Name: id, Installed: true, Running: true, Status: "ready", Ready: true,
	}
	switch id {
	case "caddy":
		base.Capabilities = []provider.Capability{
			provider.CapLoadCert, provider.CapTLSTerminate, provider.CapAutoCert,
			provider.CapListenTCP, provider.CapRouteHost, provider.CapHTTP1,
			provider.CapUpstreamTCP,
		}
	case "haproxy":
		base.Capabilities = []provider.Capability{
			provider.CapListenTCP, provider.CapSNIPreread, provider.CapTLSPassthrough,
			provider.CapUpstreamTCP,
		}
	}
	return base
}

func (p *previewHarness) insertRoute(t *testing.T, id, domain, composition string) {
	t.Helper()
	err := route.NewRepository(p.db).Create(&route.Route{
		ID: id, Domain: domain, ServiceID: "svc_test", Composition: composition,
		TLSEnabled: composition != "http_route", Status: "active",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("insert route %s: %v", id, err)
	}
}

type previewBody struct {
	Preview struct {
		CurrentMode string `json:"current_mode"`
		TargetMode  string `json:"target_mode"`
	} `json:"preview"`
	RPCBConflicts []struct {
		RouteID         string   `json:"route_id"`
		Domain          string   `json:"domain"`
		Capabilities    []string `json:"capabilities"`
		Compatible      bool     `json:"compatible"`
		Reason          string   `json:"reason"`
		CurrentExecutor string   `json:"current_executor"`
		TargetExecutor  string   `json:"target_executor"`
		StateClass      string   `json:"state_class"`
		Migration       string   `json:"migration"`
	} `json:"rpcb_conflicts"`
}

func (p *previewHarness) preview(t *testing.T, target string) (int, previewBody) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/mode/preview?target="+target, nil)
	rec := httptest.NewRecorder()
	p.h.ModePreview(rec, req)

	var body previewBody
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode preview: %v — body: %s", err, rec.Body.String())
		}
	}
	return rec.Code, body
}

// TestModePreviewReportsIncompatibleRouteBeforeSwitching is the assertion the
// debug scripts made by eye, fifteen times.
//
// A TLS passthrough route needs SNI pre-read, which only HAProxy provides. In
// legacy mode HAProxy is gone, so that route has no executor. The preview is the
// only thing standing between an operator and a switch that silently strands the
// route: if it reports compatible, the switch proceeds and the domain goes dark
// with no error anywhere.
func TestModePreviewReportsIncompatibleRouteBeforeSwitching(t *testing.T) {
	p := newPreviewHarness(t, "caddy", "haproxy")
	p.insertRoute(t, "rt_pass", "pass.example.com", "tls_passthrough")
	p.insertRoute(t, "rt_http", "web.example.com", "https_route")

	status, body := p.preview(t, "legacy")
	if status != http.StatusOK {
		t.Fatalf("preview status = %d", status)
	}
	if len(body.RPCBConflicts) != 2 {
		t.Fatalf("expected one entry per route, got %d", len(body.RPCBConflicts))
	}

	byID := map[string]int{}
	for i, rc := range body.RPCBConflicts {
		byID[rc.RouteID] = i
	}

	pass := body.RPCBConflicts[byID["rt_pass"]]
	if pass.Compatible {
		t.Error("tls_passthrough reported compatible with legacy mode, which has no SNI pre-read — " +
			"an operator would switch and strand the route")
	}
	if pass.TargetExecutor != "" {
		t.Errorf("an incompatible route must not name a target executor, got %q", pass.TargetExecutor)
	}
	if pass.Reason == "" {
		t.Error("an incompatible route must carry a reason; the UI shows it verbatim")
	}

	web := body.RPCBConflicts[byID["rt_http"]]
	if !web.Compatible {
		t.Errorf("https_route must remain compatible in legacy mode: reason=%q", web.Reason)
	}
	if web.TargetExecutor == "" {
		t.Error("a compatible route must name the executor that will own it after the switch")
	}
}

// TestModePreviewNamesBothExecutorsForAMigratingRoute pins the fields the UI uses
// to render "who owns this now, who owns it after". A route whose executor changes
// is the case an operator most needs to see; if current and target are both blank
// the switch looks like a no-op.
func TestModePreviewNamesBothExecutorsForAMigratingRoute(t *testing.T) {
	p := newPreviewHarness(t, "caddy", "haproxy")
	p.insertRoute(t, "rt_web", "web.example.com", "https_route")

	_, body := p.preview(t, "edge_mux")
	if len(body.RPCBConflicts) != 1 {
		t.Fatalf("expected one conflict entry, got %d", len(body.RPCBConflicts))
	}
	rc := body.RPCBConflicts[0]

	if rc.Domain != "web.example.com" {
		t.Errorf("domain = %q, want the route's domain — the UI keys its rows on it", rc.Domain)
	}
	if len(rc.Capabilities) == 0 {
		t.Error("capabilities must be reported: they are the provenance the switch decision rests on")
	}
	if rc.StateClass == "" {
		t.Error("state_class must be reported — it decides whether the certificate survives the switch")
	}
	if rc.Migration == "" {
		t.Error("migration strategy must be reported; a blank value renders as an empty cell")
	}
}

// TestModePreviewRejectsUnknownAndMissingTarget guards the two input errors. A
// silently-defaulted target would preview one mode and switch to another.
func TestModePreviewRejectsUnknownAndMissingTarget(t *testing.T) {
	p := newPreviewHarness(t, "caddy")

	for _, target := range []string{"", "not_a_mode"} {
		status, _ := p.preview(t, target)
		if status != http.StatusBadRequest {
			t.Errorf("target=%q: status = %d, want 400 — a defaulted target would preview the wrong mode",
				target, status)
		}
	}
}

// TestModePreviewFailsClosedWithoutWiring guards that a missing registry or route
// service returns 501 rather than an empty, reassuring preview.
func TestModePreviewFailsClosedWithoutWiring(t *testing.T) {
	for name, h := range map[string]*Handlers{
		"no registry": {Route: &route.AppService{}},
		"no routes":   {ProvReg: provider.NewRegistry()},
		"neither":     {},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/admin/v1/mode/preview?target=legacy", nil)
		rec := httptest.NewRecorder()
		h.ModePreview(rec, req)
		if rec.Code != http.StatusNotImplemented {
			t.Errorf("%s: status = %d, want 501 — an empty preview reads as 'nothing will break'",
				name, rec.Code)
		}
	}
}
