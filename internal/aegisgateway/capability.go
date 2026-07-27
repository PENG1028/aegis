package aegisgateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

type Capability struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ReadOnly    bool     `json:"read_only"`
	Scopes      []string `json:"scopes,omitempty"`
}

type CapabilityRequest struct {
	Input json.RawMessage `json:"input,omitempty"`
}

type CapabilityResponse struct {
	Capability string      `json:"capability"`
	Result     interface{} `json:"result,omitempty"`
}

type CapabilityInvoker func(ctx context.Context, req CapabilityRequest) (interface{}, error)

type CapabilityRegistry struct {
	capabilities map[string]registeredCapability
}

type registeredCapability struct {
	info   Capability
	invoke CapabilityInvoker
}

func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{capabilities: make(map[string]registeredCapability)}
}

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
	r.capabilities[info.Name] = registeredCapability{info: info, invoke: invoke}
	return nil
}

func (r *CapabilityRegistry) List() []Capability {
	if r == nil {
		return []Capability{}
	}
	out := make([]Capability, 0, len(r.capabilities))
	for _, cap := range r.capabilities {
		out = append(out, cap.info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *CapabilityRegistry) Invoke(ctx context.Context, name string, req CapabilityRequest) (*CapabilityResponse, error) {
	if r == nil {
		return nil, statusError{code: http.StatusNotImplemented, message: "capability registry is not available"}
	}
	cap, ok := r.capabilities[name]
	if !ok {
		return nil, statusError{code: http.StatusNotFound, message: "capability not found: " + name}
	}
	result, err := cap.invoke(ctx, req)
	if err != nil {
		return nil, err
	}
	return &CapabilityResponse{Capability: name, Result: result}, nil
}
