package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aegis/internal/action"
	"aegis/internal/aegisgateway"
	"aegis/internal/cluster"
	"aegis/internal/store"

	_ "modernc.org/sqlite"
)

// scopedRequest builds a request authenticated as the given token type, the same
// shape the auth middleware produces.
func scopedRequest(method, target, tokenType string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	return req.WithContext(action.WithActionContext(req.Context(), &action.ActionContext{
		SpaceID:   "caller",
		TokenType: tokenType,
		TokenID:   "ticket",
		Actor:     "api",
	}))
}

// TestCapabilityCallDeniesCallerOutsideDeclaredScope is the boundary test for the
// gap this change closed. node.list declared Scopes:["service"] from the day it
// was written, but nothing read that field — any authenticated caller could
// invoke it. The declaration is now the gate.
func TestCapabilityCallDeniesCallerOutsideDeclaredScope(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := scopedRequest(http.MethodPost, "/api/service-auth/v1/capabilities/node.list/call", "space")
	req.SetPathValue("name", "node.list")
	rec := httptest.NewRecorder()

	h.ServiceCapabilityCall(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d — a space caller invoked a capability scoped to service/admin. body=%s",
			rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

// TestCapabilityListReflectsCallerScope keeps the advertised set and the callable
// set identical per caller. If they diverge, the SDK offers calls that always 403.
func TestCapabilityListReflectsCallerScope(t *testing.T) {
	for tokenType, wantNames := range map[string][]string{
		"service": {"node.list"},
		"admin":   {"node.list"},
		"space":   {},
	} {
		h := &Handlers{NodeSvc: newTestNodeService(t)}
		req := scopedRequest(http.MethodGet, "/api/service-auth/v1/capabilities", tokenType)
		rec := httptest.NewRecorder()

		h.ServiceCapabilities(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status = %d body=%s", tokenType, rec.Code, rec.Body.String())
		}
		var body struct {
			Capabilities []struct {
				Name   string   `json:"name"`
				Scopes []string `json:"scopes"`
			} `json:"capabilities"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: decode: %v", tokenType, err)
		}
		if len(body.Capabilities) != len(wantNames) {
			t.Errorf("%s caller sees %d capabilities, want %d: %+v",
				tokenType, len(body.Capabilities), len(wantNames), body.Capabilities)
			continue
		}
		for i, want := range wantNames {
			if body.Capabilities[i].Name != want {
				t.Errorf("%s: capability[%d] = %s, want %s", tokenType, i, body.Capabilities[i].Name, want)
			}
		}
	}
}

// TestCapabilityScopesSurviveJSONRoundTrip pins the wire spelling. The scope
// values are a contract with the SDK; renaming the Go constants without changing
// the JSON would be invisible here otherwise.
func TestCapabilityScopesSurviveJSONRoundTrip(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := scopedRequest(http.MethodGet, "/api/service-auth/v1/capabilities", "service")
	rec := httptest.NewRecorder()

	h.ServiceCapabilities(rec, req)

	var body struct {
		Capabilities []struct {
			Name     string   `json:"name"`
			ReadOnly bool     `json:"read_only"`
			Scopes   []string `json:"scopes"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Capabilities) == 0 {
		t.Fatal("no capabilities returned")
	}
	for _, c := range body.Capabilities {
		if len(c.Scopes) == 0 {
			t.Errorf("capability %s serialized with no scopes: the SDK cannot tell who may call it", c.Name)
		}
		for _, s := range c.Scopes {
			if !aegisgateway.Scope(s).Valid() {
				t.Errorf("capability %s advertises scope %q which the registry does not recognize", c.Name, s)
			}
		}
	}
}

// TestUnauthenticatedCapabilityCallIsRejected keeps the no-context path at 401
// rather than letting an empty scope fall through to a 403 or, worse, a match.
func TestUnauthenticatedCapabilityCallIsRejected(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := httptest.NewRequest(http.MethodPost, "/api/service-auth/v1/capabilities/node.list/call", nil)
	req.SetPathValue("name", "node.list")
	rec := httptest.NewRecorder()

	h.ServiceCapabilityCall(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// TestUnknownTokenTypeGetsNoCapabilities is the fail-closed check at the HTTP
// boundary. A token type the capability layer has not been taught about must see
// nothing, not everything.
func TestUnknownTokenTypeGetsNoCapabilities(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}
	req := scopedRequest(http.MethodGet, "/api/service-auth/v1/capabilities", "operator")
	rec := httptest.NewRecorder()

	h.ServiceCapabilities(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d for an unrecognized token type", rec.Code, http.StatusUnauthorized)
	}
}

// TestRegistryWiresMutationRecorderFromPendingState proves the handler supplies a
// recorder when PendingState exists, and that invoking a mutating capability
// actually lands a pending marker end to end.
//
// This is the wiring that makes the first ReadOnly=false capability registrable at
// all. It also demonstrates the intended consequence: a capability that changes
// cluster state leaves the same trace an admin mutation endpoint leaves, so the
// Changes page and `aegis apply` see it.
func TestRegistryWiresMutationRecorderFromPendingState(t *testing.T) {
	db := newPendingDB(t)
	pending := cluster.NewPendingState(db)
	h := &Handlers{NodeSvc: newTestNodeService(t), PendingState: pending}

	reg, err := h.newCapabilityRegistry()
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Register(aegisgateway.Capability{
		Name:     "test.mutating",
		ReadOnly: false,
		Scopes:   []aegisgateway.Scope{aegisgateway.ScopeAdmin},
	}, func(context.Context, aegisgateway.CapabilityRequest) (interface{}, error) {
		return map[string]string{"ok": "true"}, nil
	}); err != nil {
		t.Fatalf("registering a mutating capability failed, so no recorder was wired: %v", err)
	}

	if pending.Status().Pending {
		t.Fatal("fixture started with a pending marker")
	}
	if _, err := reg.Invoke(context.Background(), "test.mutating",
		aegisgateway.CapabilityRequest{}, aegisgateway.ScopeAdmin); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	status := pending.Status()
	if !status.Pending {
		t.Error("a mutating capability ran without recording pending apply — the change is invisible to apply and to the operator")
	}
	if status.Reason == "" || !strings.Contains(status.Reason, "test.mutating") {
		t.Errorf("pending reason = %q, want it to name the capability", status.Reason)
	}
}

// TestRegistryWithoutPendingStateRefusesMutatingCapability pins the fail-loud
// direction. If PendingState is absent, a mutating capability must fail to
// register rather than run and record nothing.
func TestRegistryWithoutPendingStateRefusesMutatingCapability(t *testing.T) {
	h := &Handlers{NodeSvc: newTestNodeService(t)}

	reg, err := h.newCapabilityRegistry()
	if err != nil {
		t.Fatalf("build registry: %v", err)
	}
	if err := reg.Register(aegisgateway.Capability{
		Name:     "test.mutating",
		ReadOnly: false,
		Scopes:   []aegisgateway.Scope{aegisgateway.ScopeAdmin},
	}, func(context.Context, aegisgateway.CapabilityRequest) (interface{}, error) {
		return nil, nil
	}); err == nil {
		t.Error("a mutating capability registered with no pending-apply recorder available")
	}
}

func newPendingDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "pending.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.RunMigrations(db); err != nil {
		t.Fatal(err)
	}
	return db
}
