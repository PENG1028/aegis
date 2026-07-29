package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aegis/internal/hostdep/provider"
)

// ctlProvider implements the base provider.Provider and records what was asked of
// it. The optional interfaces (Reloadable, ServiceController, Lifecycle) are
// implemented by the wrapper types below, because these handlers branch on type
// assertion — a provider that lacks a capability must be refused, not silently
// handled by some fallback.
type ctlProvider struct {
	state     provider.ProviderState
	applyErr  error
	applyCall [][]provider.ConfigFile
}

func (p *ctlProvider) State() provider.ProviderState                       { return p.state }
func (p *ctlProvider) Diagnose() provider.ProviderDiagnostic               { return provider.ProviderDiagnostic{} }
func (p *ctlProvider) Render(provider.Plan) ([]provider.ConfigFile, error) { return nil, nil }
func (p *ctlProvider) Apply(cfgs []provider.ConfigFile) error {
	p.applyCall = append(p.applyCall, cfgs)
	return p.applyErr
}

// ctlReloadable adds Reload.
type ctlReloadable struct {
	*ctlProvider
	reloadErr   error
	reloadCalls int
}

func (p *ctlReloadable) Reload() error {
	p.reloadCalls++
	return p.reloadErr
}

// ctlServiceCtl adds Start/Stop/Restart.
type ctlServiceCtl struct {
	*ctlProvider
	startErr, stopErr, restartErr error
	calls                         []string
}

func (p *ctlServiceCtl) Start() error {
	p.calls = append(p.calls, "start")
	return p.startErr
}
func (p *ctlServiceCtl) Stop() error {
	p.calls = append(p.calls, "stop")
	return p.stopErr
}
func (p *ctlServiceCtl) Restart() error {
	p.calls = append(p.calls, "restart")
	return p.restartErr
}

// ctlLifecycle adds install/uninstall.
type ctlLifecycle struct {
	*ctlProvider
	canUninstall   bool
	uninstallErr   error
	uninstallCalls int
}

func (p *ctlLifecycle) CanInstall() bool { return false }
func (p *ctlLifecycle) Install() error   { return nil }
func (p *ctlLifecycle) CanUninstall() bool {
	return p.canUninstall
}
func (p *ctlLifecycle) Uninstall() error {
	p.uninstallCalls++
	return p.uninstallErr
}

var (
	_ provider.Provider           = (*ctlProvider)(nil)
	_ provider.ReloadableProvider = (*ctlReloadable)(nil)
	_ provider.ServiceController  = (*ctlServiceCtl)(nil)
	_ provider.LifecycleProvider  = (*ctlLifecycle)(nil)
)

func ctlHandlers(p provider.Provider) *Handlers {
	reg := provider.NewRegistry()
	if p != nil {
		reg.Register(p)
	}
	return &Handlers{ProvReg: reg}
}

func ctlDo(h *Handlers, fn func(*Handlers, http.ResponseWriter, *http.Request),
	method, name, body string) (*httptest.ResponseRecorder, map[string]any) {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, "/api/admin/v1/providers/"+name, nil)
	} else {
		r = httptest.NewRequest(method, "/api/admin/v1/providers/"+name, strings.NewReader(body))
	}
	r.SetPathValue("provider", name)
	rec := httptest.NewRecorder()
	fn(h, rec, r)

	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec, out
}

func baseState(id string) provider.ProviderState {
	return provider.ProviderState{ID: id, Name: id, Installed: true, Running: true,
		ConfigPath: "/etc/" + id + "/" + id + ".cfg"}
}

