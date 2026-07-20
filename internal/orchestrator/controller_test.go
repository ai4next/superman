package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/bus"
)

func TestControllerAdvancesPlanWhenTaskCompletes(t *testing.T) {
	queue := bus.NewChannelQueue(10)
	defer queue.Close()
	store := FileStore{Dir: t.TempDir()}
	controller := NewController(store, queue)
	plan := Plan{
		ID:   "p1",
		Goal: "finish work",
		Tasks: []TaskNode{
			{ID: "a", Expert: "architect", Input: TaskInput{Prompt: "design"}},
			{ID: "b", Expert: "reviewer", DependsOn: []string{"a"}, Input: TaskInput{Prompt: "review"}},
		},
	}
	if result, err := controller.Submit(plan); err != nil || len(result.Queued) != 1 {
		t.Fatalf("submit result=%#v err=%v", result, err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		controller.Run(ctx, 5*time.Millisecond)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	first, ok, err := queue.Dequeue(bus.WorkerRef{ID: "worker-a", Queue: "experts"})
	if err != nil || !ok || first.Task.ID != "p1:a" {
		t.Fatalf("first task=%#v ok=%v err=%v", first, ok, err)
	}
	if err := queue.Ack(first.Task.ID, bus.TaskResult{Result: "design done"}); err != nil {
		t.Fatal(err)
	}

	second := waitForTask(t, queue, "p1:b")
	if err := queue.Ack(second.Task.ID, bus.TaskResult{Result: "review done"}); err != nil {
		t.Fatal(err)
	}
	waitForPlanStatus(t, store, "p1", PlanStatusDone)

	stored, err := store.Load("p1")
	if err != nil {
		t.Fatal(err)
	}
	if stored.State("a") != TaskStatusSucceeded || stored.State("b") != TaskStatusSucceeded || stored.FinalOutput == "" {
		t.Fatalf("stored plan = %#v", stored)
	}
}

func waitForTask(t *testing.T, queue bus.TaskQueue, taskID string) bus.RunningTask {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		running, ok, err := queue.Dequeue(bus.WorkerRef{ID: "worker-b", Queue: "experts"})
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			if running.Task.ID != taskID {
				t.Fatalf("task = %q, want %q", running.Task.ID, taskID)
			}
			return running
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for task %q", taskID)
	return bus.RunningTask{}
}

func waitForPlanStatus(t *testing.T, store FileStore, planID string, status PlanStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		plan, err := store.Load(planID)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Status == status {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	plan, err := store.Load(planID)
	if err != nil {
		t.Fatal(err)
	}
	t.Fatalf("plan status = %q, want %q", plan.Status, status)
}

func TestControllerRequeuesPersistedTaskWhenQueueWasRestarted(t *testing.T) {
	store := FileStore{Dir: t.TempDir()}
	plan := Plan{
		ID:   "p1",
		Goal: "recover",
		Tasks: []TaskNode{{
			ID:       "a",
			Expert:   "architect",
			Input:    TaskInput{Prompt: "recover work"},
			Metadata: map[string]string{"bus_task_id": "p1:a"},
		}},
		Status:     PlanStatusRunning,
		TaskStates: map[string]TaskStatus{"a": TaskStatusQueued},
	}
	if err := store.Save(plan); err != nil {
		t.Fatal(err)
	}
	queue := bus.NewChannelQueue(10)
	defer queue.Close()
	if err := NewController(store, queue).ReconcileActive(); err != nil {
		t.Fatal(err)
	}
	if task, ok, err := queue.Task("p1:a"); err != nil || !ok || task.Status != bus.TaskStatusReady {
		t.Fatalf("recovered task=%#v ok=%v err=%v", task, ok, err)
	}
}
