package aegisgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aegis/internal/action"
)

// TestScopeVocabularyMatchesActionTokenTypes is the guardrail for the whole
// scheme: capability scopes and auth token types are two spellings of one
// vocabulary, and nothing else in the build connects them.
//
// Both drift directions fail silently in production. Add a token type without a
// scope and that caller class is denied every capability. Add a scope no token
// type produces and the capability declaring it is reachable by nobody. Neither
// logs, neither errors — the feature simply does nothing, which is the hardest
// class of bug to attribute.
func TestScopeVocabularyMatchesActionTokenTypes(t *testing.T) {
	tokenTypes := discoverTokenTypes(t)
	if len(tokenTypes) == 0 {
		t.Fatal("found no Is<X>() predicates on action.ActionContext — discovery is broken, not the vocabulary")
	}

	for _, tokenType := range tokenTypes {
		scope, ok := ScopeFor(tokenType)
		if !ok {
			t.Errorf("token type %q has no capability scope: callers of that class are denied every capability. "+
				"Add a Scope%s constant to AllScopes() in capability.go, or drop the matching predicate.",
				tokenType, tokenType)
			continue
		}
		if string(scope) != tokenType {
			t.Errorf("ScopeFor(%q) = %q, want the same spelling", tokenType, scope)
		}
	}

	known := make(map[string]bool, len(tokenTypes))
	for _, tt := range tokenTypes {
		known[tt] = true
	}
	for _, scope := range AllScopes() {
		if !known[string(scope)] {
			t.Errorf("scope %q matches no auth token type: a capability declaring it is reachable by nobody", scope)
		}
	}
}

// discoverTokenTypes derives the caller classes action.ActionContext recognizes by
// reflecting over its Is<X>() bool predicates and confirming each one accepts the
// lowercased suffix as its token type.
//
// Reflection rather than a literal list on purpose. A hand-written table here
// would go stale the moment someone adds a caller class, which is precisely the
// drift this test exists to catch — the test would keep passing while the new
// class silently had access to nothing.
func discoverTokenTypes(t *testing.T) []string {
	t.Helper()
	acType := reflect.TypeOf(&action.ActionContext{})
	var out []string
	for i := 0; i < acType.NumMethod(); i++ {
		m := acType.Method(i)
		if !strings.HasPrefix(m.Name, "Is") || m.Name == "Is" {
			continue
		}
		if m.Type.NumIn() != 1 || m.Type.NumOut() != 1 || m.Type.Out(0).Kind() != reflect.Bool {
			continue
		}
		tokenType := strings.ToLower(strings.TrimPrefix(m.Name, "Is"))
		ac := &action.ActionContext{TokenType: tokenType}
		if !m.Func.Call([]reflect.Value{reflect.ValueOf(ac)})[0].Bool() {
			t.Errorf("action.ActionContext.%s() rejected TokenType=%q; this test assumes Is<X> matches the lowercased suffix",
				m.Name, tokenType)
			continue
		}
		out = append(out, tokenType)
	}
	return out
}

// TestScopeForRejectsUnknownTokenType pins the fail-closed direction. A caller
// class added to the auth layer must not inherit access to existing capabilities
// by defaulting into some scope.
func TestScopeForRejectsUnknownTokenType(t *testing.T) {
	for _, tokenType := range []string{"", "root", "Admin", "operator"} {
		if scope, ok := ScopeFor(tokenType); ok {
			t.Errorf("ScopeFor(%q) = %q, want rejected", tokenType, scope)
		}
	}
}

// TestRegisterRejectsCapabilityWithoutScopes is the reason this scheme has teeth.
// Before scopes were enforced, a capability could declare any scope list — or
// none — and be invocable by every authenticated caller regardless. Registration
// now refuses, so the failure lands at startup instead of at the security
// boundary.
func TestRegisterRejectsCapabilityWithoutScopes(t *testing.T) {
	reg := NewCapabilityRegistry()
	err := reg.Register(Capability{Name: "no.scopes", ReadOnly: true}, noopInvoker)
	if err == nil {
		t.Fatal("registered a capability with no scopes — it would be unenforceable")
	}
	if len(reg.List(ScopeService)) != 0 {
		t.Error("rejected capability was still added to the registry")
	}
}

// TestRegisterRejectsUnknownScope catches the typo case: "services" instead of
// "service" would otherwise register fine and deny every real caller.
func TestRegisterRejectsUnknownScope(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{
		Name:     "typo.scope",
		ReadOnly: true,
		Scopes:   []Scope{"services"},
	}, noopInvoker); err == nil {
		t.Fatal("registered a capability with an unknown scope")
	}
}

// TestRegisterRejectsMutatingCapabilityWithoutRecorder pins the other half of the
// declaration contract. ReadOnly=false is a claim that the capability changes
// cluster state; without a recorder that claim has no consequence, and the
// mutation leaves no pending marker for an operator to find.
func TestRegisterRejectsMutatingCapabilityWithoutRecorder(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{
		Name:     "node.drain",
		ReadOnly: false,
		Scopes:   []Scope{ScopeAdmin},
	}, noopInvoker); err == nil {
		t.Fatal("registered a mutating capability with no way to record the mutation")
	}
}

// TestMutatingCapabilityRecordsPendingApply is the positive case: ReadOnly=false
// now has an observable effect.
func TestMutatingCapabilityRecordsPendingApply(t *testing.T) {
	var recorded []string
	reg := NewCapabilityRegistry(WithMutationRecorder(func(name string) error {
		recorded = append(recorded, name)
		return nil
	}))
	if err := reg.Register(Capability{
		Name:     "node.drain",
		ReadOnly: false,
		Scopes:   []Scope{ScopeAdmin},
	}, noopInvoker); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.Invoke(context.Background(), "node.drain", CapabilityRequest{}, ScopeAdmin); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if !reflect.DeepEqual(recorded, []string{"node.drain"}) {
		t.Errorf("recorded = %v, want the mutating capability name", recorded)
	}
}

