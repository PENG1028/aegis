package onboarding

import (
	"context"
	"errors"
	"testing"
)

func TestPipelineRunsStepsInOrder(t *testing.T) {
	var order []string
	result := Pipeline{Steps: []Step{
		{Name: "connect", Run: func(ctx context.Context, result *EnsureResult) error {
			order = append(order, "connect")
			return nil
		}},
		{Name: "join", Run: func(ctx context.Context, result *EnsureResult) error {
			order = append(order, "join")
			result.Action = "joined"
			return nil
		}},
	}}.Run(context.Background(), nil)

	if !result.Success {
		t.Fatalf("Success = false, want true: %#v", result)
	}
	if result.Action != "joined" {
		t.Fatalf("Action = %q, want joined", result.Action)
	}
	if len(order) != 2 || order[0] != "connect" || order[1] != "join" {
		t.Fatalf("order = %#v, want connect then join", order)
	}
	if len(result.Steps) != 2 || result.Steps[0].Status != StepOK || result.Steps[1].Status != StepOK {
		t.Fatalf("steps = %#v, want two ok steps", result.Steps)
	}
}

func TestPipelineStopsOnFailure(t *testing.T) {
	var ranAfter bool
	result := Pipeline{Steps: []Step{
		{Name: "connect", Run: func(ctx context.Context, result *EnsureResult) error {
			return errors.New("dial failed")
		}},
		{Name: "deploy", Run: func(ctx context.Context, result *EnsureResult) error {
			ranAfter = true
			return nil
		}},
	}}.Run(context.Background(), &EnsureResult{Action: string(ModeDeploy)})

	if result.Success {
		t.Fatal("Success = true, want false")
	}
	if ranAfter {
		t.Fatal("pipeline should stop after failed step")
	}
	if len(result.Steps) != 1 || result.Steps[0].Name != "connect" || result.Steps[0].Status != StepFailed {
		t.Fatalf("steps = %#v, want failed connect only", result.Steps)
	}
}

func TestPipelineKeepsExplicitStepStatus(t *testing.T) {
	result := Pipeline{Steps: []Step{
		{Name: "target_apply", Run: func(ctx context.Context, result *EnsureResult) error {
			result.AddStep("target_apply", StepWarning, "HTTP 000")
			return nil
		}},
	}}.Run(context.Background(), nil)

	if !result.Success {
		t.Fatal("Success = false, want true")
	}
	if len(result.Steps) != 1 {
		t.Fatalf("steps = %d, want 1", len(result.Steps))
	}
	if result.Steps[0].Status != StepWarning {
		t.Fatalf("status = %q, want warning", result.Steps[0].Status)
	}
}
