package cli

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/memory"
)

// delegateService runs experts through the same agent builder as Superman,
// while keeping each expert's memory isolated.
type delegateService struct {
	cfg      *config.Config
	llm      model.LLM
	registry *expert.Registry
}

func newDelegateService(cfg *config.Config, llm model.LLM, registry *expert.Registry) *delegateService {
	return &delegateService{cfg: cfg, llm: llm, registry: registry}
}

func (ds *delegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return "", fmt.Errorf("expert %q not found: %w", specName, err)
	}

	expertMemoryDir := filepath.Join(ds.cfg.Dir, "experts", spec.Name, "memory")
	memSvc := memory.New(ds.cfg.Memory.L1.MaxEntries, expertMemoryDir)
	if err := memSvc.LoadFromDisk(); err != nil {
		log.Printf("[expert] memory load warning for %s: %v", spec.Name, err)
	}
	searchAdapter := &memorySearchAdapter{svc: memSvc}

	a, extraPlugins, err := agent.NewFromConfig(ds.llm, ds.cfg, agent.BuildConfig{
		Name:              "expert-" + spec.Name,
		Description:       spec.Summary,
		Instruction:       spec.SystemPrompt,
		MemoryService:     memSvc,
		MemorySearcher:    searchAdapter,
		EnableExpertTools: false,
	})
	if err != nil {
		return "", fmt.Errorf("create expert agent: %w", err)
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           ds.cfg.Session.AppName + "-expert",
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
	return response.String(), nil
}
