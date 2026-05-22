package agent

import (
	"context"
	_ "embed"
	"log"
	"os"
	"path/filepath"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/skilltoolset"
	"google.golang.org/adk/tool/skilltoolset/skill"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
)

//go:embed prompt/system.txt
var systemPrompt string

// BuildConfig describes one concrete agent instance. Superman and experts use
// this same builder so their runtime wiring stays consistent.
type BuildConfig struct {
	Name              string
	Description       string
	Instruction       string
	MemoryService     *memory.Service
	MemorySearcher    tools.MemorySearcher
	SOPContent        string
	ExpertRegistry    *expert.Registry
	DelegateRunner    tools.DelegateRunner
	EnableExpertTools bool
}

// NewFromConfig creates an agent with the shared Superman runtime wiring:
// configured tools, optional isolated memory, shared hooks, and shared skills.
func NewFromConfig(llm model.LLM, cfg *config.Config, build BuildConfig) (adkagent.Agent, []*plugin.Plugin, error) {
	if build.Name == "" {
		build.Name = "superman"
	}
	if build.Description == "" {
		build.Description = "Superman - general-purpose autonomous AI assistant"
	}
	if build.Instruction == "" {
		build.Instruction = systemPrompt
	}

	expertRegistry := build.ExpertRegistry
	delegateRunner := build.DelegateRunner
	if !build.EnableExpertTools {
		expertRegistry = nil
		delegateRunner = nil
	}

	deps := tools.Dependencies{
		Config:         cfg,
		Workspace:      cfg.Tools.CodeRun.Workspace,
		MemoryService:  build.MemoryService,
		MemorySearcher: build.MemorySearcher,
		ExpertManager:  expertRegistry,
		DelegateRunner: delegateRunner,
		ExpertTools:    build.EnableExpertTools,
	}

	toolList := tools.RegisterAll(deps)

	instruction := build.Instruction
	if build.SOPContent != "" {
		instruction += "\n\n## SOP Rules\n" + build.SOPContent
	}

	var beforeModelCallbacks []llmagent.BeforeModelCallback
	if build.MemoryService != nil {
		beforeModelCallbacks = append(beforeModelCallbacks, func(ctx adkagent.CallbackContext, req *model.LLMRequest) (*model.LLMResponse, error) {
			l1Content := build.MemoryService.GetL1Content()
			if l1Content == "" {
				return nil, nil
			}
			if req.Config == nil {
				req.Config = &genai.GenerateContentConfig{}
			}
			if req.Config.SystemInstruction == nil {
				req.Config.SystemInstruction = genai.NewContentFromText(l1Content, genai.RoleUser)
				return nil, nil
			}
			if len(req.Config.SystemInstruction.Parts) > 0 && req.Config.SystemInstruction.Parts[len(req.Config.SystemInstruction.Parts)-1].Text != "" {
				req.Config.SystemInstruction.Parts[len(req.Config.SystemInstruction.Parts)-1].Text += "\n\n" + l1Content
				return nil, nil
			}
			req.Config.SystemInstruction.Parts = append(req.Config.SystemInstruction.Parts, genai.NewPartFromText(l1Content))
			return nil, nil
		})
	}

	var extraPlugins []*plugin.Plugin
	hookMgr, err := hook.NewManager(filepath.Join(cfg.Dir, "hooks"))
	if err != nil {
		log.Printf("[agent] hook manager: %v", err)
	} else {
		extraPlugins = append(extraPlugins, hookMgr.Plugin())
	}

	var agentToolsets []tool.Toolset
	skillsDir := filepath.Join(cfg.Dir, "skills")
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
		Name:                 build.Name,
		Model:                llm,
		Description:          build.Description,
		Instruction:          instruction,
		Tools:                toolList,
		Toolsets:             agentToolsets,
		BeforeModelCallbacks: beforeModelCallbacks,
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("[agent] created %s agent with %d tools", build.Name, len(toolList))

	checkpoints := tools.GetCheckpoints()
	log.Printf("[agent] loaded %d checkpoints", len(checkpoints))

	return a, extraPlugins, nil
}

// New creates a new Superman agent with all registered tools, optional SOP rules,
// and L1 memory index injected into the system prompt.
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, memSearcher tools.MemorySearcher, sopContent string, expertRegistry *expert.Registry, delegateRunner tools.DelegateRunner) (adkagent.Agent, []*plugin.Plugin, error) {
	return NewFromConfig(llm, cfg, BuildConfig{
		Name:              "superman",
		Description:       "Superman - general-purpose autonomous AI assistant",
		Instruction:       systemPrompt,
		MemoryService:     memSvc,
		MemorySearcher:    memSearcher,
		SOPContent:        sopContent,
		ExpertRegistry:    expertRegistry,
		DelegateRunner:    delegateRunner,
		EnableExpertTools: true,
	})
}

// NewWithoutMemory creates an agent without a memory service (for simple CLI usage).
func NewWithoutMemory(llm model.LLM, cfg *config.Config) (adkagent.Agent, []*plugin.Plugin, error) {
	return New(llm, cfg, nil, nil, "", nil, nil)
}
