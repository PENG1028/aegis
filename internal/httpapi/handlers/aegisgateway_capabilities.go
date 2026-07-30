package handlers

import (
	"net/http"

	"aegis/internal/action"
	"aegis/internal/aegisgateway"
)

func (h *Handlers) ServiceCapabilities(w http.ResponseWriter, r *http.Request) {
	caller, ok := callerScope(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	reg, err := h.newCapabilityRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": reg.List(caller),
	})
}

func (h *Handlers) ServiceCapabilityCall(w http.ResponseWriter, r *http.Request) {
	caller, ok := callerScope(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "service auth required")
		return
	}
	name := r.PathValue("name")
	var req aegisgateway.CapabilityRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
	}
	reg, err := h.newCapabilityRegistry()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp, err := reg.Invoke(r.Context(), name, req, caller)
	if err != nil {
		writeError(w, aegisgateway.ErrorStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// newCapabilityRegistry builds the capability set for one request.
//
// The mutation recorder is wired here rather than inside the registry so
// aegisgateway keeps no dependency on cluster state. When PendingState is absent
// the recorder is nil, and Register then refuses any capability declaring
// ReadOnly=false — a startup-visible failure instead of mutations that leave no
// pending marker behind.
func (h *Handlers) newCapabilityRegistry() (*aegisgateway.CapabilityRegistry, error) {
	var opts []aegisgateway.RegistryOption
	if h.PendingState != nil {
		opts = append(opts, aegisgateway.WithMutationRecorder(func(capability string) error {
			return h.PendingState.MarkPending("capability invoked: " + capability)
		}))
	}
	reg := aegisgateway.NewCapabilityRegistry(opts...)
	if err := aegisgateway.RegisterNodeCapabilities(reg, h.NodeSvc); err != nil {
		return nil, err
	}
	return reg, nil
}

// callerScope resolves the request's authenticated caller class into a capability
// scope. A request with no action context, or one whose token type the capability
// layer does not recognize, gets no scope — the registry then denies everything.
func callerScope(r *http.Request) (aegisgateway.Scope, bool) {
	ac := action.GetActionContext(r.Context())
	if ac == nil {
		return "", false
	}
	return aegisgateway.ScopeFor(ac.TokenType)
}
