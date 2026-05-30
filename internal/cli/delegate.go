package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/bus"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/orchestrator"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/tool"
)

// delegateService runs experts through the same agent builder as Superman.
type delegateService struct {
	llm         model.LLM
	registry    *expert.Registry
	evolutionCh chan<- hook.EvolutionSignal
	queue       bus.TaskQueue
}

func newDelegateService(llm model.LLM, registry *expert.Registry, evolutionCh ...chan<- hook.EvolutionSignal) *delegateService {
	ds := &delegateService{llm: llm, registry: registry}
	if len(evolutionCh) > 0 {
		ds.evolutionCh = evolutionCh[0]
	}
	return ds
}

func newDelegateServiceWithQueue(llm model.LLM, registry *expert.Registry, queue bus.TaskQueue, evolutionCh ...chan<- hook.EvolutionSignal) *delegateService {
	ds := newDelegateService(llm, registry, evolutionCh...)
	ds.queue = queue
	return ds
}

func (ds *delegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	return ds.runDelegateSync(ctx, specName, task)
}

func (ds *delegateService) EnqueueDelegate(ctx context.Context, req tool.DelegateTaskRequest) (tool.DelegateTaskReceipt, error) {
	if ds.queue == nil {
		return tool.DelegateTaskReceipt{}, fmt.Errorf("delegate queue not available")
	}
	receipt, err := ds.queue.Enqueue(bus.Task{
		Type:        "delegate",
		Queue:       "experts",
		MaxAttempts: 2,
		Payload: map[string]string{
			"expert_name": req.ExpertName,
			"task":        req.Task,
		},
	})
	if err != nil {
		return tool.DelegateTaskReceipt{}, err
	}
	return tool.DelegateTaskReceipt{TaskID: receipt.TaskID, Status: string(receipt.Status)}, nil
}

func (ds *delegateService) SubmitPlan(ctx context.Context, planJSON string) (tool.OrchestratorReceipt, error) {
	if ds.queue == nil {
		return tool.OrchestratorReceipt{}, fmt.Errorf("orchestrator queue not available")
	}
	var plan orchestrator.Plan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		return tool.OrchestratorReceipt{}, fmt.Errorf("decode plan: %w", err)
	}
	reconcile, err := orchestrator.Reconcile(&plan, ds.queue)
	if err != nil {
		return tool.OrchestratorReceipt{}, err
	}
	store := orchestrator.FileStore{Dir: global.PlansDir()}
	if err := store.Save(plan); err != nil {
		return tool.OrchestratorReceipt{}, err
	}
	return tool.OrchestratorReceipt{PlanID: plan.ID, Status: string(plan.Status), Queued: len(reconcile.Queued)}, nil
}

func (ds *delegateService) RunQueuedDelegates(ctx context.Context, workerID string) {
	if ds.queue == nil {
		return
	}
	if workerID == "" {
		workerID = "delegate-worker"
	}
	idleTicker := time.NewTicker(time.Second)
	defer idleTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		running, ok, err := ds.queue.Dequeue(bus.WorkerRef{ID: workerID, Queue: "experts", Type: "delegate"})
		if err != nil {
			log.Printf("[delegate] dequeue failed: %v", err)
			waitForDelegateWork(ctx, idleTicker)
			continue
		}
		if !ok {
			waitForDelegateWork(ctx, idleTicker)
			continue
		}
		ds.runQueuedDelegate(ctx, running)
	}
}

func (ds *delegateService) runQueuedDelegate(ctx context.Context, running bus.RunningTask) {
	expertName := running.Task.Payload["expert_name"]
	taskText := running.Task.Payload["task"]
	result, err := ds.runDelegateSync(ctx, expertName, taskText)
	if err != nil {
		if failErr := ds.queue.Fail(running.Task.ID, bus.TaskFailure{Error: err.Error(), Retryable: true}); failErr != nil {
			log.Printf("[delegate] fail task %s: %v", running.Task.ID, failErr)
		}
		return
	}
	if err := ds.queue.Ack(running.Task.ID, bus.TaskResult{
		TaskID:  running.Task.ID,
		Status:  "succeeded",
		Summary: "delegate task completed",
		Result:  result,
	}); err != nil {
		log.Printf("[delegate] ack task %s: %v", running.Task.ID, err)
	}
}

func waitForDelegateWork(ctx context.Context, ticker *time.Ticker) {
	select {
	case <-ctx.Done():
	case <-ticker.C:
	}
}

func (ds *delegateService) runDelegateSync(ctx context.Context, specName string, task string) (string, error) {
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
	auditLogger := bus.NewAuditLogger(global.BusEventsPath())
	var response strings.Builder
	var responseEventID string
	for event, evtErr := range supermanruntime.StreamRun(ctx, r, req, nil) {
		if err := auditLogger.Write(event); err != nil {
			log.Printf("[expert] audit write failed: %v", err)
		}
		if evtErr != nil {
			return "", evtErr
		}
		if event.Type == bus.EventTextDelta && event.Author == a.Name()+"_executor" {
			if event.EventID != "" && event.EventID != responseEventID {
				response.Reset()
				responseEventID = event.EventID
			}
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
