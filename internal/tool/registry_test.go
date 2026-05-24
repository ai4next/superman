package tool

import (
	"context"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"google.golang.org/adk/tool"
)

type fakeExpertManager struct{}

func (fakeExpertManager) List() []*expert.Spec { return nil }

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
	if withoutNames["delegate_to_expert"] {
		t.Fatalf("tool %q should be disabled when ExpertTools=false", "delegate_to_expert")
	}

	with := RegisterAll(Dependencies{
		Config:         cfg,
		ExpertManager:  fakeExpertManager{},
		DelegateRunner: fakeDelegateRunner{},
		ExpertTools:    true,
	})
	withNames := toolNames(with)
	if !withNames["delegate_to_expert"] {
		t.Fatalf("tool %q should be enabled when ExpertTools=true", "delegate_to_expert")
	}
}

func TestRegisterAllBrowserUse(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.BrowserUse.Enabled = true

	tools := RegisterAll(Dependencies{Config: cfg})
	names := toolNames(tools)
	if !names["browser_use"] {
		t.Fatalf("tool %q should be enabled when browser_use.enabled=true", "browser_use")
	}
}
