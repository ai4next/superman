package orchestrator

import (
	"testing"

	"github.com/ai4next/superman/internal/bus"
)

func TestValidatePlanRejectsCycles(t *testing.T) {
	plan := Plan{
		ID:   "p1",
		Goal: "test",
		Tasks: []TaskNode{
			{ID: "a", Expert: "architect", DependsOn: []string{"b"}, Input: TaskInput{Prompt: "a"}},
			{ID: "b", Expert: "architect", DependsOn: []string{"a"}, Input: TaskInput{Prompt: "b"}},
		},
	}
	if err := ValidatePlan(plan); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestSchedulerEnqueuesOnlyReadyTasks(t *testing.T) {
	q := bus.NewChannelQueue(100)
	defer q.Close()
	plan := &Plan{
		ID:   "p1",
		Goal: "ship it",
		Tasks: []TaskNode{
			{ID: "a", Title: "A", Expert: "architect", Input: TaskInput{Prompt: "do a"}},
			{ID: "b", Title: "B", Expert: "reviewer", DependsOn: []string{"a"}, Input: TaskInput{Prompt: "do b"}},
		},
	}
	scheduler := Scheduler{Queue: q}
	receipts, err := scheduler.EnqueueReady(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || plan.State("a") != TaskStatusQueued || plan.State("b") != TaskStatusPending {
		t.Fatalf("receipts=%#v states=%#v", receipts, plan.TaskStates)
	}
	if TaskBusID(plan, "a") == "" {
		t.Fatal("missing bus task id")
	}

	MarkSucceeded(plan, "a")
	receipts, err = scheduler.EnqueueReady(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || plan.State("b") != TaskStatusQueued {
		t.Fatalf("receipts=%#v states=%#v", receipts, plan.TaskStates)
	}
}
