// Package onboarding defines the generic node ensure workflow used by DistNode
// integrations.
//
// The package intentionally models the flow and report shape, not a specific
// product's install details. Embedding projects provide the concrete adapter:
// artifact selection, config rendering, service commands, and post-join hooks.
package onboarding

import (
	"context"
	"fmt"
)

// Mode selects how EnsureNode should treat an existing target.
type Mode string

const (
	ModeAuto     Mode = "auto"
	ModeJoinOnly Mode = "join_only"
	ModeDeploy   Mode = "deploy"
	ModeRedeploy Mode = "redeploy"
)

// StepStatus is the UI/API friendly outcome of a workflow step.
type StepStatus string

const (
	StepPending StepStatus = "pending"
	StepSkipped StepStatus = "skipped"
	StepOK      StepStatus = "ok"
	StepWarning StepStatus = "warning"
	StepFailed  StepStatus = "failed"
)

// StepReport records one pipeline stage.
type StepReport struct {
	Name    string     `json:"name"`
	Status  StepStatus `json:"status"`
	Message string     `json:"message,omitempty"`
}

// Capability describes what the running environment or target can do.
type Capability struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Detail    string `json:"detail,omitempty"`
}

// EnsureResult is the canonical result of adding or preparing a node.
type EnsureResult struct {
	Success      bool         `json:"success"`
	Action       string       `json:"action,omitempty"`
	NodeID       string       `json:"node_id,omitempty"`
	PeerAddr     string       `json:"peer_addr,omitempty"`
	Message      string       `json:"message,omitempty"`
	NextStep     string       `json:"next_step,omitempty"`
	Steps        []StepReport `json:"steps,omitempty"`
	Capabilities []Capability `json:"capabilities,omitempty"`
}

// AddStep appends a step outcome and returns the result for fluent use.
func (r *EnsureResult) AddStep(name string, status StepStatus, message string) *EnsureResult {
	r.Steps = append(r.Steps, StepReport{Name: name, Status: status, Message: message})
	return r
}

// StepFunc performs one onboarding stage.
type StepFunc func(ctx context.Context, result *EnsureResult) error

// Step is one named onboarding stage.
type Step struct {
	Name string
	Run  StepFunc
}

// Pipeline executes onboarding stages in order and stops at the first failure.
type Pipeline struct {
	Steps []Step
}

// Run executes the pipeline into result. If result is nil, Run creates one.
func (p Pipeline) Run(ctx context.Context, result *EnsureResult) *EnsureResult {
	if result == nil {
		result = &EnsureResult{}
	}
	for _, step := range p.Steps {
		if step.Name == "" {
			step.Name = "unnamed"
		}
		if step.Run == nil {
			result.AddStep(step.Name, StepSkipped, "no step function")
			continue
		}
		if err := step.Run(ctx, result); err != nil {
			result.AddStep(step.Name, StepFailed, err.Error())
			if result.Message == "" {
				result.Message = fmt.Sprintf("%s failed: %v", step.Name, err)
			}
			result.Success = false
			return result
		}
		if !hasStep(result, step.Name) {
			result.AddStep(step.Name, StepOK, "")
		}
	}
	result.Success = true
	return result
}

func hasStep(result *EnsureResult, name string) bool {
	for _, step := range result.Steps {
		if step.Name == name {
			return true
		}
	}
	return false
}
