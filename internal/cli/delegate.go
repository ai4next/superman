package cli

import (
	"context"
	"fmt"
	"log"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
)

// delegateService runs experts through the same agent builder as Superman.
type delegateService struct {
	llm         model.LLM
	registry    *expert.Registry
	evolutionCh chan<- hook.EvolutionSignal
}

func newDelegateService(llm model.LLM, registry *expert.Registry, evolutionCh ...chan<- hook.EvolutionSignal) *delegateService {
	ds := &delegateService{llm: llm, registry: registry}
	if len(evolutionCh) > 0 {
		ds.evolutionCh = evolutionCh[0]
	}
	return ds
}

func (ds *delegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return "", fmt.Errorf("expert %q not found: %w", specName, err)
	}

	cfg := global.Config()
	memSvc := memory.NewExpert(spec.Name)
	if err := memSvc.LoadFromDisk(); err != nil {
		return "", fmt.Errorf("load expert memory: %w", err)
	}
	sessionService, err := supermansession.NewServiceInRoot(global.ExpertDir(spec.Name))
	if err != nil {
		return "", fmt.Errorf("create expert session service: %w", err)
	}
	a, extraPlugins, err := agent.NewFromConfig(ds.llm, cfg, agent.BuildConfig{
		Name:              spec.Name,
		Instruction:       spec.SystemPrompt + "\n\nReturn only a concise plain-text summary of the delegated task result.",
		MemoryService:     memSvc,
		SessionService:    sessionService,
		ContextMessages:   8,
		EnableExpertTools: false,
		EvolutionSignal: hook.EvolutionSignal{
			UserID:    "expert-user",
			AgentName: spec.Name,
			Role:      "expert",
			RootDir:   global.ExpertDir(spec.Name),
		},
		EvolutionCh: ds.evolutionCh,
	})
	if err != nil {
		return "", fmt.Errorf("create expert agent: %w", err)
	}

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

	req := buildDelegateRunRequest(cfg, sessionService, spec.Name, task)
	if err := ensureRunSession(ctx, sessionService, &req); err != nil {
		return "", err
	}
	auditLogger := supermanruntime.NewAuditLogger(global.RuntimeEventsPath())
	var response strings.Builder
	for event, evtErr := range supermanruntime.StreamRun(ctx, r, req, nil) {
		if err := auditLogger.Write(event); err != nil {
			log.Printf("[expert] audit write failed: %v", err)
		}
		if evtErr != nil {
			return "", evtErr
		}
		if event.Type == supermanruntime.EventTextDelta {
			response.WriteString(event.Text)
		}
	}
	result := strings.TrimSpace(response.String())
	if result == "" {
		return "", fmt.Errorf("delegate returned an empty response")
	}
	return result, nil
}

func buildDelegateRunRequest(cfg *config.Config, sessionService adksession.Service, expertName string, task string) supermanruntime.RunRequest {
	return supermanruntime.RunRequest{
		AppName:    cfg.Session.AppName + "-expert",
		UserID:     "expert-user",
		Message:    genai.NewContentFromText(task, genai.RoleUser),
		StateDelta: supermanruntime.PromptStateDelta(cfg.Workspace, task),
		LoopDetection: supermanruntime.LoopDetectionConfig{
			Enabled:    cfg.Session.LoopDetection.Enabled,
			WindowSize: cfg.Session.LoopDetection.WindowSize,
			MaxRepeats: cfg.Session.LoopDetection.MaxRepeats,
		},
		Compact: supermanruntime.SessionCompactor(sessionService, cfg.Session.MaxTurns),
	}
}
