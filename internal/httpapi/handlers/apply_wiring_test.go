package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestApplySurfaceFailsClosedWithoutWorkflow covers every handler on the apply
// surface at once.
//
// All seven dereferenced h.Workflow with no nil check. An unwired workflow — the
// exact condition a partial startup produces — panicked the request instead of
// answering it. A panic mid-response reaches the operator as a dropped
// connection: no status, no message, and on the Apply/Changes pages that is
// indistinguishable from the gateway being unreachable.
//
// Every other admin surface in this package already fails closed with 501. This
// pins the apply surface to the same convention, including the panic-freedom that
// is the actual regression risk: a future handler added here without the guard.
func TestApplySurfaceFailsClosedWithoutWorkflow(t *testing.T) {
	cases := []struct {
		name    string
		method  string
		path    string
		invoke  func(*Handlers, http.ResponseWriter, *http.Request)
		mutates bool
	}{
		{"ConfigPreview", http.MethodGet, "/api/config/preview",
			(*Handlers).ConfigPreview, false},
		{"ConfigCurrent", http.MethodGet, "/api/config/current",
			(*Handlers).ConfigCurrent, false},
		{"ConfigDiff", http.MethodGet, "/api/config/diff",
			(*Handlers).ConfigDiff, false},
		{"ApplyDryRun", http.MethodPost, "/api/apply/dry-run",
			(*Handlers).ApplyDryRun, false},
		{"ApplyHistory", http.MethodGet, "/api/apply/history",
			(*Handlers).ApplyHistory, false},
		{"ApplyConfig", http.MethodPost, "/api/apply",
			(*Handlers).ApplyConfig, true},
		{"Rollback", http.MethodPost, "/api/apply/rollback",
			(*Handlers).Rollback, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Handlers{} // deliberately unwired
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Fatalf("%s panicked on missing wiring (%v) — the client sees a dropped "+
							"connection, not an error", tc.name, p)
					}
				}()
				tc.invoke(h, rec, req)
			}()

			if rec.Code != http.StatusNotImplemented {
				t.Errorf("%s: status = %d, want 501 to match every other admin surface",
					tc.name, rec.Code)
			}
			if rec.Body.Len() == 0 {
				t.Errorf("%s: empty body — the operator needs to know what is missing", tc.name)
			}
		})
	}
}

// TestApplyMutatingHandlersDoNotActWithoutWorkflow states the property that makes
// the guard more than cosmetic: the two handlers that change live state must
// refuse before touching anything, not fail partway through.
func TestApplyMutatingHandlersDoNotActWithoutWorkflow(t *testing.T) {
	for name, invoke := range map[string]func(*Handlers, http.ResponseWriter, *http.Request){
		"ApplyConfig": (*Handlers).ApplyConfig,
		"Rollback":    (*Handlers).Rollback,
	} {
		h := &Handlers{}
		rec := httptest.NewRecorder()
		invoke(h, rec, httptest.NewRequest(http.MethodPost, "/x", nil))

		if rec.Code == http.StatusOK {
			t.Errorf("%s returned 200 with no workflow wired — it would report success "+
				"for an apply that never ran", name)
		}
	}
}
