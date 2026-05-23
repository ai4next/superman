package cli

import (
	"context"
	"fmt"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
)

// delegateService runs experts through the same agent builder as Superman.
type delegateService struct {
	llm         model.LLM
	registry    *expert.Registry
	evolutionCh chan<- hook.EvolutionSignal
}

func newDelegateService(llm model.LLM, registry *expert.Registry, evolutionCh chan<- hook.EvolutionSignal) *delegateService {
	return &delegateService{llm: llm, registry: registry, evolutionCh: evolutionCh}
}

func (ds *delegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return "", fmt.Errorf("expert %q not found: %w", specName, err)
	}

	cfg := global.Config()
	expertMemoryDir := cfg.ExpertMemoryDir(spec.Name)
	memSvc := memory.New(expertMemoryDir)
	if err := memSvc.LoadFromDisk(); err != nil {
		return "", fmt.Errorf("load expert memory: %w", err)
	}
	a, extraPlugins, err := agent.NewFromConfig(ds.llm, cfg, agent.BuildConfig{
		Name:              spec.Name,
		Description:       spec.Name,
		Instruction:       spec.SystemPrompt + "\n\nReturn only a concise plain-text summary of the delegated task result.",
		MemoryService:     memSvc,
		EnableExpertTools: false,
		EvolutionSignal: hook.EvolutionSignal{
			Role:      "expert",
			MemoryDir: expertMemoryDir,
		},
		EvolutionCh: ds.evolutionCh,
	})
	if err != nil {
		return "", fmt.Errorf("create expert agent: %w", err)
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           cfg.Session.AppName + "-expert",
		SessionService:    sessionService,
		PluginConfig:      runner.PluginConfig{Plugins: extraPlugins},
		AutoCreateSession: true,
	})
	if err != nil {
		return "", fmt.Errorf("create expert runner: %w", err)
	}

	msg := genai.NewContentFromText(task, "user")
	var response strings.Builder
	for evt, evtErr := range r.Run(ctx, "expert-user", "expert-"+spec.Name, msg, adkagent.RunConfig{}) {
		if evtErr != nil {
			return "", evtErr
		}
		if evt.Content != nil {
			for _, part := range evt.Content.Parts {
				response.WriteString(part.Text)
			}
		}
	}
	result := strings.TrimSpace(response.String())
	if result == "" {
		return "", fmt.Errorf("delegate returned an empty response")
	}
	return result, nil
}
