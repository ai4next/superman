package expert

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai4next/superman/internal/config"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// DelegateService runs a task using an expert's system prompt.
type DelegateService struct {
	cfg      *config.Config
	llm      model.LLM
	registry *Registry
}

// NewDelegateService creates a new delegate service.
func NewDelegateService(cfg *config.Config, llm model.LLM, registry *Registry) *DelegateService {
	return &DelegateService{cfg: cfg, llm: llm, registry: registry}
}

// RunDelegate executes a task using the named expert's system prompt.
func (ds *DelegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return "", fmt.Errorf("expert %q not found: %w", specName, err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "expert-" + spec.Name,
		Model:       ds.llm,
		Description: spec.Summary,
		Instruction: spec.SystemPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("create expert agent: %w", err)
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           ds.cfg.Session.AppName + "-expert",
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", fmt.Errorf("create expert runner: %w", err)
	}

	msg := genai.NewContentFromText(task, "user")
	var response strings.Builder
	for evt, evtErr := range r.Run(ctx, "expert-user", "expert-"+spec.Name, msg, agent.RunConfig{}) {
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
