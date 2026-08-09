package tool

import (
	"context"
	"errors"
	"testing"
)

type recordingOrchestrator struct {
	ctx      context.Context
	planJSON string
}

func (o *recordingOrchestrator) SubmitPlan(ctx context.Context, planJSON string) (OrchestratorReceipt, error) {
	o.ctx = ctx
	o.planJSON = planJSON
	return OrchestratorReceipt{PlanID: "plan-1", Status: "queued", Queued: 2}, nil
}

func TestRunOrchestrateToolPropagatesContext(t *testing.T) {
	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "request-1")
	orchestrator := &recordingOrchestrator{}

	out, err := runOrchestrateTool(ctx, Dependencies{Orchestrator: orchestrator}, orchestrateInput{PlanJSON: `{"goal":"review"}`})
	if err != nil {
		t.Fatal(err)
	}
	if out.PlanID != "plan-1" || out.Status != "queued" || out.Queued != 2 {
		t.Fatalf("output = %#v", out)
	}
	if orchestrator.planJSON != `{"goal":"review"}` {
		t.Fatalf("plan JSON = %q", orchestrator.planJSON)
	}
	if got := orchestrator.ctx.Value(contextKey("request")); got != "request-1" {
		t.Fatalf("orchestrator context value = %v, want request-1", got)
	}
}

func TestRunOrchestrateToolHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	orchestrator := &recordingOrchestrator{}

	_, err := runOrchestrateTool(ctx, Dependencies{Orchestrator: orchestrator}, orchestrateInput{PlanJSON: `{}`})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if orchestrator.ctx != nil {
		t.Fatal("orchestrator should not be called after cancellation")
	}
}