// TestProviderControlRejectsUnknownProvider covers all four endpoints. The path
// segment is operator-supplied, so a typo must be refused rather than resolved to
// something else — acting on the wrong gateway is the failure to avoid.
func TestProviderControlRejectsUnknownProvider(t *testing.T) {
	endpoints := map[string]struct {
		fn     func(*Handlers, http.ResponseWriter, *http.Request)
		method string
		body   string
	}{
		"reload":     {(*Handlers).ProviderReload, http.MethodPost, ""},
		"service":    {(*Handlers).ProviderServiceControl, http.MethodPost, `{"action":"restart"}`},
		"uninstall":  {(*Handlers).ProviderUninstall, http.MethodDelete, ""},
		"saveConfig": {(*Handlers).ProviderSaveConfig, http.MethodPut, `{"content":"x"}`},
	}
	for name, ep := range endpoints {
		t.Run(name, func(t *testing.T) {
			h := ctlHandlers(&ctlProvider{state: baseState("caddy")})

			rec, _ := ctlDo(h, ep.fn, ep.method, "cadddy", ep.body) // typo
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 for an unregistered provider: %s",
					rec.Code, rec.Body.String())
			}
		})
	}
}

// TestReloadRefusesProviderWithoutReloadCapability guards the type assertion. A
// provider that cannot reload must be told so; reporting success would leave the
// operator believing new config is live when the process never re-read it.
func TestReloadRefusesProviderWithoutReloadCapability(t *testing.T) {
	h := ctlHandlers(&ctlProvider{state: baseState("caddy")}) // base only, no Reload

	rec, body := ctlDo(h, (*Handlers).ProviderReload, http.MethodPost, "caddy", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if status, _ := body["status"].(string); status == "success" {
		t.Error("a provider with no reload capability reported success")
	}
}

// TestReloadReportsFailureInBody pins this surface's convention: the request was
// valid, so it answers 200, and the outcome lives in status. A caller that only
// reads the HTTP code cannot distinguish a completed reload from a refused one, so
// status and error both have to be present.
func TestReloadReportsFailureInBody(t *testing.T) {
	p := &ctlReloadable{ctlProvider: &ctlProvider{state: baseState("caddy")},
		reloadErr: fmt.Errorf("config validation failed at line 12")}
	h := ctlHandlers(p)

	rec, body := ctlDo(h, (*Handlers).ProviderReload, http.MethodPost, "caddy", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if got, _ := body["status"].(string); got != "failed" {
		t.Errorf("status = %q, want \"failed\" — this endpoint answers 200 on a valid request, "+
			"so the body is the only place the outcome appears", got)
	}
	if e, _ := body["error"].(string); !strings.Contains(e, "line 12") {
		t.Errorf("the provider's reason must reach the operator, got %q", e)
	}
	if p.reloadCalls != 1 {
		t.Errorf("Reload ran %d times, want 1", p.reloadCalls)
	}
}

// TestServiceControlRejectsUnknownActionBeforeTouchingTheService is the important
// one on this endpoint. The action string is operator-supplied and reaches
// systemctl in the fallback path; anything outside start/stop/restart must be
// refused before any process control happens.
func TestServiceControlRejectsUnknownActionBeforeTouchingTheService(t *testing.T) {
	for _, action := range []string{"disable", "mask", "kill", "", "restart; rm -rf /"} {
		t.Run(fmt.Sprintf("action=%q", action), func(t *testing.T) {
			p := &ctlServiceCtl{ctlProvider: &ctlProvider{state: baseState("caddy")}}
			h := ctlHandlers(p)

			body := fmt.Sprintf(`{"action":%q}`, action)
			rec, _ := ctlDo(h, (*Handlers).ProviderServiceControl, http.MethodPost, "caddy", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if len(p.calls) != 0 {
				t.Errorf("the service was acted on with an unvalidated action: %v", p.calls)
			}
		})
	}
}

// TestServiceControlPrefersCapabilityOverSystemctl guards the branch the comment in
// the handler promises. If the ServiceController assertion stops matching, control
// silently falls through to shelling out to systemctl — which bypasses whatever the
// provider does around start/stop and, on a host without that unit, reports failure
// for a service that is fine.
func TestServiceControlPrefersCapabilityOverSystemctl(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			p := &ctlServiceCtl{ctlProvider: &ctlProvider{state: baseState("caddy")}}
			h := ctlHandlers(p)

			rec, body := ctlDo(h, (*Handlers).ProviderServiceControl, http.MethodPost, "caddy",
				fmt.Sprintf(`{"action":%q}`, action))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
			}
			if len(p.calls) != 1 || p.calls[0] != action {
				t.Fatalf("provider calls = %v, want exactly [%s] — control fell through to "+
					"systemctl, bypassing the provider's own service handling", p.calls, action)
			}
			if got, _ := body["status"].(string); got != "success" {
				t.Errorf("status = %q, want success", got)
			}
		})
	}
}

