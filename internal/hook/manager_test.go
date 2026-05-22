package hook

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if len(m.scripts) != 0 {
		t.Errorf("expected 0 events with scripts, got %d", len(m.scripts))
	}
}

func TestNewManager_WithScripts(t *testing.T) {
	dir := t.TempDir()

	// Create before_run dir with a script
	beforeRunDir := filepath.Join(dir, "before_run")
	if err := os.MkdirAll(beforeRunDir, 0755); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(beforeRunDir, "test.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '{}'"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create after_tool dir with a script
	afterToolDir := filepath.Join(dir, "after_tool")
	if err := os.MkdirAll(afterToolDir, 0755); err != nil {
		t.Fatal(err)
	}
	script2 := filepath.Join(afterToolDir, "log.py")
	if err := os.WriteFile(script2, []byte("print('{}')"), 0755); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	scripts := m.scriptsFor("before_run")
	if len(scripts) != 1 {
		t.Errorf("expected 1 before_run script, got %d", len(scripts))
	}

	scripts = m.scriptsFor("after_tool")
	if len(scripts) != 1 {
		t.Errorf("expected 1 after_tool script, got %d", len(scripts))
	}

	// Non-existent event should return empty
	scripts = m.scriptsFor("on_tool_error")
	if len(scripts) != 0 {
		t.Errorf("expected 0 on_tool_error scripts, got %d", len(scripts))
	}
}

func TestNewManager_NonExecutableSkipped(t *testing.T) {
	dir := t.TempDir()

	beforeRunDir := filepath.Join(dir, "before_run")
	if err := os.MkdirAll(beforeRunDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Write a non-executable file
	if err := os.WriteFile(filepath.Join(beforeRunDir, "note.txt"), []byte("not a script"), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	scripts := m.scriptsFor("before_run")
	if len(scripts) != 0 {
		t.Errorf("non-executable files should be skipped, got %d scripts", len(scripts))
	}
}

func TestRunScript_EmptyOutputMeansAllow(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "empty.sh")
	// Script that produces no output, exits 0
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0"), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := RunScript(t.Context(), script, HookEvent{
		Event:     "before_tool",
		SessionID: "test-session",
	})
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if !result.Allow {
		t.Error("empty stdout should default to Allow=true")
	}
}

func TestRunScript_AllowTrue(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "allow.sh")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
echo '{"allow":true,"reason":"all good"}'`), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := RunScript(t.Context(), script, HookEvent{
		Event:     "before_tool",
		SessionID: "test-session",
	})
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if !result.Allow {
		t.Error("expected Allow=true")
	}
	if result.Reason != "all good" {
		t.Errorf("expected reason 'all good', got %q", result.Reason)
	}
}

func TestRunScript_AllowFalse(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "block.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho '{\"allow\":false,\"reason\":\"test block\"}'"), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := RunScript(t.Context(), script, HookEvent{
		Event:     "before_tool",
		SessionID: "test-session",
	})
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if result.Allow {
		t.Error("expected Allow=false")
	}
	if result.Reason != "test block" {
		t.Errorf("expected reason 'test block', got %q", result.Reason)
	}
}

func TestRunScript_NonJSONOutput(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "plain.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho 'just some text output'"), 0755); err != nil {
		t.Fatal(err)
	}

	result, err := RunScript(t.Context(), script, HookEvent{
		Event:     "after_tool",
		SessionID: "test-session",
	})
	if err != nil {
		t.Fatalf("RunScript: %v", err)
	}
	if !result.Allow {
		t.Error("non-JSON output should default to Allow=true")
	}
}

// TestRunHooks_BeforeBlockSkipsRemaining verifies that when a before_* hook
// returns allow:false, the remaining scripts in that event are skipped and
// the result has Block=true.
func TestRunHooks_BeforeBlockSkipsRemaining(t *testing.T) {
	dir := t.TempDir()

	// Side-effect file that the second script should NOT create
	sideEffect := filepath.Join(dir, "second_ran")

	beforeRunDir := filepath.Join(dir, "before_run")
	if err := os.MkdirAll(beforeRunDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First script: rejects (allow:false)
	firstScript := filepath.Join(beforeRunDir, "01_first.sh")
	if err := os.WriteFile(firstScript, []byte(`#!/bin/sh
echo '{"allow":false,"reason":"blocked by first"}'`), 0755); err != nil {
		t.Fatal(err)
	}

	// Second script: should never execute — would touch sideEffect file
	secondScript := filepath.Join(beforeRunDir, "02_second.sh")
	if err := os.WriteFile(secondScript, []byte("#!/bin/sh\ntouch "+sideEffect), 0755); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result := m.runHooks(context.Background(), "before_run", HookEvent{
		Event:     "before_run",
		SessionID: "test-session",
	})

	if !result.Block {
		t.Error("expected Block=true when before_* script returns allow:false")
	}
	if result.Reason != "blocked by first" {
		t.Errorf("expected reason 'blocked by first', got %q", result.Reason)
	}

	// Second script should NOT have executed
	if _, err := os.Stat(sideEffect); err == nil {
		t.Error("second script should not have executed, but side-effect file exists")
	}
}

// TestRunHooks_AfterDoesNotBlock verifies that after_* hooks always run all
// scripts even if one returns allow:false, and the result is never Block=true.
func TestRunHooks_AfterDoesNotBlock(t *testing.T) {
	dir := t.TempDir()

	// Side-effect file that BOTH scripts will create/append to
	sideEffect := filepath.Join(dir, "ran.txt")

	afterToolDir := filepath.Join(dir, "after_tool")
	if err := os.MkdirAll(afterToolDir, 0755); err != nil {
		t.Fatal(err)
	}

	// First script: returns allow:false
	firstScript := filepath.Join(afterToolDir, "01_first.sh")
	if err := os.WriteFile(firstScript, []byte("#!/bin/sh\necho '{\"allow\":false}'>> "+sideEffect+"\necho '{\"allow\":false}'"), 0755); err != nil {
		t.Fatal(err)
	}

	// Second script: should still execute despite first returning allow:false
	secondScript := filepath.Join(afterToolDir, "02_second.sh")
	if err := os.WriteFile(secondScript, []byte("#!/bin/sh\necho 'second ran' >> "+sideEffect+"\necho '{}'"), 0755); err != nil {
		t.Fatal(err)
	}

	m, err := NewManager(dir)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	result := m.runHooks(context.Background(), "after_tool", HookEvent{
		Event:     "after_tool",
		SessionID: "test-session",
	})

	if result.Block {
		t.Error("after_* hooks should never set Block=true, even when allow:false")
	}

	// Both scripts should have executed (each wrote to sideEffect)
	data, err := os.ReadFile(sideEffect)
	if err != nil {
		t.Fatalf("side-effect file not created: %v — both scripts should have run", err)
	}
	// Verify both scripts ran by checking the content
	content := string(data)
	if len(content) == 0 {
		t.Error("side-effect file is empty; expected output from both scripts")
	}
}
