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

func TestSchedulerRespectsMaxParallelAndPassesTaskPolicies(t *testing.T) {
	q := bus.NewChannelQueue(100)
	defer q.Close()
	plan := &Plan{
		ID:   "p1",
		Goal: "ship it",
		Constraints: Constraints{
			MaxParallel:    1,
			TimeoutSeconds: 30,
		},
		Tasks: []TaskNode{
			{ID: "a", Expert: "architect", Input: TaskInput{Prompt: "do a"}, Retry: RetryPolicy{MaxAttempts: 3, BackoffSeconds: 2}},
			{ID: "b", Expert: "reviewer", Input: TaskInput{Prompt: "do b"}},
		},
	}
	scheduler := Scheduler{Queue: q}
	receipts, err := scheduler.EnqueueReady(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(receipts) != 1 || plan.State("b") != TaskStatusPending {
		t.Fatalf("receipts=%#v states=%#v", receipts, plan.TaskStates)
	}
	queued, ok, err := q.Task(TaskBusID(plan, "a"))
	if err != nil || !ok {
		t.Fatalf("queued task=%#v ok=%v err=%v", queued, ok, err)
	}
	if queued.MaxAttempts != 3 || queued.Payload["timeout_seconds"] != "30" || queued.Payload["retry_backoff_seconds"] != "2" {
		t.Fatalf("queued task policies=%#v", queued)
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
