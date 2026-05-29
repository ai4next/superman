package reflect

import (
	"context"
	"iter"
	"path/filepath"
	"testing"
	"time"

	supermanagent "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/bus"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/genai"
)

type fakeLLM struct{}

func (fakeLLM) Name() string { return "reflect-fake" }

func (fakeLLM) GenerateContent(ctx context.Context, req *adkmodel.LLMRequest, stream bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {
		yield(&adkmodel.LLMResponse{
			Content: genai.NewContentFromText("reflect result", genai.RoleModel),
		}, nil)
	}
}

func TestReflectRunRequestUsesRuntimeFeatures(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Session: config.SessionConfig{
			AppName:  "app",
			MaxTurns: 18,
			LoopDetection: config.LoopDetectionConfig{
				Enabled:    true,
				WindowSize: 6,
				MaxRepeats: 3,
			},
		},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}

	req := reflectRunRequest(cfg, svc, "reflect-user", "1", "inspect @main.go")
	if req.AppName != "app" || req.UserID != "reflect-user" || req.SessionID != "1" {
		t.Fatalf("request identity = app:%q user:%q session:%q", req.AppName, req.UserID, req.SessionID)
	}
	if req.Message == nil || len(req.Message.Parts) != 1 || req.Message.Parts[0].Text != "inspect @main.go" {
		t.Fatalf("message = %#v", req.Message)
	}
	if !req.LoopDetection.Enabled || req.LoopDetection.WindowSize != 6 || req.LoopDetection.MaxRepeats != 3 {
		t.Fatalf("loop detection = %#v", req.LoopDetection)
	}
	if req.Compact == nil {
		t.Fatal("compactor should be configured")
	}
}

func TestExecutorRunPersistsReferencesAndAudit(t *testing.T) {
	workspace := t.TempDir()
	cfg := reflectTestConfig(workspace)
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	a, plugins, err := supermanagent.New(fakeLLM{}, cfg, nil, svc, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	got, err := newExecutor(a, svc, runner.PluginConfig{Plugins: plugins}).run(
		context.Background(),
		cfg,
		"scheduler-user",
		"1",
		`inspect @main.go using [session:past role:user] prior decision`,
		"scheduler",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got != "reflect result" {
		t.Fatalf("result = %q", got)
	}

	extended := svc.(*supermansession.Service)
	files, err := extended.SessionFiles("app", "scheduler-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(workspace, "main.go")
	if len(files) != 1 || files[0].Path != wantPath {
		t.Fatalf("files = %#v, want %s", files, wantPath)
	}
	refs, err := extended.SessionReferences("app", "scheduler-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "prior decision" {
		t.Fatalf("refs = %#v", refs)
	}

	events, err := bus.ReadAuditLog(filepath.Join(workspace, "bus", "events.jsonl"), bus.AuditFilter{
		SessionID: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertLifecycleEvents(t, events)
}

func TestIdleWatcherAndSchedulerUseExecutor(t *testing.T) {
	workspace := t.TempDir()
	cfg := reflectTestConfig(workspace)
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	a, plugins, err := supermanagent.New(fakeLLM{}, cfg, nil, svc, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	pluginCfg := runner.PluginConfig{Plugins: plugins}

	watcher := NewIdleWatcherWithPlugins(a, svc, pluginCfg)
	watcher.execute(context.Background())
	scheduler := NewSchedulerWithPlugins(a, svc, pluginCfg)
	scheduler.executeTask(context.Background(), ScheduleTask{Name: "daily", Prompt: "reflect on progress", Enabled: true})

	events, err := bus.ReadAuditLog(filepath.Join(workspace, "bus", "events.jsonl"), bus.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var reflectFinished, schedulerFinished bool
	for _, event := range events {
		if event.Type == bus.EventRunFinished && event.SessionID == "1" {
			reflectFinished = true
		}
		if event.Type == bus.EventRunFinished && event.SessionID == "1" {
			schedulerFinished = true
		}
	}
	if !reflectFinished || !schedulerFinished {
		t.Fatalf("events missing reflect/scheduler finish: %#v", events)
	}
}

func reflectTestConfig(workspace string) *config.Config {
	return &config.Config{
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
		Reflect: config.ReflectConfig{
			Autonomous: config.AutonomousConfig{IdleTimeout: config.Duration(time.Minute)},
		},
	}
}

func assertLifecycleEvents(t *testing.T, events []bus.Event) {
	t.Helper()
	var hasStarted, hasText, hasFinished bool
	for _, event := range events {
		switch event.Type {
		case bus.EventRunStarted:
			hasStarted = true
		case bus.EventTextDelta:
			hasText = event.Text == "reflect result"
		case bus.EventRunFinished:
			hasFinished = true
		}
	}
	if !hasStarted || !hasText || !hasFinished {
		t.Fatalf("audit events missing expected lifecycle: %#v", events)
	}
}
