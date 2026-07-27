package lifecycle

func BuildPlan(input PlanInput) Plan {
	plan := Plan{
		Node:         normalizeNode(input.Node),
		Runtime:      normalizeRuntime(input.Runtime),
		Endpoints:    input.Endpoints,
		Capabilities: input.Capabilities,
		Actions:      input.Actions,
		Acceptance:   AcceptanceUnknown,
	}

	hasBlockedAction := hasActionLevel(plan.Actions, ActionBlocked)
	hasRunnableRuntime := plan.Runtime.Status == StatusReady || plan.Runtime.Status == StatusDegraded
	hasReadyEndpoint := hasServableEndpoint(plan.Endpoints)
	hasLocalOnlyEndpoint := hasEndpointKind(plan.Endpoints, EndpointLocalOnly)

	plan.CanJoin = hasRunnableRuntime && !hasBlockedAction
	plan.CanServe = hasRunnableRuntime && hasReadyEndpoint && !hasBlockedAction

	switch {
	case hasBlockedAction || plan.Runtime.Status == StatusBlocked:
		plan.Acceptance = AcceptanceBlocked
		plan.NextStep = "resolve blocked actions before joining this node"
	case plan.CanServe:
		plan.Acceptance = AcceptanceReady
		plan.NextStep = "node is ready"
	case plan.CanJoin && hasLocalOnlyEndpoint:
		plan.Acceptance = AcceptancePartial
		plan.NextStep = "node can join, but only local access is declared"
	case plan.CanJoin:
		plan.Acceptance = AcceptancePartial
		plan.NextStep = "node can join, but no ready service endpoint is declared"
	case plan.Runtime.Status == StatusMissing:
		plan.Acceptance = AcceptanceBlocked
		plan.NextStep = "install or start the node runtime"
	default:
		plan.Acceptance = AcceptanceUnknown
		plan.NextStep = "declare runtime and endpoint readiness"
	}

	return plan
}

func normalizeNode(node NodeDeclaration) NodeDeclaration {
	if node.Role == "" {
		node.Role = RoleUnknown
	}
	return node
}

func normalizeRuntime(runtime RuntimeDeclaration) RuntimeDeclaration {
	if runtime.Status == "" {
		runtime.Status = StatusUnknown
	}
	return runtime
}

func hasActionLevel(actions []Action, level ActionLevel) bool {
	for _, action := range actions {
		if action.Level == level {
			return true
		}
	}
	return false
}

func hasEndpointStatus(endpoints []EndpointDeclaration, status Status) bool {
	for _, endpoint := range endpoints {
		if endpoint.Status == status {
			return true
		}
	}
	return false
}

func hasServableEndpoint(endpoints []EndpointDeclaration) bool {
	for _, endpoint := range endpoints {
		if endpoint.Status != StatusReady {
			continue
		}
		if endpoint.Kind == EndpointDirect || endpoint.Kind == EndpointDeclaredIngress || endpoint.Kind == EndpointTunnel {
			return true
		}
	}
	return false
}

func hasEndpointKind(endpoints []EndpointDeclaration, kind EndpointKind) bool {
	for _, endpoint := range endpoints {
		if endpoint.Kind == kind {
			return true
		}
	}
	return false
}
