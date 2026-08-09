package tool

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingDelegateRunner struct {
	expert string
	task   string
	ctx    context.Context
}

func (r *recordingDelegateRunner) RunDelegate(ctx context.Context, expertName string, task string) (string, error) {
	r.ctx = ctx
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
	type contextKey string
	ctx := context.WithValue(t.Context(), contextKey("request"), "request-1")
	out, err := runDelegateTool(ctx, Dependencies{DelegateRunner: runner}, delegateInput{
		ExpertName: " architect ",
		Task:       " design this ",
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
	if got := runner.ctx.Value(contextKey("request")); got != "request-1" {
		t.Fatalf("delegate context value = %v, want request-1", got)
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

func TestRunDelegateToolRejectsInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input delegateInput
		want  string
	}{
		{name: "expert", input: delegateInput{Task: "review"}, want: "expert_name is required"},
		{name: "task", input: delegateInput{ExpertName: "reviewer"}, want: "task is required"},
		{name: "mode", input: delegateInput{ExpertName: "reviewer", Task: "review", Mode: "later"}, want: "unsupported delegate mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runDelegateTool(t.Context(), Dependencies{DelegateRunner: &recordingDelegateRunner{}}, tc.input)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestRunDelegateToolHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	runner := &recordingDelegateRunner{}

	_, err := runDelegateTool(ctx, Dependencies{DelegateRunner: runner}, delegateInput{
		ExpertName: "reviewer",
		Task:       "review",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if runner.ctx != nil {
		t.Fatal("delegate runner should not be called after cancellation")
	}
}
