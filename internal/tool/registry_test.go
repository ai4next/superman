package tool

import (
	"context"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
)

type fakeExpertManager struct {
	experts []*expert.Spec
}

func (m fakeExpertManager) List() []*expert.Spec { return m.experts }

type fakeDelegateRunner struct{}

func (fakeDelegateRunner) RunDelegate(context.Context, string, string) (string, error) {
	return "", nil
}

type fakeDelegateScheduler struct{}

func (fakeDelegateScheduler) EnqueueDelegate(context.Context, DelegateTaskRequest) (DelegateTaskReceipt, error) {
	return DelegateTaskReceipt{TaskID: "task-1", Status: "queued"}, nil
}

type fakeOrchestrator struct{}

func (fakeOrchestrator) SubmitPlan(context.Context, string) (OrchestratorReceipt, error) {
	return OrchestratorReceipt{PlanID: "p1", Status: "running", Queued: 1}, nil
}

func toolNames(ts []tool.Tool) map[string]bool {
	names := make(map[string]bool, len(ts))
	for _, t := range ts {
		names[t.Name()] = true
	}
	return names
}

func TestRegisterAllExpertToolsFlag(t *testing.T) {
	cfg := &config.Config{}

	without := RegisterAll(Dependencies{
		Config:         cfg,
		ExpertManager:  fakeExpertManager{},
		DelegateRunner: fakeDelegateRunner{},
		ExpertTools:    false,
	})
	withoutNames := toolNames(without)
	if withoutNames["delegate"] {
		t.Fatalf("tool %q should be disabled when ExpertTools=false", "delegate")
	}

	with := RegisterAll(Dependencies{
		Config:         cfg,
		ExpertManager:  fakeExpertManager{experts: []*expert.Spec{{Name: "architect"}}},
		DelegateRunner: fakeDelegateRunner{},
		ExpertTools:    true,
	})
	withNames := toolNames(with)
	if !withNames["delegate"] {
		t.Fatalf("tool %q should be enabled when ExpertTools=true", "delegate")
	}
}

func TestRegisterAllSkipsDelegateWithoutExperts(t *testing.T) {
	cfg := &config.Config{}

	tools := RegisterAll(Dependencies{
		Config:         cfg,
		ExpertManager:  fakeExpertManager{},
		DelegateRunner: fakeDelegateRunner{},
		ExpertTools:    true,
	})
	if toolNames(tools)["delegate"] {
		t.Fatalf("tool %q should be disabled when no experts are available", "delegate")
	}
}

func TestRegisterAllAllowsDelegateWithSchedulerOnly(t *testing.T) {
	cfg := &config.Config{}

	tools := RegisterAll(Dependencies{
		Config:            cfg,
		ExpertManager:     fakeExpertManager{experts: []*expert.Spec{{Name: "architect"}}},
		DelegateScheduler: fakeDelegateScheduler{},
		ExpertTools:       true,
	})
	if !toolNames(tools)["delegate"] {
		t.Fatalf("tool %q should be enabled when scheduler is available", "delegate")
	}
}

func TestRegisterAllIncludesOrchestrateTool(t *testing.T) {
	tools := RegisterAll(Dependencies{
		Config:        &config.Config{},
		ExpertManager: fakeExpertManager{experts: []*expert.Spec{{Name: "architect"}}},
		Orchestrator:  fakeOrchestrator{},
		ExpertTools:   true,
	})
	if !toolNames(tools)["orchestrate"] {
		t.Fatalf("tool %q should be enabled when orchestrator is available", "orchestrate")
	}
}
