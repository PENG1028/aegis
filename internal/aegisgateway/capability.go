package aegisgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
)

// Scope names a class of caller allowed to invoke a capability.
//
// The vocabulary is deliberately the same one action.ActionContext.TokenType
// carries. TestScopeVocabularyMatchesActionTokenTypes pins the two together,
// because either half drifting fails silently: a scope no token type can produce
// locks its capability away from every caller, and a token type with no matching
// scope locks its callers out of every capability. Both look like "the feature
// just doesn't work" rather than like a bug.
type Scope string

const (
	ScopeAdmin   Scope = "admin"
	ScopeService Scope = "service"
	ScopeSpace   Scope = "space"
)

// AllScopes returns every scope a capability may be declared for.
func AllScopes() []Scope { return []Scope{ScopeAdmin, ScopeService, ScopeSpace} }

// Valid reports whether s is a scope the registry recognizes.
func (s Scope) Valid() bool {
	for _, known := range AllScopes() {
		if s == known {
			return true
		}
	}
	return false
}

// ScopeFor maps an action context token type onto a capability scope.
//
// It fails closed: an unrecognized token type yields no scope rather than a
// default, so a new caller class added to the auth layer cannot silently inherit
// access to existing capabilities.
func ScopeFor(tokenType string) (Scope, bool) {
	s := Scope(tokenType)
	if !s.Valid() {
		return "", false
	}
	return s, true
}

// Capability describes one invocable capability exposed to authenticated callers.
//
// Scopes and ReadOnly are enforced by the registry, not advisory:
//   - Scopes gates Invoke and filters List. At least one valid scope is required
//     at registration.
//   - ReadOnly=false means the capability mutates cluster state, so the registry
//     records a pending-apply marker after a successful invoke. Registering a
//     mutating capability without a recorder configured is an error.
type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ReadOnly    bool    `json:"read_only"`
	Scopes      []Scope `json:"scopes,omitempty"`
}

type CapabilityRequest struct {
	Input json.RawMessage `json:"input,omitempty"`
}

type CapabilityResponse struct {
	Capability string      `json:"capability"`
	Result     interface{} `json:"result,omitempty"`
}

type CapabilityInvoker func(ctx context.Context, req CapabilityRequest) (interface{}, error)

// MutationRecorder records that a state-mutating capability ran, so an operator
// and the apply pipeline can see that desired state moved ahead of applied state.
// In Aegis this is cluster.PendingState.MarkPending.
type MutationRecorder func(capability string) error

// RegistryOption configures a CapabilityRegistry at construction.
type RegistryOption func(*CapabilityRegistry)

// WithMutationRecorder supplies the recorder used after a successful invoke of a
// capability whose ReadOnly is false. Without it, registering such a capability
// fails rather than quietly skipping the marker.
func WithMutationRecorder(rec MutationRecorder) RegistryOption {
	return func(r *CapabilityRegistry) { r.recordMutation = rec }
}

type CapabilityRegistry struct {
	capabilities   map[string]registeredCapability
	recordMutation MutationRecorder
}

type registeredCapability struct {
	info   Capability
	invoke CapabilityInvoker
}

func NewCapabilityRegistry(opts ...RegistryOption) *CapabilityRegistry {
	r := &CapabilityRegistry{capabilities: make(map[string]registeredCapability)}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Register adds a capability, rejecting any declaration the registry cannot
// enforce. Every rejection here surfaces at startup as a failure to build the
// registry, which is the loudest available signal — the alternative is a
// capability that is reachable by nobody, or one that mutates state without
// anything noticing.
func (r *CapabilityRegistry) Register(info Capability, invoke CapabilityInvoker) error {
	if r == nil {
		return fmt.Errorf("capability registry is not available")
	}
	if info.Name == "" {
		return fmt.Errorf("capability name is required")
	}
	if invoke == nil {
		return fmt.Errorf("capability %s has no invoker", info.Name)
	}
	if _, exists := r.capabilities[info.Name]; exists {
		return fmt.Errorf("capability %s already registered", info.Name)
	}
	if len(info.Scopes) == 0 {
		return fmt.Errorf("capability %s must declare at least one scope", info.Name)
	}
	for _, s := range info.Scopes {
		if !s.Valid() {
			return fmt.Errorf("capability %s declares unknown scope %q", info.Name, s)
		}
	}
	if !info.ReadOnly && r.recordMutation == nil {
		return fmt.Errorf("capability %s mutates state but no mutation recorder is configured", info.Name)
	}
	r.capabilities[info.Name] = registeredCapability{info: info, invoke: invoke}
	return nil
}

// List returns the capabilities the given caller may invoke, in name order.
// A caller never sees a capability it would be denied.
func (r *CapabilityRegistry) List(caller Scope) []Capability {
	if r == nil {
		return []Capability{}
	}
	out := make([]Capability, 0, len(r.capabilities))
	for _, cap := range r.capabilities {
		if allows(cap.info, caller) {
			out = append(out, cap.info)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Invoke runs a capability on behalf of a caller in the given scope.
//
// The scope check happens before the invoker runs, and a scope miss is 403 rather
// than 404: hiding existence would mean an operator debugging a denied service
// call cannot tell "wrong caller class" from "capability gone".
func (r *CapabilityRegistry) Invoke(ctx context.Context, name string, req CapabilityRequest, caller Scope) (*CapabilityResponse, error) {
	if r == nil {
		return nil, statusError{code: http.StatusNotImplemented, message: "capability registry is not available"}
	}
	cap, ok := r.capabilities[name]
	if !ok {
		return nil, statusError{code: http.StatusNotFound, message: "capability not found: " + name}
	}
	if !allows(cap.info, caller) {
		return nil, statusError{
			code:    http.StatusForbidden,
			message: fmt.Sprintf("capability %s is not available to %q callers", name, string(caller)),
		}
	}
	result, err := cap.invoke(ctx, req)
	if err != nil {
		return nil, err
	}
	if !cap.info.ReadOnly && r.recordMutation != nil {
		// The capability already ran, so failing the response here would tell the
		// caller nothing happened when something did. Log instead: a lost pending
		// marker misleads an operator later, and staying silent about it is how
		// that kind of state goes unnoticed for weeks.
		if recErr := r.recordMutation(name); recErr != nil {
			log.Printf("aegisgateway: record mutation for capability %s: %v", name, recErr)
		}
	}
	return &CapabilityResponse{Capability: name, Result: result}, nil
}

// allows reports whether a caller in the given scope may invoke this capability.
// An unrecognized caller scope matches nothing.
func allows(info Capability, caller Scope) bool {
	if !caller.Valid() {
		return false
	}
	for _, s := range info.Scopes {
		if s == caller {
			return true
		}
	}
	return false
}
