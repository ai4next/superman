package cli

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/bus"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tool"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/genai"
)

type delegateFakeLLM struct{}

func (delegateFakeLLM) Name() string { return "delegate-fake" }

func (delegateFakeLLM) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{
			Content: genai.NewContentFromText("expert result", genai.RoleModel),
		}, nil)
	}
}

func TestDelegateServiceEnqueueAndRunQueuedDelegate(t *testing.T) {
	workspace := t.TempDir()
	expertDir := filepath.Join(workspace, "state", "reviewer")
	if err := os.MkdirAll(expertDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expertDir, "soul.md"), []byte("You review code."), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Workspace: workspace,
		Model:     config.ModelConfig{Provider: "test", Name: "fake"},
		Session:   config.SessionConfig{AppName: "app", MaxTurns: 20},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })

	registry := expert.NewRegistry(global.ExpertsDir())
	if err := registry.LoadFromDisk(); err != nil {
		t.Fatal(err)
	}
	queue := bus.NewChannelQueue(100)
	defer queue.Close()
	ds := newDelegateServiceWithQueue(delegateFakeLLM{}, registry, queue)

	receipt, err := ds.EnqueueDelegate(context.Background(), tool.DelegateTaskRequest{
		ExpertName: "reviewer",
		Task:       "inspect queued task",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.TaskID == "" || receipt.Status != "ready" {
		t.Fatalf("receipt = %#v", receipt)
	}
	running, ok, err := queue.Dequeue(bus.WorkerRef{ID: "test-worker", Queue: "experts", Type: "delegate"})
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected queued delegate task")
	}
	ds.runQueuedDelegate(context.Background(), running)
	result, ok, err := queue.TaskResult(receipt.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || strings.TrimSpace(result.Result) != "expert result" {
		t.Fatalf("result = %#v ok=%v", result, ok)
	}
}

func TestDelegateServiceSubmitPlan(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{
		Workspace: workspace,
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	queue := bus.NewChannelQueue(100)
	defer queue.Close()
	ds := newDelegateServiceWithQueue(delegateFakeLLM{}, expert.NewRegistry(global.ExpertsDir()), queue)
	receipt, err := ds.SubmitPlan(context.Background(), `{"plan_id":"p1","goal":"g","tasks":[{"id":"t1","expert":"reviewer","input":{"prompt":"do"}}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PlanID != "p1" || receipt.Queued != 1 {
		t.Fatalf("receipt = %#v", receipt)
	}
	if _, err := os.Stat(filepath.Join(global.PlansDir(), "p1.json")); err != nil {
		t.Fatalf("plan not saved: %v", err)
	}
}

func TestBuildDelegateRunRequestUsesRuntimeFeatures(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Session: config.SessionConfig{
			AppName:  "app",
			MaxTurns: 12,
			LoopDetection: config.LoopDetectionConfig{
				Enabled:    true,
				WindowSize: 4,
				MaxRepeats: 2,
			},
		},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewServiceInRoot(global.ExpertDir("reviewer"))
	if err != nil {
		t.Fatal(err)
	}

	req := buildDelegateRunRequest(cfg, svc, "reviewer", "inspect @main.go")
	if req.AppName != "app-expert" || req.UserID != "expert-user" || req.SessionID != "" {
		t.Fatalf("request identity = app:%q user:%q session:%q", req.AppName, req.UserID, req.SessionID)
	}
	if req.Message == nil || len(req.Message.Parts) != 1 || req.Message.Parts[0].Text != "inspect @main.go" {
		t.Fatalf("message = %#v", req.Message)
	}
	if !req.LoopDetection.Enabled || req.LoopDetection.WindowSize != 4 || req.LoopDetection.MaxRepeats != 2 {
		t.Fatalf("loop detection = %#v", req.LoopDetection)
	}
	if req.Compact == nil {
		t.Fatal("compactor should be configured")
	}
}

func TestDelegateRunPersistsSessionReferencesAndAudit(t *testing.T) {
	workspace := t.TempDir()
	expertDir := filepath.Join(workspace, "state", "reviewer")
	if err := os.MkdirAll(expertDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expertDir, "soul.md"), []byte("You review code."), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Workspace: workspace,
		Model:     config.ModelConfig{Provider: "test", Name: "fake"},
		Session: config.SessionConfig{
			AppName:  "app",
			MaxTurns: 20,
			LoopDetection: config.LoopDetectionConfig{
				Enabled:    true,
				WindowSize: 5,
				MaxRepeats: 3,
			},
		},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })

	registry := expert.NewRegistry(global.ExpertsDir())
	if err := registry.LoadFromDisk(); err != nil {
		t.Fatal(err)
	}
	evolutionCh := make(chan hook.EvolutionSignal, 1)
	ds := newDelegateService(delegateFakeLLM{}, registry, evolutionCh)

	result, err := ds.RunDelegate(context.Background(), "reviewer", `inspect @main.go using [session:past role:user] prior decision`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != "expert result" {
		t.Fatalf("result = %q", result)
	}

	svc, err := supermansession.NewServiceInRoot(global.ExpertDir("reviewer"))
	if err != nil {
		t.Fatal(err)
	}
	extended := svc.(*supermansession.Service)
	files, err := extended.SessionFiles("app-expert", "expert-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(workspace, "main.go")
	if len(files) != 1 || files[0].Path != wantPath {
		t.Fatalf("files = %#v, want %s", files, wantPath)
	}
	refs, err := extended.SessionReferences("app-expert", "expert-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "prior decision" {
		t.Fatalf("refs = %#v", refs)
	}
	if _, err := os.Stat(filepath.Join(global.AgentSessionsDir("reviewer"), "1.log")); err != nil {
		t.Fatalf("expert session log missing: %v", err)
	}

	events, err := bus.ReadAuditLog(filepath.Join(workspace, "bus", "events.jsonl"), bus.AuditFilter{
		SessionID: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	var hasStarted, hasText, hasFinished bool
	for _, event := range events {
		switch event.Type {
		case bus.EventRunStarted:
			hasStarted = true
		case bus.EventTextDelta:
			hasText = event.Text == "expert result"
		case bus.EventRunFinished:
			hasFinished = true
		}
	}
	if !hasStarted || !hasText || !hasFinished {
		t.Fatalf("audit events missing expected lifecycle: %#v", events)
	}

	select {
	case signal := <-evolutionCh:
		if signal.Role != "expert" || signal.AgentName != "reviewer" || signal.UserID != "expert-user" || signal.SessionID != "1" {
			t.Fatalf("evolution signal = %#v", signal)
		}
		if signal.RootDir != global.ExpertDir("reviewer") {
			t.Fatalf("signal RootDir = %q, want %q", signal.RootDir, global.ExpertDir("reviewer"))
		}
	default:
		t.Fatal("expected expert evolution signal")
	}
}
