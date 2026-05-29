package orchestrator

import "testing"

func TestNewPlanCreatesSingleDelegateTask(t *testing.T) {
	plan := NewPlan(CreateOptions{ID: "p1", Goal: "ship feature", Expert: "reviewer"})
	if plan.ID != "p1" || plan.Goal != "ship feature" || len(plan.Tasks) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
	if plan.Tasks[0].Expert != "reviewer" || plan.Tasks[0].Input.Prompt != "ship feature" {
		t.Fatalf("task = %#v", plan.Tasks[0])
	}
	if err := ValidatePlan(plan); err != nil {
		t.Fatal(err)
	}
}
