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

func toolNames(ts []tool.Tool) map[string]bool {
	names := make(map[string]bool, len(ts))
	for _, t := range ts {
		names[t.Name()] = true
	}
	return names
}

func TestRegisterAllExpertToolsFlag(t *testing.T) {
	cfg := &config.Config{}
	cfg.Expert.Enabled = true

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
	cfg.Expert.Enabled = true

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