// TestServiceControlReportsCapabilityFailure pins that a provider-reported failure
// is not smoothed into success.
func TestServiceControlReportsCapabilityFailure(t *testing.T) {
	p := &ctlServiceCtl{ctlProvider: &ctlProvider{state: baseState("caddy")},
		restartErr: fmt.Errorf("unit entered failed state")}
	h := ctlHandlers(p)

	_, body := ctlDo(h, (*Handlers).ProviderServiceControl, http.MethodPost, "caddy",
		`{"action":"restart"}`)
	if got, _ := body["status"].(string); got != "failed" {
		t.Errorf("status = %q, want failed", got)
	}
	if running, ok := body["running"].(bool); !ok || running {
		t.Error("running must be reported false after a failed restart — a caller reading it " +
			"as up would stop investigating")
	}
}

// TestServiceControlRefusesProviderWithNoService guards the empty-ID gate. Without
// it the fallback path would run `systemctl <action> ""`.
func TestServiceControlRefusesProviderWithNoService(t *testing.T) {
	st := baseState("caddy")
	st.ID = ""
	h := ctlHandlers(&ctlProvider{state: st})

	// Registered under the empty ID, so resolve by that name.
	r := httptest.NewRequest(http.MethodPost, "/api/admin/v1/providers/", strings.NewReader(`{"action":"start"}`))
	r.SetPathValue("provider", "")
	rec := httptest.NewRecorder()
	h.ProviderServiceControl(rec, r)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 — an empty service name would reach systemctl: %s",
			rec.Code, rec.Body.String())
	}
}

