package agent

import (
	"context"
	_ "embed"
	"log"
	"os"
	"path/filepath"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/skilltoolset"
	"google.golang.org/adk/tool/skilltoolset/skill"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/tool"
)

//go:embed system.txt
var systemPrompt string

// BuildConfig describes one concrete agent instance. Superman and experts use
// this same builder so their runtime wiring stays consistent.
type BuildConfig struct {
	Name              string
	Description       string
	Instruction       string
	MemoryService     *memory.Service
	SOPContent        string
	ExpertRegistry    *expert.Registry
	DelegateRunner    tool.DelegateRunner
	EnableExpertTools bool
	EvolutionSignal   hook.EvolutionSignal
	EvolutionCh       chan<- hook.EvolutionSignal // completed-run signal receiver
}

// NewFromConfig creates an agent with the shared Superman runtime wiring:
// configured tools, optional isolated memory, shared hooks, and shared skills.
func NewFromConfig(llm model.LLM, cfg *config.Config, build BuildConfig) (adkagent.Agent, []*plugin.Plugin, error) {
	expertRegistry := build.ExpertRegistry
	delegateRunner := build.DelegateRunner
	if !build.EnableExpertTools {
		expertRegistry = nil
		delegateRunner = nil
	}

	deps := tool.Dependencies{
		Config:         cfg,
		ExpertManager:  expertRegistry,
		DelegateRunner: delegateRunner,
		ExpertTools:    build.EnableExpertTools,
	}

	toolList := tool.RegisterAll(deps)

	var extraPlugins []*plugin.Plugin
	signal := build.EvolutionSignal
	if signal.AgentName == "" {
		signal.AgentName = build.Name
	}
	hookMgr, err := hook.NewManagerWithSignal(filepath.Join(cfg.Workspace, "hooks"), signal, build.EvolutionCh)
	if err != nil {
		log.Printf("[agent] hook manager: %v", err)
	} else {
		extraPlugins = append(extraPlugins, hookMgr.Plugin())
	}

	var agentToolsets []adktool.Toolset
	skillsDir := filepath.Join(cfg.Workspace, "skills")
	skillFS := os.DirFS(skillsDir)
	skillSource := skill.NewFileSystemSource(skillFS)
	skillTS, err := skilltoolset.New(context.Background(), skilltoolset.Config{Source: skillSource})
	if err != nil {
		log.Printf("[agent] skill toolset: %v", err)
	} else {
		agentToolsets = append(agentToolsets, skillTS)
		log.Printf("[agent] skill toolset loaded from %s", skillsDir)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:                build.Name,
		Model:               llm,
		Description:         build.Description,
		InstructionProvider: instructionProvider(build),
		Tools:               toolList,
		Toolsets:            agentToolsets,
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("[agent] created %s agent with %d tools", build.Name, len(toolList))

	return a, extraPlugins, nil
}

func instructionProvider(build BuildConfig) func(adkagent.ReadonlyContext) (string, error) {
	builder := strings.Builder{}
	return func(adkagent.ReadonlyContext) (string, error) {
		defer builder.Reset()
		builder.WriteString(build.Instruction)
		if build.SOPContent != "" {
			builder.WriteString("\n\n## SOP Rules\n")
			builder.WriteString(build.SOPContent)
		}
		if build.MemoryService != nil {
			if l0Content := build.MemoryService.GetL0Content(); l0Content != "" {
				builder.WriteString("\n\n")
				builder.WriteString(l0Content)
			}
		}
		return builder.String(), nil
	}
}

// New creates a new Superman agent with all registered tools, optional SOP rules,
// L0 index injected into the system prompt, and completed-run signal receivers.
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, sopContent string, expertRegistry *expert.Registry, delegateRunner tool.DelegateRunner, evolutionCh chan<- hook.EvolutionSignal) (adkagent.Agent, []*plugin.Plugin, error) {
	expertDir := ""
	if cfg.Expert.Enabled {
		expertDir = cfg.ExpertDir()
	}
	memoryDir := ""
	if memSvc != nil {
		memoryDir = memSvc.MemoryDir()
	}

	return NewFromConfig(llm, cfg, BuildConfig{
		Name:              "superman",
		Description:       "Superman - general-purpose autonomous AI assistant",
		Instruction:       systemPrompt,
		MemoryService:     memSvc,
		SOPContent:        sopContent,
		ExpertRegistry:    expertRegistry,
		DelegateRunner:    delegateRunner,
		EnableExpertTools: true,
		EvolutionSignal: hook.EvolutionSignal{
			Role:      "superman",
			MemoryDir: memoryDir,
			ExpertDir: expertDir,
		},
		EvolutionCh: evolutionCh,
	})
}

// NewWithoutMemory creates an agent without a memory service (for simple CLI usage).
func NewWithoutMemory(llm model.LLM, cfg *config.Config) (adkagent.Agent, []*plugin.Plugin, error) {
	return New(llm, cfg, nil, "", nil, nil, nil)
}
