package tool

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

type orchestrateInput struct {
	PlanJSON string `json:"plan_json" jsonschema:"DAG plan JSON to enqueue"`
}

type orchestrateOutput struct {
	PlanID string `json:"plan_id"`
	Status string `json:"status"`
	Queued int    `json:"queued"`
}

type Orchestrator interface {
	SubmitPlan(ctx context.Context, planJSON string) (OrchestratorReceipt, error)
}

type OrchestratorReceipt struct {
	PlanID string `json:"plan_id"`
	Status string `json:"status"`
	Queued int    `json:"queued"`
}

func newOrchestrateTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input orchestrateInput) (orchestrateOutput, error) {
		return runOrchestrateTool(tctx, deps, input)
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "orchestrate",
		Description: "Submit a DAG plan JSON to the Superman orchestrator. Use this for complex multi-expert tasks after planning dependencies.",
	}, handler)
	return t
}

func runOrchestrateTool(ctx context.Context, deps Dependencies, input orchestrateInput) (orchestrateOutput, error) {
	if deps.Orchestrator == nil {
		return orchestrateOutput{}, fmt.Errorf("orchestrator not available")
	}
	if err := ctx.Err(); err != nil {
		return orchestrateOutput{}, err
	}
	receipt, err := deps.Orchestrator.SubmitPlan(ctx, input.PlanJSON)
	if err != nil {
		return orchestrateOutput{}, err
	}
	return orchestrateOutput{PlanID: receipt.PlanID, Status: receipt.Status, Queued: receipt.Queued}, nil
}