// TestUninstallHonoursCanUninstall is the destructive endpoint. CanUninstall is the
// provider's own veto — a provider that says no must not have Uninstall called on
// it at all, since by then removal has already begun.
func TestUninstallHonoursCanUninstall(t *testing.T) {
	p := &ctlLifecycle{ctlProvider: &ctlProvider{state: baseState("caddy")}, canUninstall: false}
	h := ctlHandlers(p)

	rec, _ := ctlDo(h, (*Handlers).ProviderUninstall, http.MethodDelete, "caddy", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if p.uninstallCalls != 0 {
		t.Error("Uninstall ran despite CanUninstall() being false — removal would have started " +
			"on a provider that declared it unsafe")
	}
}

// TestUninstallRefusesProviderWithoutLifecycle covers the same gate one level up: a
// provider with no lifecycle capability cannot be uninstalled at all.
func TestUninstallRefusesProviderWithoutLifecycle(t *testing.T) {
	h := ctlHandlers(&ctlProvider{state: baseState("caddy")}) // no LifecycleProvider

	rec, _ := ctlDo(h, (*Handlers).ProviderUninstall, http.MethodDelete, "caddy", "")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

// TestUninstallStatesConfigIsPreserved pins the message. An operator deciding
// whether to uninstall needs to know config survives; without it the safe action
// reads as destructive and gets avoided, or the reverse.
func TestUninstallStatesConfigIsPreserved(t *testing.T) {
	p := &ctlLifecycle{ctlProvider: &ctlProvider{state: baseState("caddy")}, canUninstall: true}
	h := ctlHandlers(p)

	_, body := ctlDo(h, (*Handlers).ProviderUninstall, http.MethodDelete, "caddy", "")
	if got, _ := body["status"].(string); got != "uninstalled" {
		t.Errorf("status = %q, want uninstalled", got)
	}
	if msg, _ := body["message"].(string); !strings.Contains(strings.ToLower(msg), "config") {
		t.Errorf("the message should say what happened to config, got %q", msg)
	}
	if p.uninstallCalls != 1 {
		t.Errorf("Uninstall ran %d times, want 1", p.uninstallCalls)
	}
}

// TestSaveConfigRejectsEmptyContent guards against truncating a gateway config to
// nothing. An empty body would validate as "no routes" and reload cleanly, taking
// every site down without any error to point at.
func TestSaveConfigRejectsEmptyContent(t *testing.T) {
	for name, body := range map[string]string{
		"empty string": `{"content":""}`,
		"absent field": `{}`,
		"bad json":     `{"content":`,
	} {
		t.Run(name, func(t *testing.T) {
			p := &ctlProvider{state: baseState("caddy")}
			h := ctlHandlers(p)

			rec, _ := ctlDo(h, (*Handlers).ProviderSaveConfig, http.MethodPut, "caddy", body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			if len(p.applyCall) != 0 {
				t.Error("an empty config was written to the gateway — it would reload cleanly " +
					"and serve nothing")
			}
		})
	}
}

// TestSaveConfigWritesToTheProviderConfigPath pins where content lands and that it
// is newline-terminated. A config written to the wrong path leaves the gateway
// running its old file while the UI shows the new one.
func TestSaveConfigWritesToTheProviderConfigPath(t *testing.T) {
	p := &ctlProvider{state: baseState("caddy")}
	h := ctlHandlers(p)

	_, body := ctlDo(h, (*Handlers).ProviderSaveConfig, http.MethodPut, "caddy",
		`{"content":"listen 80"}`)

	if len(p.applyCall) != 1 || len(p.applyCall[0]) != 1 {
		t.Fatalf("Apply calls = %v, want one file", p.applyCall)
	}
	f := p.applyCall[0][0]
	if f.Path != "/etc/caddy/caddy.cfg" {
		t.Errorf("written to %q, want the provider's own ConfigPath", f.Path)
	}
	if string(f.Content) != "listen 80\n" {
		t.Errorf("content = %q, want the body plus a trailing newline", string(f.Content))
	}
	if got, _ := body["config_path"].(string); got != "/etc/caddy/caddy.cfg" {
		t.Errorf("config_path = %q — the response must name the file actually written", got)
	}
}

// TestSaveConfigReportsApplyFailure pins that a rejected config is not reported as
// saved. This is the most consequential of the four bodies: an operator who
// believes a config is live stops looking for why the change had no effect.
func TestSaveConfigReportsApplyFailure(t *testing.T) {
	p := &ctlProvider{state: baseState("caddy"),
		applyErr: fmt.Errorf("validate failed: unknown directive 'listn'")}
	h := ctlHandlers(p)

	_, body := ctlDo(h, (*Handlers).ProviderSaveConfig, http.MethodPut, "caddy",
		`{"content":"listn 80"}`)

	if got, _ := body["status"].(string); got != "save_failed" {
		t.Errorf("status = %q, want save_failed", got)
	}
	if e, _ := body["error"].(string); !strings.Contains(e, "listn") {
		t.Errorf("the validation reason must reach the operator, got %q", e)
	}
}

// TestProviderControlFailsClosedWithoutRegistry matches the convention on the rest
// of the admin surface: unwired dependencies answer, they do not panic.
func TestProviderControlFailsClosedWithoutRegistry(t *testing.T) {
	for name, fn := range map[string]func(*Handlers, http.ResponseWriter, *http.Request){
		"reload":     (*Handlers).ProviderReload,
		"service":    (*Handlers).ProviderServiceControl,
		"uninstall":  (*Handlers).ProviderUninstall,
		"saveConfig": (*Handlers).ProviderSaveConfig,
	} {
		t.Run(name, func(t *testing.T) {
			h := &Handlers{} // no ProvReg

			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("panicked with no registry (%v) — the client sees a dropped "+
						"connection", p)
				}
			}()
			rec, _ := ctlDo(h, fn, http.MethodPost, "caddy", `{"action":"restart","content":"x"}`)
			if rec.Code < 400 {
				t.Errorf("status = %d, want an error status", rec.Code)
			}
		})
	}
}
