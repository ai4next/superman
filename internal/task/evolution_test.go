package task

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	adkmodel "google.golang.org/adk/model"

	"github.com/ai4next/superman/internal/bus"
)

type evolutionFakeLLM struct{}

func (evolutionFakeLLM) Name() string { return "evolution-fake" }

func (evolutionFakeLLM) GenerateContent(context.Context, *adkmodel.LLMRequest, bool) iter.Seq2[*adkmodel.LLMResponse, error] {
	return func(yield func(*adkmodel.LLMResponse, error) bool) {}
}

func TestEvolutionLoopPublishesFailedEvent(t *testing.T) {
	global.SetConfig(&config.Config{Workspace: t.TempDir()})
	t.Cleanup(func() { global.SetConfig(nil) })

	broker := bus.NewMemoryBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := broker.Subscribe(ctx, bus.EventFilter{})
	if err != nil {
		t.Fatal(err)
	}

	e := &Evolution{signal: make(chan hook.EvolutionSignal, 1)}
	e.SetBroker(broker)
	go e.Loop(ctx)
	e.signal <- hook.EvolutionSignal{SessionID: "s1", Role: "superman"}

	var got []bus.Event
	deadline := time.After(time.Second)
	for len(got) < 2 {
		select {
		case event := <-events:
			got = append(got, event)
		case <-deadline:
			t.Fatalf("timed out waiting for evolution events: %+v", got)
		}
	}
	if got[0].Type != bus.EventEvolutionStarted {
		t.Fatalf("first event = %+v", got[0])
	}
	if got[1].Type != bus.EventEvolutionFailed || strings.TrimSpace(got[1].Error) == "" {
		t.Fatalf("second event = %+v", got[1])
	}
}

