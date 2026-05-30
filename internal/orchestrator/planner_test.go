package orchestrator

import (
	"context"
	"iter"
	"testing"

	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

type plannerFakeLLM struct {
	text string
}

func (p plannerFakeLLM) Name() string { return "planner-fake" }

func (p plannerFakeLLM) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{Content: genai.NewContentFromText(p.text, genai.RoleModel)}, nil)
	}
}

func TestParsePlanJSONStripsFence(t *testing.T) {
	plan, err := ParsePlanJSON("```json\n{\"plan_id\":\"p1\",\"goal\":\"g\",\"tasks\":[{\"id\":\"t1\",\"expert\":\"e\",\"input\":{\"prompt\":\"do\"}}]}\n```")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID != "p1" || len(plan.Tasks) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
}

func TestGeneratePlanFromLLM(t *testing.T) {
	llm := plannerFakeLLM{text: `{"plan_id":"p1","goal":"g","tasks":[{"id":"t1","expert":"reviewer","input":{"prompt":"do"}}]}`}
	plan, err := GeneratePlan(context.Background(), llm, PlannerOptions{Goal: "g", Experts: []string{"reviewer"}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.ID != "p1" || plan.Tasks[0].Expert != "reviewer" {
		t.Fatalf("plan = %#v", plan)
	}
}
