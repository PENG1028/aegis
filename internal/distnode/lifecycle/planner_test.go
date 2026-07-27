package lifecycle

import "testing"

func TestBuildPlanReadyFromDeclaredIngress(t *testing.T) {
	plan := BuildPlan(PlanInput{
		Node:    NodeDeclaration{NodeID: "node-a", Role: RoleWorker},
		Runtime: RuntimeDeclaration{Status: StatusReady, Address: "127.0.0.1:17380"},
		Endpoints: []EndpointDeclaration{{
			Name:       "public-http",
			Kind:       EndpointDeclaredIngress,
			Status:     StatusReady,
			PublicAddr: "43.159.34.11:80",
			LocalAddr:  "127.0.0.1:17380",
			DeclaredBy: "aegis-provider",
		}},
	})

	if plan.Acceptance != AcceptanceReady {
		t.Fatalf("acceptance = %q, want ready", plan.Acceptance)
	}
	if !plan.CanJoin || !plan.CanServe {
		t.Fatalf("CanJoin=%v CanServe=%v, want both true", plan.CanJoin, plan.CanServe)
	}
}

func TestBuildPlanLocalPushNodeCanJoinButCannotServe(t *testing.T) {
	plan := BuildPlan(PlanInput{
		Node:    NodeDeclaration{NodeID: "local-laptop", Role: RoleLocalPush},
		Runtime: RuntimeDeclaration{Status: StatusReady, Address: "127.0.0.1:17380"},
		Endpoints: []EndpointDeclaration{{
			Name:      "local-ui",
			Kind:      EndpointLocalOnly,
			Status:    StatusReady,
			LocalAddr: "127.0.0.1:17380",
		}},
	})

	if plan.Acceptance != AcceptancePartial {
		t.Fatalf("acceptance = %q, want partial", plan.Acceptance)
	}
	if !plan.CanJoin {
		t.Fatal("CanJoin = false, want true")
	}
	if plan.CanServe {
		t.Fatal("CanServe = true, want false for local-only endpoint")
	}
}

func TestBuildPlanBlockedActionPreventsJoin(t *testing.T) {
	plan := BuildPlan(PlanInput{
		Runtime: RuntimeDeclaration{Status: StatusReady, Address: "127.0.0.1:17380"},
		Endpoints: []EndpointDeclaration{{
			Name:       "public-http",
			Kind:       EndpointDeclaredIngress,
			Status:     StatusReady,
			PublicAddr: "43.159.34.11:80",
		}},
		Actions: []Action{{
			Name:        "resolve_port_owner",
			Source:      SourceIngress,
			Level:       ActionBlocked,
			Description: "external ingress declaration reports a blocked port",
		}},
	})

	if plan.Acceptance != AcceptanceBlocked {
		t.Fatalf("acceptance = %q, want blocked", plan.Acceptance)
	}
	if plan.CanJoin || plan.CanServe {
		t.Fatalf("CanJoin=%v CanServe=%v, want both false", plan.CanJoin, plan.CanServe)
	}
}

func TestBuildPlanMissingRuntimeNeedsRuntimeBeforeEndpoint(t *testing.T) {
	plan := BuildPlan(PlanInput{
		Runtime: RuntimeDeclaration{Status: StatusMissing},
		Endpoints: []EndpointDeclaration{{
			Name:       "public-http",
			Kind:       EndpointDeclaredIngress,
			Status:     StatusReady,
			PublicAddr: "43.159.34.11:80",
		}},
	})

	if plan.Acceptance != AcceptanceBlocked {
		t.Fatalf("acceptance = %q, want blocked", plan.Acceptance)
	}
	if plan.NextStep != "install or start the node runtime" {
		t.Fatalf("next step = %q", plan.NextStep)
	}
}

func TestBuildPlanDefaultsUnknownDeclarations(t *testing.T) {
	plan := BuildPlan(PlanInput{})

	if plan.Node.Role != RoleUnknown {
		t.Fatalf("role = %q, want unknown", plan.Node.Role)
	}
	if plan.Runtime.Status != StatusUnknown {
		t.Fatalf("runtime status = %q, want unknown", plan.Runtime.Status)
	}
	if plan.Acceptance != AcceptanceUnknown {
		t.Fatalf("acceptance = %q, want unknown", plan.Acceptance)
	}
}
