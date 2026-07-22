package aegisgateway

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"aegis/internal/node"
)

type fakeNodeLister struct {
	nodes []node.NodeRecord
	err   error
}

func (l fakeNodeLister) ListAll() ([]node.NodeRecord, error) {
	return l.nodes, l.err
}

func TestCapabilityRegistryListsCapabilitiesInNameOrder(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{Name: "z.capability"}, func(context.Context, CapabilityRequest) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register z: %v", err)
	}
	if err := reg.Register(Capability{Name: "a.capability"}, func(context.Context, CapabilityRequest) (interface{}, error) {
		return nil, nil
	}); err != nil {
		t.Fatalf("register a: %v", err)
	}

	list := reg.List()
	if len(list) != 2 {
		t.Fatalf("len = %d, want 2", len(list))
	}
	if list[0].Name != "a.capability" || list[1].Name != "z.capability" {
		t.Fatalf("capabilities not sorted: %+v", list)
	}
}

func TestCapabilityRegistryInvokesCapability(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := reg.Register(Capability{Name: "echo"}, func(_ context.Context, req CapabilityRequest) (interface{}, error) {
		return string(req.Input), nil
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	resp, err := reg.Invoke(context.Background(), "echo", CapabilityRequest{Input: json.RawMessage(`{"ok":true}`)})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if resp.Capability != "echo" || resp.Result != `{"ok":true}` {
		t.Fatalf("response = %+v", resp)
	}
}

func TestCapabilityRegistryMissingCapability(t *testing.T) {
	_, err := NewCapabilityRegistry().Invoke(context.Background(), "missing", CapabilityRequest{})
	if err == nil {
		t.Fatal("Invoke succeeded, want missing capability error")
	}
	if ErrorStatus(err) != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", ErrorStatus(err), http.StatusNotFound)
	}
}

func TestRegisterNodeCapabilities(t *testing.T) {
	reg := NewCapabilityRegistry()
	if err := RegisterNodeCapabilities(reg, fakeNodeLister{nodes: []node.NodeRecord{
		{NodeID: "node-a", Name: "alpha"},
	}}); err != nil {
		t.Fatalf("RegisterNodeCapabilities: %v", err)
	}

	list := reg.List()
	if len(list) != 1 || list[0].Name != "node.list" || !list[0].ReadOnly {
		t.Fatalf("capabilities = %+v", list)
	}
	resp, err := reg.Invoke(context.Background(), "node.list", CapabilityRequest{})
	if err != nil {
		t.Fatalf("Invoke node.list: %v", err)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("result type = %T", resp.Result)
	}
	if result["count"] != 1 {
		t.Fatalf("result = %+v", result)
	}
}
