package agent

import (
	_ "embed"
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"

	"github.com/ai4next/superman/internal/agent/tools"
	"github.com/ai4next/superman/internal/config"
)

//go:embed prompt/system.txt
var systemPrompt string

// New creates a new Superman agent with all registered tools and optional SOP rules.
// sopContent is appended to the system prompt when provided.
func New(llm model.LLM, cfg *config.Config, memory tools.MemoryStorer, sopContent string) (agent.Agent, error) {
	deps := tools.Dependencies{
		Config:        cfg,
		Workspace:     cfg.Tools.CodeRun.Workspace,
		MemoryService: memory,
	}

	toolList := tools.RegisterAll(deps)

	instruction := systemPrompt
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
	return New(llm, cfg, nil, "")
}