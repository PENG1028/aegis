package aegisgateway

import (
	"context"

	"aegis/internal/node"
)

type NodeLister interface {
	ListAll() ([]node.NodeRecord, error)
}

func RegisterNodeCapabilities(reg *CapabilityRegistry, nodes NodeLister) error {
	if nodes == nil {
		return nil
	}
	return reg.Register(Capability{
		Name:        "node.list",
		Description: "List known Aegis nodes",
		ReadOnly:    true,
		Scopes:      []string{"service"},
	}, func(context.Context, CapabilityRequest) (interface{}, error) {
		list, err := nodes.ListAll()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"nodes": list,
			"count": len(list),
		}, nil
	})
}