func TestNewEvolutionCreatesPersistentEvolutionRoot(t *testing.T) {
	workspace := t.TempDir()
	global.SetConfig(&config.Config{
		Workspace: workspace,
		Tools: config.ToolsConfig{
			Read:  config.ReadConfig{Enabled: true},
			Write: config.WriteConfig{Enabled: true},
			Patch: config.PatchConfig{Enabled: true},
		},
	})
	t.Cleanup(func() { global.SetConfig(nil) })

	e, err := NewEvolution(evolutionFakeLLM{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.supermanRunner == nil || e.expertRunner == nil {
		t.Fatalf("evolution runners were not initialized: %#v", e)
	}
	if e.metaRunner == nil {
		t.Fatalf("meta evolution runner was not initialized: %#v", e)
	}
	for _, path := range []string{
		filepath.Join(workspace, "evolution"),
		filepath.Join(workspace, "evolution", "memory"),
		filepath.Join(workspace, "evolution", "memory", "l1.toml"),
		filepath.Join(workspace, "evolution", "memory", "l2"),
		filepath.Join(workspace, "evolution", "sessions"),
		filepath.Join(workspace, "evolution", "state.db"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected evolution path %s: %v", path, err)
		}
	}
}

func TestEvolutionDataFromMetaSignalUsesEvolutionScope(t *testing.T) {
	workspace := t.TempDir()
	global.SetConfig(&config.Config{Workspace: workspace})
	t.Cleanup(func() { global.SetConfig(nil) })

	data := evolutionDataFromSignal(hook.EvolutionSignal{
		SessionID: "agent-evolution-superman-superman-1",
		AgentName: "meta-evolver",
		Role:      "meta",
		RootDir:   filepath.Join(workspace, "evolution"),
	})

	if data.RootDir != filepath.Join(workspace, "evolution") {
		t.Fatalf("RootDir = %q", data.RootDir)
	}
	if data.SessionLogPath != filepath.Join(workspace, "evolution", "sessions", "agent-evolution-superman-superman-1.log") {
		t.Fatalf("SessionLogPath = %q", data.SessionLogPath)
	}
	if data.L1Path != filepath.Join(workspace, "evolution", "memory", "l1.toml") {
		t.Fatalf("L1Path = %q", data.L1Path)
	}
	if data.SOPDir != filepath.Join(workspace, "evolution", "memory", "l2") {
		t.Fatalf("SOPDir = %q", data.SOPDir)
	}
	if !data.MetaEvolution || data.CultivateExperts || data.CanEditSoul || data.ExpertDir != "" || data.SoulPath != "" {
		t.Fatalf("meta scope flags = meta:%v cultivate:%v edit:%v expertDir:%q soul:%q", data.MetaEvolution, data.CultivateExperts, data.CanEditSoul, data.ExpertDir, data.SoulPath)
	}
	prompt, err := renderEvolutionPrompt(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "This is meta evolution") || !strings.Contains(prompt, "Do not trigger or request another meta-evolution pass") {
		t.Fatalf("meta prompt missing boundary instructions: %q", prompt)
	}
}

func TestEvolutionDataFromSupermanSignalUsesWorkspaceScope(t *testing.T) {
	workspace := t.TempDir()
	global.SetConfig(&config.Config{
		Workspace: workspace,
		Expert:    config.ExpertConfig{MaxCount: 10},
	})
	t.Cleanup(func() { global.SetConfig(nil) })

	data := evolutionDataFromSignal(hook.EvolutionSignal{
		SessionID: "s1",
		AgentName: "superman",
		Role:      "superman",
	})

	if data.RootDir != workspace {
		t.Fatalf("RootDir = %q, want %q", data.RootDir, workspace)
	}
	if data.SessionLogPath != filepath.Join(workspace, "sessions", "s1.log") {
		t.Fatalf("SessionLogPath = %q", data.SessionLogPath)
	}
	if data.L1Path != filepath.Join(workspace, "memory", "l1.toml") {
		t.Fatalf("L1Path = %q", data.L1Path)
	}
	if data.SOPDir != filepath.Join(workspace, "memory", "l2") {
		t.Fatalf("SOPDir = %q", data.SOPDir)
	}
	if data.ExpertDir != filepath.Join(workspace, "experts") || !data.CultivateExperts || !data.CanCreateExpert {
		t.Fatalf("expert cultivation flags/path = dir:%q cultivate:%v create:%v", data.ExpertDir, data.CultivateExperts, data.CanCreateExpert)
	}
	if data.SoulPath != "" || data.CanEditSoul {
		t.Fatalf("superman scope should not edit a soul directly: soul=%q edit=%v", data.SoulPath, data.CanEditSoul)
	}
	prompt, err := renderEvolutionPrompt(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Autonomously govern the expert team") {
		t.Fatalf("superman prompt should authorize autonomous expert governance: %q", prompt)
	}
	if strings.Contains(prompt, "explicitly allows") {
		t.Fatalf("superman prompt should not require explicit user instruction: %q", prompt)
	}
}

func TestEvolutionDataFromExpertSignalUsesExpertScope(t *testing.T) {
	workspace := t.TempDir()
	global.SetConfig(&config.Config{Workspace: workspace})
	t.Cleanup(func() { global.SetConfig(nil) })

	expertRoot := filepath.Join(workspace, "experts", "reviewer")
	data := evolutionDataFromSignal(hook.EvolutionSignal{
		SessionID: "7",
		AgentName: "reviewer",
		Role:      "expert",
		RootDir:   expertRoot,
	})

	if data.RootDir != expertRoot {
		t.Fatalf("RootDir = %q, want %q", data.RootDir, expertRoot)
	}
	if data.SessionLogPath != filepath.Join(expertRoot, "sessions", "7.log") {
		t.Fatalf("SessionLogPath = %q", data.SessionLogPath)
	}
	if data.L1Path != filepath.Join(expertRoot, "memory", "l1.toml") {
		t.Fatalf("L1Path = %q", data.L1Path)
	}
	if data.SOPDir != filepath.Join(expertRoot, "memory", "l2") {
		t.Fatalf("SOPDir = %q", data.SOPDir)
	}
	if data.SoulPath != filepath.Join(expertRoot, "soul.md") || !data.CanEditSoul {
		t.Fatalf("expert soul flags/path = soul:%q edit:%v", data.SoulPath, data.CanEditSoul)
	}
	if data.ExpertDir != "" || data.CultivateExperts || data.CanCreateExpert {
		t.Fatalf("expert scope should not cultivate experts: dir:%q cultivate:%v create:%v", data.ExpertDir, data.CultivateExperts, data.CanCreateExpert)
	}
	prompt, err := renderEvolutionPrompt(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Do not create, delete, rename, merge, or otherwise govern other experts") {
		t.Fatalf("expert prompt should forbid expert team governance: %q", prompt)
	}
	if strings.Contains(prompt, "Autonomously govern the expert team") {
		t.Fatalf("expert prompt should not include superman governance instruction: %q", prompt)
	}
}