// TestReadOnlyCapabilityRecordsNothing guards the inverse: a read-only capability
// must not manufacture pending-apply state. A marker with no change behind it
// tells an operator an apply is outstanding when none is.
func TestReadOnlyCapabilityRecordsNothing(t *testing.T) {
	recorded := 0
	reg := NewCapabilityRegistry(WithMutationRecorder(func(string) error {
		recorded++
		return nil
	}))
	if err := reg.Register(Capability{
		Name:     "node.list",
		ReadOnly: true,
		Scopes:   []Scope{ScopeService},
	}, noopInvoker); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.Invoke(context.Background(), "node.list", CapabilityRequest{}, ScopeService); err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if recorded != 0 {
		t.Errorf("read-only capability recorded %d mutations, want 0", recorded)
	}
}

// TestFailedMutationDoesNotRecord pins that the marker follows the actual state
// change, not the attempt.
func TestFailedMutationDoesNotRecord(t *testing.T) {
	recorded := 0
	reg := NewCapabilityRegistry(WithMutationRecorder(func(string) error {
		recorded++
		return nil
	}))
	if err := reg.Register(Capability{
		Name:     "node.drain",
		ReadOnly: false,
		Scopes:   []Scope{ScopeAdmin},
	}, func(context.Context, CapabilityRequest) (interface{}, error) {
		return nil, statusError{code: http.StatusBadGateway, message: "drain failed"}
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.Invoke(context.Background(), "node.drain", CapabilityRequest{}, ScopeAdmin); err == nil {
		t.Fatal("invoke succeeded, want the invoker's error")
	}
	if recorded != 0 {
		t.Errorf("a failed mutation recorded %d markers, want 0", recorded)
	}
}

// TestInvokeDeniesCallerOutsideScope is the enforcement point that did not exist
// before: node.list declared Scopes:["service"] and was invocable by any
// authenticated caller.
func TestInvokeDeniesCallerOutsideScope(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{
		Name:     "service.only",
		ReadOnly: true,
		Scopes:   []Scope{ScopeService},
	}, noopInvoker); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, err := reg.Invoke(context.Background(), "service.only", CapabilityRequest{}, ScopeSpace); err == nil {
		t.Fatal("a space caller invoked a service-only capability")
	} else if got := ErrorStatus(err); got != http.StatusForbidden {
		t.Errorf("status = %d, want %d — a scope miss is a permission answer, not a missing route", got, http.StatusForbidden)
	}

	if _, err := reg.Invoke(context.Background(), "service.only", CapabilityRequest{}, ScopeService); err != nil {
		t.Errorf("the declared scope was denied: %v", err)
	}
}

// TestInvokeDeniesUnrecognizedCallerScope covers the empty scope that callerScope
// returns for an unauthenticated or unknown caller.
func TestInvokeDeniesUnrecognizedCallerScope(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{
		Name:     "any",
		ReadOnly: true,
		Scopes:   AllScopes(),
	}, noopInvoker); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := reg.Invoke(context.Background(), "any", CapabilityRequest{}, Scope("")); err == nil {
		t.Fatal("an unscoped caller invoked a capability open to all known scopes")
	}
}

// TestListHidesCapabilitiesOutsideCallerScope keeps the listing and the gate
// consistent. A capability a caller cannot invoke must not appear in its list,
// or the SDK advertises calls that always fail.
func TestListHidesCapabilitiesOutsideCallerScope(t *testing.T) {
	reg := NewCapabilityRegistry()
	for _, c := range []Capability{
		{Name: "service.thing", ReadOnly: true, Scopes: []Scope{ScopeService}},
		{Name: "admin.thing", ReadOnly: true, Scopes: []Scope{ScopeAdmin}},
	} {
		if err := reg.Register(c, noopInvoker); err != nil {
			t.Fatalf("register %s: %v", c.Name, err)
		}
	}

	for caller, want := range map[Scope]string{
		ScopeService: "service.thing",
		ScopeAdmin:   "admin.thing",
	} {
		list := reg.List(caller)
		if len(list) != 1 || list[0].Name != want {
			t.Errorf("List(%q) = %+v, want only %s", caller, list, want)
		}
	}
	if list := reg.List(ScopeSpace); len(list) != 0 {
		t.Errorf("List(space) = %+v, want empty", list)
	}
}

// TestListedCapabilitiesAreInvocable is the completeness check across the real
// registry: everything a caller is shown must actually be callable by that
// caller. It fails if a future capability is registered with scopes that List and
// Invoke disagree about.
func TestListedCapabilitiesAreInvocable(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := RegisterNodeCapabilities(reg, fakeNodeLister{}); err != nil {
		t.Fatalf("RegisterNodeCapabilities: %v", err)
	}

	seen := 0
	for _, caller := range AllScopes() {
		for _, c := range reg.List(caller) {
			seen++
			if _, err := reg.Invoke(context.Background(), c.Name, CapabilityRequest{Input: json.RawMessage(`{}`)}, caller); err != nil {
				t.Errorf("capability %s is listed for %q but denied on invoke: %v", c.Name, caller, err)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no capability was listed for any scope — the registry is unreachable")
	}
}

func noopInvoker(context.Context, CapabilityRequest) (interface{}, error) { return nil, nil }
