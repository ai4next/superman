package agent

import (
	_ "embed"
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"

	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/memory"
)

//go:embed prompt/system.txt
var systemPrompt string

// New creates a new Superman agent with all registered tools, optional SOP rules,
// and L1 memory index injected into the system prompt.
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, memSearcher tools.MemorySearcher, sopContent string) (agent.Agent, error) {
	deps := tools.Dependencies{
		Config:         cfg,
		Workspace:      cfg.Tools.CodeRun.Workspace,
		MemoryService:  memSvc,
		MemorySearcher: memSearcher,
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

	a, err := llmagent.New(llmagent.Config{
		Name:        "superman",
		Model:       llm,
		Description: "Superman - general-purpose autonomous AI assistant",
		Instruction: instruction,
		Tools:       toolList,
	})
	if err != nil {
		return nil, err
	}

	log.Printf("[agent] created superman agent with %d tools", len(toolList))

	checkpoints := tools.GetCheckpoints()
	log.Printf("[agent] loaded %d checkpoints", len(checkpoints))

	return a, nil
}

// NewWithoutMemory creates an agent without a memory service (for simple CLI usage).
func NewWithoutMemory(llm model.LLM, cfg *config.Config) (agent.Agent, error) {
	return New(llm, cfg, nil, nil, "")
}