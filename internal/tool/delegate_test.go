package tool

import (
	"context"
	"testing"
)

type recordingDelegateRunner struct {
	expert string
	task   string
}

func (r *recordingDelegateRunner) RunDelegate(ctx context.Context, expertName string, task string) (string, error) {
	r.expert = expertName
	r.task = task
	return "sync result", nil
}

type recordingDelegateScheduler struct {
	req DelegateTaskRequest
}

func (s *recordingDelegateScheduler) EnqueueDelegate(ctx context.Context, req DelegateTaskRequest) (DelegateTaskReceipt, error) {
	s.req = req
	return DelegateTaskReceipt{TaskID: "task-123", Status: "queued"}, nil
}

func TestRunDelegateToolDefaultsToSync(t *testing.T) {
	runner := &recordingDelegateRunner{}
	out, err := runDelegateTool(context.Background(), Dependencies{DelegateRunner: runner}, delegateInput{
		ExpertName: "architect",
		Task:       "design this",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Response != "sync result" || out.Status != "succeeded" {
		t.Fatalf("output = %#v", out)
	}
	if runner.expert != "architect" || runner.task != "design this" {
		t.Fatalf("runner call = expert:%q task:%q", runner.expert, runner.task)
	}
}

func TestRunDelegateToolAsyncEnqueuesTask(t *testing.T) {
	scheduler := &recordingDelegateScheduler{}
	out, err := runDelegateTool(context.Background(), Dependencies{DelegateScheduler: scheduler}, delegateInput{
		ExpertName: "reviewer",
		Task:       "review this",
		Mode:       "async",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.TaskID != "task-123" || out.Status != "queued" || out.Response != "" {
		t.Fatalf("output = %#v", out)
	}
	if scheduler.req.ExpertName != "reviewer" || scheduler.req.Task != "review this" {
		t.Fatalf("scheduler req = %#v", scheduler.req)
	}
}

func TestRunDelegateToolAsyncRequiresScheduler(t *testing.T) {
	_, err := runDelegateTool(context.Background(), Dependencies{DelegateRunner: &recordingDelegateRunner{}}, delegateInput{
		ExpertName: "reviewer",
		Task:       "review this",
		Mode:       "async",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
