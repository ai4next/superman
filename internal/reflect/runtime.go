package reflect

import (
	"context"
	"fmt"
	"log"
	"strings"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
)

type executor struct {
	agent     adkagent.Agent
	sessions  adksession.Service
	pluginCfg runner.PluginConfig
}

func newExecutor(a adkagent.Agent, sessions adksession.Service, pluginCfg runner.PluginConfig) executor {
	return executor{agent: a, sessions: sessions, pluginCfg: pluginCfg}
}

func (e executor) run(ctx context.Context, cfg *config.Config, userID, sessionID, prompt, logPrefix string) (string, error) {
	sessionService, err := e.sessionService(cfg)
	if err != nil {
		return "", err
	}
	r, err := runner.New(runner.Config{
		Agent:             e.agent,
		AppName:           cfg.Session.AppName,
		SessionService:    sessionService,
		PluginConfig:      e.pluginCfg,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", fmt.Errorf("create runner: %w", err)
	}

	req := reflectRunRequest(cfg, sessionService, userID, sessionID, prompt)
	if err := ensureSession(ctx, sessionService, &req); err != nil {
		return "", err
	}

	auditLogger := runtime.NewAuditLogger(global.RuntimeEventsPath())
	var response strings.Builder
	for event, evtErr := range runtime.StreamRun(ctx, r, req, nil) {
		if err := auditLogger.Write(event); err != nil {
			log.Printf("[%s] audit write failed: %v", logPrefix, err)
		}
		if evtErr != nil {
			return response.String(), evtErr
		}
		if event.Type == runtime.EventTextDelta {
			response.WriteString(event.Text)
			if event.Text != "" {
				log.Printf("[%s] output: %s", logPrefix, truncate(event.Text, 200))
			}
		}
	}
	return strings.TrimSpace(response.String()), nil
}

func (e executor) sessionService(cfg *config.Config) (adksession.Service, error) {
	if e.sessions != nil {
		return e.sessions, nil
	}
	svc, err := supermansession.NewService()
	if err != nil {
		return nil, fmt.Errorf("create session service: %w", err)
	}
	return svc, nil
}

func reflectRunRequest(cfg *config.Config, sessionService adksession.Service, userID, sessionID, prompt string) runtime.RunRequest {
	return runtime.RunRequest{
		AppName:    cfg.Session.AppName,
		UserID:     userID,
		SessionID:  sessionID,
		Message:    genai.NewContentFromText(prompt, genai.RoleUser),
		StateDelta: runtime.PromptStateDelta(cfg.Workspace, prompt),
		LoopDetection: runtime.LoopDetectionConfig{
			Enabled:    cfg.Session.LoopDetection.Enabled,
			WindowSize: cfg.Session.LoopDetection.WindowSize,
			MaxRepeats: cfg.Session.LoopDetection.MaxRepeats,
		},
		Compact: runtime.SessionCompactor(sessionService, cfg.Session.MaxTurns),
	}
}

func ensureSession(ctx context.Context, sessionService adksession.Service, req *runtime.RunRequest) error {
	if sessionService == nil {
		return nil
	}
	if _, err := sessionService.Get(ctx, &adksession.GetRequest{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	}); err == nil {
		return nil
	}
	created, err := sessionService.Create(ctx, &adksession.CreateRequest{
		AppName:   req.AppName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		return fmt.Errorf("create session %s: %w", req.SessionID, err)
	}
	req.SessionID = created.Session.ID()
	return nil
}
