package agent

import (
	"fmt"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/prompt"
	supermantool "github.com/ai4next/superman/internal/tool"
)

const (
	SupermanEvolverName = "superman-evolver"
	ExpertEvolverName   = "expert-evolver"
	MetaEvolverName     = "meta-evolver"
	EvolverName         = SupermanEvolverName
)

func NewEvolver(llm model.LLM, cfg *config.Config, memSvc *memory.Service) (adkagent.Agent, error) {
	return newEvolver(llm, cfg, memSvc, SupermanEvolverName, prompt.SupermanEvolverSystem())
}

func NewExpertEvolver(llm model.LLM, cfg *config.Config, memSvc *memory.Service) (adkagent.Agent, error) {
	return newEvolver(llm, cfg, memSvc, ExpertEvolverName, prompt.ExpertEvolverSystem())
}

func NewMetaEvolver(llm model.LLM, cfg *config.Config, memSvc *memory.Service) (adkagent.Agent, error) {
	return newEvolver(llm, cfg, memSvc, MetaEvolverName, prompt.MetaEvolverSystem())
}

func newEvolver(llm model.LLM, cfg *config.Config, memSvc *memory.Service, name string, instruction string) (adkagent.Agent, error) {
	a, err := llmagent.New(llmagent.Config{
		Name:                name,
		Model:               llm,
		InstructionProvider: evolverInstructionProvider(instruction, memSvc),
		Tools:               evolverTools(cfg),
	})
	if err != nil {
		return nil, fmt.Errorf("create evolution agent: %w", err)
	}
	return a, nil
}

func evolverTools(cfg *config.Config) []tool.Tool {
	return supermantool.RegisterAll(supermantool.Dependencies{Config: cfg, EvolverTools: true})
}

func evolverInstructionProvider(instruction string, memSvc *memory.Service) llmagent.InstructionProvider {
	return func(adkagent.ReadonlyContext) (string, error) {
		builder := strings.Builder{}
		builder.WriteString(instruction)
		if memSvc != nil {
			if l0Content := memSvc.GetL0Content(); l0Content != "" {
				builder.WriteString("\n\n")
				builder.WriteString(l0Content)
			}
		}
		return strings.TrimSpace(builder.String()), nil
	}
}
