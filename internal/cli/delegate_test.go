package cli

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
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
	svc, err := supermansession.NewService()
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
	expertDir := filepath.Join(workspace, "experts", "reviewer")
	if err := os.MkdirAll(expertDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expertDir, "expert.yaml"), []byte("name: reviewer\nprompt: You review code.\n"), 0o644); err != nil {
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
		Permissions: config.PermissionsConfig{SkipRequests: true},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })

	registry := expert.NewRegistry(global.ExpertsDir())
	if err := registry.LoadFromDisk(); err != nil {
		t.Fatal(err)
	}
	ds := newDelegateService(delegateFakeLLM{}, registry)

	result, err := ds.RunDelegate(context.Background(), "reviewer", `inspect @main.go using [session:past role:user] prior decision`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(result) != "expert result" {
		t.Fatalf("result = %q", result)
	}

	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	files, err := svc.SessionFiles("app-expert", "expert-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(workspace, "main.go")
	if len(files) != 1 || files[0].Path != wantPath {
		t.Fatalf("files = %#v, want %s", files, wantPath)
	}
	refs, err := svc.SessionReferences("app-expert", "expert-user", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "prior decision" {
		t.Fatalf("refs = %#v", refs)
	}

	events, err := supermanruntime.ReadAuditLog(filepath.Join(workspace, "runtime", "events.jsonl"), supermanruntime.AuditFilter{
		SessionID: "1",
	})
	if err != nil {
		t.Fatal(err)
	}
	var hasStarted, hasText, hasFinished bool
	for _, event := range events {
		switch event.Type {
		case supermanruntime.EventRunStarted:
			hasStarted = true
		case supermanruntime.EventTextDelta:
			hasText = event.Text == "expert result"
		case supermanruntime.EventRunFinished:
			hasFinished = true
		}
	}
	if !hasStarted || !hasText || !hasFinished {
		t.Fatalf("audit events missing expected lifecycle: %#v", events)
	}
}
