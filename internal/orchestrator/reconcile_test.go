package orchestrator

import (
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/bus"
)

func TestReconcileEnqueuesDownstreamAndMarksDone(t *testing.T) {
	q := bus.NewChannelQueue(100)
	defer q.Close()
	plan := &Plan{
		ID:   "p1",
		Goal: "goal",
		Tasks: []TaskNode{
			{ID: "a", Expert: "architect", Input: TaskInput{Prompt: "a"}},
			{ID: "b", Expert: "reviewer", DependsOn: []string{"a"}, Input: TaskInput{Prompt: "b"}},
		},
	}
	first, err := Reconcile(plan, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Queued) != 1 || plan.State("a") != TaskStatusQueued || plan.State("b") != TaskStatusPending {
		t.Fatalf("first=%#v states=%#v", first, plan.TaskStates)
	}
	running, ok, err := q.Dequeue(bus.WorkerRef{ID: "w1", Queue: "experts"})
	if err != nil || !ok {
		t.Fatalf("dequeue ok=%v err=%v", ok, err)
	}
	if err := q.Ack(running.Task.ID, bus.TaskResult{Result: "a done"}); err != nil {
		t.Fatal(err)
	}
	second, err := Reconcile(plan, q)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Queued) != 1 || plan.State("a") != TaskStatusSucceeded || plan.State("b") != TaskStatusQueued {
		t.Fatalf("second=%#v states=%#v", second, plan.TaskStates)
	}
	running, ok, err = q.Dequeue(bus.WorkerRef{ID: "w2", Queue: "experts"})
	if err != nil || !ok {
		t.Fatalf("dequeue b ok=%v err=%v", ok, err)
	}
	if err := q.Ack(running.Task.ID, bus.TaskResult{Result: "b done"}); err != nil {
		t.Fatal(err)
	}
	done, err := Reconcile(plan, q)
	if err != nil {
		t.Fatal(err)
	}
	if !done.Complete || plan.Status != PlanStatusDone {
		t.Fatalf("done=%#v status=%s states=%#v", done, plan.Status, plan.TaskStates)
	}
	if plan.FinalOutput == "" || !contains(plan.FinalOutput, "a done") || !contains(plan.FinalOutput, "b done") {
		t.Fatalf("final output = %q", plan.FinalOutput)
	}
}

func contains(value, sub string) bool {
	return strings.Contains(value, sub)
}
