// Package lifecycle models the generic lifecycle of a distributed node.
//
// DistNode owns node lifecycle declarations and readiness planning, but it does
// not own middleware implementations. Embedding projects may declare ingress,
// deployment, runtime, or profile facts; this package only combines those
// declarations into a consistent plan.
package lifecycle

type Status string

const (
	StatusUnknown  Status = "unknown"
	StatusReady    Status = "ready"
	StatusDegraded Status = "degraded"
	StatusMissing  Status = "missing"
	StatusBlocked  Status = "blocked"
)

type NodeRole string

const (
	RoleUnknown    NodeRole = "unknown"
	RoleController NodeRole = "controller"
	RoleTarget     NodeRole = "target"
	RoleWorker     NodeRole = "worker"
	RoleLocalPush  NodeRole = "local_push"
)

type EndpointKind string

const (
	EndpointDirect          EndpointKind = "direct"
	EndpointDeclaredIngress EndpointKind = "declared_ingress"
	EndpointTunnel          EndpointKind = "tunnel"
	EndpointLocalOnly       EndpointKind = "local_only"
)

type ActionLevel string

const (
	ActionSafeAuto     ActionLevel = "safe_auto"
	ActionNeedsConfirm ActionLevel = "needs_confirmation"
	ActionManualOnly   ActionLevel = "manual_only"
	ActionBlocked      ActionLevel = "blocked"
)

type ActionSource string

const (
	SourceRuntime  ActionSource = "runtime"
	SourceDeploy   ActionSource = "deploy"
	SourceIngress  ActionSource = "ingress"
	SourceProfile  ActionSource = "profile"
	SourceDistNode ActionSource = "distnode"
)

type NodeDeclaration struct {
	NodeID string   `json:"node_id,omitempty"`
	Name   string   `json:"name,omitempty"`
	Role   NodeRole `json:"role,omitempty"`
}

type RuntimeDeclaration struct {
	Status  Status `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Address string `json:"address,omitempty"`
}

type EndpointDeclaration struct {
	Name        string       `json:"name"`
	Kind        EndpointKind `json:"kind"`
	Status      Status       `json:"status"`
	PublicAddr  string       `json:"public_addr,omitempty"`
	LocalAddr   string       `json:"local_addr,omitempty"`
	DeclaredBy  string       `json:"declared_by,omitempty"`
	Constraints []string     `json:"constraints,omitempty"`
	Detail      string       `json:"detail,omitempty"`
}

type CapabilityDeclaration struct {
	Name       string            `json:"name"`
	Status     Status            `json:"status"`
	DeclaredBy string            `json:"declared_by,omitempty"`
	Detail     string            `json:"detail,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type Action struct {
	Name        string       `json:"name"`
	Source      ActionSource `json:"source"`
	Level       ActionLevel  `json:"level"`
	Description string       `json:"description,omitempty"`
	Commands    []string     `json:"commands,omitempty"`
}

type Acceptance string

const (
	AcceptanceReady   Acceptance = "ready"
	AcceptancePartial Acceptance = "partial"
	AcceptanceBlocked Acceptance = "blocked"
	AcceptanceUnknown Acceptance = "unknown"
)

type PlanInput struct {
	Node         NodeDeclaration         `json:"node"`
	Runtime      RuntimeDeclaration      `json:"runtime"`
	Endpoints    []EndpointDeclaration   `json:"endpoints,omitempty"`
	Capabilities []CapabilityDeclaration `json:"capabilities,omitempty"`
	Actions      []Action                `json:"actions,omitempty"`
}

type Plan struct {
	Node         NodeDeclaration         `json:"node"`
	Runtime      RuntimeDeclaration      `json:"runtime"`
	Endpoints    []EndpointDeclaration   `json:"endpoints,omitempty"`
	Capabilities []CapabilityDeclaration `json:"capabilities,omitempty"`
	Actions      []Action                `json:"actions,omitempty"`
	Acceptance   Acceptance              `json:"acceptance"`
	CanJoin      bool                    `json:"can_join"`
	CanServe     bool                    `json:"can_serve"`
	NextStep     string                  `json:"next_step,omitempty"`
}
