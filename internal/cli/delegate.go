package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

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
	result, err := ds.RunDelegateResult(ctx, specName, expert.NewTask(task, nil))
	if err != nil {
		return "", err
	}
	if result.RawResponse != "" {
		return result.RawResponse, nil
	}
	return result.Summary, nil
}

func (ds *delegateService) RunDelegateResult(ctx context.Context, specName string, task expert.ExpertTask) (*expert.ExpertResult, error) {
	start := time.Now()
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return nil, fmt.Errorf("expert %q not found: %w", specName, err)
	}
	record := func(success bool) {
		if recErr := ds.registry.RecordCall(spec.Name, expert.CallRecord{
			Timestamp:  start,
			TaskDesc:   task.Task,
			Mode:       expert.ModeDelegate,
			Success:    success,
			DurationMs: time.Since(start).Milliseconds(),
		}); recErr != nil {
			log.Printf("[expert] record delegate call warning for %s: %v", spec.Name, recErr)
		}
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
		Instruction:       spec.SystemPrompt + "\n\nReturn a JSON object matching ExpertResult: success, summary, findings, actions_taken, files_touched, tool_calls, confidence, risks, next_steps.",
		MemoryService:     memSvc,
		MemorySearcher:    searchAdapter,
		EnableExpertTools: false,
	})
	if err != nil {
		record(false)
		return nil, fmt.Errorf("create expert agent: %w", err)
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
		record(false)
		return nil, fmt.Errorf("create expert runner: %w", err)
	}

	payload, err := json.Marshal(task)
	if err != nil {
		record(false)
		return nil, fmt.Errorf("marshal expert task: %w", err)
	}
	msg := genai.NewContentFromText(string(payload), "user")
	var response strings.Builder
	for evt, evtErr := range r.Run(ctx, "expert-user", "expert-"+spec.Name, msg, adkagent.RunConfig{}) {
		if evtErr != nil {
			record(false)
			return nil, evtErr
		}
		if evt.Content != nil {
			for _, part := range evt.Content.Parts {
				response.WriteString(part.Text)
			}
		}
	}
	result := parseExpertResult(response.String())
	record(result.Success)
	return result, nil
}

func parseExpertResult(raw string) *expert.ExpertResult {
	raw = strings.TrimSpace(raw)
	result := &expert.ExpertResult{
		Success:     true,
		Summary:     raw,
		Confidence:  0.5,
		RawResponse: raw,
	}
	if raw == "" {
		result.Success = false
		result.Confidence = 0
		return result
	}
	var parsed expert.ExpertResult
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		parsed.RawResponse = raw
		if parsed.Confidence == 0 {
			parsed.Confidence = 0.5
		}
		return &parsed
	}
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			if err := json.Unmarshal([]byte(raw[start:end+1]), &parsed); err == nil {
				parsed.RawResponse = raw
				if parsed.Confidence == 0 {
					parsed.Confidence = 0.5
				}
				return &parsed
			}
		}
	}
	return result
}
