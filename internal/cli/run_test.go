package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/bus"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
)

func TestRunPromptInputPriority(t *testing.T) {
	oldPrompt, oldFile := runPrompt, runFile
	t.Cleanup(func() {
		runPrompt, runFile = oldPrompt, oldFile
	})
	runPrompt = "from flag"
	runFile = ""

	got, err := runPromptInput([]string{"from arg"}, strings.NewReader("from stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from flag" {
		t.Fatalf("prompt = %q, want flag value", got)
	}
}

func TestRunPromptInputFileAndStdin(t *testing.T) {
	oldPrompt, oldFile := runPrompt, runFile
	t.Cleanup(func() {
		runPrompt, runFile = oldPrompt, oldFile
	})
	path := filepath.Join(t.TempDir(), "prompt.txt")
	if err := os.WriteFile(path, []byte("from file"), 0o644); err != nil {
		t.Fatal(err)
	}
	runPrompt = ""
	runFile = path
	got, err := runPromptInput(nil, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from file" {
		t.Fatalf("prompt = %q, want file value", got)
	}

	runFile = ""
	got, err = runPromptInput(nil, strings.NewReader("from stdin"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "from stdin" {
		t.Fatalf("prompt = %q, want stdin value", got)
	}
}

func TestRunPromptInputRejectsEmpty(t *testing.T) {
	oldPrompt, oldFile := runPrompt, runFile
	t.Cleanup(func() {
		runPrompt, runFile = oldPrompt, oldFile
	})
	runPrompt = ""
	runFile = ""
	if _, err := runPromptInput(nil, strings.NewReader("")); err == nil {
		t.Fatal("empty prompt should fail")
	}
}

func TestBuildRunRequestUsesConfiguredSession(t *testing.T) {
	oldUser, oldSession := runUser, runSession
	t.Cleanup(func() {
		runUser, runSession = oldUser, oldSession
	})
	runUser = "operator"
	runSession = "1"
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Session: config.SessionConfig{
			AppName:  "app",
			MaxTurns: 42,
			LoopDetection: config.LoopDetectionConfig{
				Enabled:    true,
				WindowSize: 7,
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

	req := buildRunRequest(cfg, svc, "ship it")
	if req.AppName != "app" || req.UserID != "operator" || req.SessionID != "1" {
		t.Fatalf("request identity = app:%q user:%q session:%q", req.AppName, req.UserID, req.SessionID)
	}
	if req.Message == nil || len(req.Message.Parts) != 1 || req.Message.Parts[0].Text != "ship it" {
		t.Fatalf("message = %#v", req.Message)
	}
	if !req.LoopDetection.Enabled || req.LoopDetection.WindowSize != 7 || req.LoopDetection.MaxRepeats != 3 {
		t.Fatalf("loop detection = %#v", req.LoopDetection)
	}
	if req.Compact == nil {
		t.Fatal("compactor should be configured")
	}
}

func TestWriteRunEventWritesAuditSynchronously(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bus", "events.jsonl")
	logger := bus.NewAuditLogger(path)
	var out bytes.Buffer
	event := bus.Event{
		Type:      bus.EventTextDelta,
		SessionID: "1",
		RunID:     "r1",
		Text:      "hello",
	}

	if err := writeRunEvent(&out, logger, event); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello" {
		t.Fatalf("stdout = %q, want hello", out.String())
	}
	events, err := bus.ReadAuditLog(path, bus.AuditFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Type != bus.EventTextDelta || events[0].Text != "hello" {
		t.Fatalf("audit events = %#v", events)
	}
}

func TestEnsureRunSessionAndRecordPromptReferences(t *testing.T) {
	oldUser, oldSession := runUser, runSession
	t.Cleanup(func() {
		runUser, runSession = oldUser, oldSession
	})
	workspace := t.TempDir()
	runUser = "operator"
	runSession = "1"
	cfg := &config.Config{
		Workspace: workspace,
		Session:   config.SessionConfig{AppName: "app"},
	}
	global.SetConfig(cfg)
	t.Cleanup(func() { global.SetConfig(nil) })
	svc, err := supermansession.NewService()
	if err != nil {
		t.Fatal(err)
	}
	extended := svc.(*supermansession.Service)
	req := buildRunRequest(cfg, svc, `review @main.go from [session:past role:user] cache decision`)

	if err := ensureRunSession(context.Background(), svc, &req); err != nil {
		t.Fatal(err)
	}
	supermansession.RecordPromptReferences(extended, req.AppName, req.UserID, req.SessionID, cfg.Workspace, `review @main.go from [session:past role:user] cache decision`)

	files, err := extended.SessionFiles("app", "operator", req.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(workspace, "main.go")
	if len(files) != 1 || files[0].Path != wantPath || files[0].ReadCount != 1 {
		t.Fatalf("files = %#v, want %s read once", files, wantPath)
	}
	refs, err := extended.SessionReferences("app", "operator", "1")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].SessionID != "past" || refs[0].Preview != "cache decision" {
		t.Fatalf("refs = %#v", refs)
	}
}
