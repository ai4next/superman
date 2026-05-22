package agent

import (
	"context"
	_ "embed"
	"log"
	"os"
	"path/filepath"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/skilltoolset"
	"google.golang.org/adk/tool/skilltoolset/skill"

	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
)

//go:embed prompt/system.txt
var systemPrompt string

// New creates a new Superman agent with all registered tools, optional SOP rules,
// and L1 memory index injected into the system prompt.
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, memSearcher tools.MemorySearcher, sopContent string, expertRegistry *expert.Registry, delegateRunner tools.DelegateRunner) (agent.Agent, []*plugin.Plugin, error) {
	deps := tools.Dependencies{
		Config:         cfg,
		Workspace:      cfg.Tools.CodeRun.Workspace,
		MemoryService:  memSvc,
		MemorySearcher: memSearcher,
		ExpertManager:  expertRegistry,
		DelegateRunner: delegateRunner,
	}

	toolList := tools.RegisterAll(deps)

	instruction := systemPrompt

	// Inject L1 memory index into the system prompt
	if memSvc != nil {
		l1Content := memSvc.GetL1Content()
		if l1Content != "" {
			instruction += "\n\n## Memory Index\n" + l1Content
		}
	}

	// Inject L0 SOP rules into the system prompt
	if sopContent != "" {
		instruction += "\n\n## SOP Rules\n" + sopContent
	}

	// Hook manager plugin
	var extraPlugins []*plugin.Plugin
	hookMgr, err := hook.NewManager(filepath.Join(cfg.Dir, "hooks"))
	if err != nil {
		log.Printf("[agent] hook manager: %v", err)
	} else {
		extraPlugins = append(extraPlugins, hookMgr.Plugin())
	}

	// Create SkillToolset from cfg.Dir/skills directory
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
		Name:        "superman",
		Model:       llm,
		Description: "Superman - general-purpose autonomous AI assistant",
		Instruction: instruction,
		Tools:       toolList,
		Toolsets:    agentToolsets,
	})
	if err != nil {
		return nil, nil, err
	}

	log.Printf("[agent] created superman agent with %d tools", len(toolList))

	checkpoints := tools.GetCheckpoints()
	log.Printf("[agent] loaded %d checkpoints", len(checkpoints))

	return a, extraPlugins, nil
}

// NewWithoutMemory creates an agent without a memory service (for simple CLI usage).
func NewWithoutMemory(llm model.LLM, cfg *config.Config) (agent.Agent, []*plugin.Plugin, error) {
	return New(llm, cfg, nil, nil, "", nil, nil)
}