package tools

import (
	"context"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
)

type fakeExpertManager struct{}

func (fakeExpertManager) Search(string) []*expert.Spec { return nil }
func (fakeExpertManager) List() []*expert.Spec         { return nil }
func (fakeExpertManager) Create(expert.Spec) (*expert.Spec, error) {
	return nil, nil
}

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
	for _, name := range []string{"query_experts", "create_expert", "delegate_to_expert"} {
		if withoutNames[name] {
			t.Fatalf("tool %q should be disabled when ExpertTools=false", name)
		}
	}

	with := RegisterAll(Dependencies{
		Config:         cfg,
		ExpertManager:  fakeExpertManager{},
		DelegateRunner: fakeDelegateRunner{},
		ExpertTools:    true,
	})
	withNames := toolNames(with)
	for _, name := range []string{"query_experts", "create_expert", "delegate_to_expert"} {
		if !withNames[name] {
			t.Fatalf("tool %q should be enabled when ExpertTools=true", name)
		}
	}
}
