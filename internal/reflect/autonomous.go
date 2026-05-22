package reflect

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/ai4next/superman/internal/config"
)

// IdleWatcher monitors user activity and triggers autonomous reflection
// after a configurable idle timeout.
type IdleWatcher struct {
	agent       agent.Agent
	cfg         *config.Config
	lastActive  time.Time
	mu          sync.Mutex
	idleTimeout time.Duration
	stopCh      chan struct{}
}

// NewIdleWatcher creates a new IdleWatcher with the given agent and configuration.
func NewIdleWatcher(a agent.Agent, cfg *config.Config) *IdleWatcher {
	return &IdleWatcher{
		agent:       a,
		cfg:         cfg,
		lastActive:  time.Now(),
		idleTimeout: cfg.Reflect.Autonomous.IdleTimeout.AsDuration(),
		stopCh:      make(chan struct{}),
	}
}

// Touch resets the idle timer. Call this whenever user activity is detected.
func (w *IdleWatcher) Touch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastActive = time.Now()
}

// Start begins the idle watcher loop. It checks every minute whether the
// idle timeout has been exceeded and triggers an autonomous reflection run.
func (w *IdleWatcher) Start(ctx context.Context) {
	log.Printf("[reflect] idle watcher started (timeout: %s)", w.idleTimeout)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.mu.Lock()
			idleDuration := time.Since(w.lastActive)
			w.mu.Unlock()

			if idleDuration >= w.idleTimeout {
				log.Printf("[reflect] idle for %s, triggering autonomous run", idleDuration)
				w.execute(ctx)
				w.Touch()
			}
		}
	}
}

// Stop signals the idle watcher to stop.
func (w *IdleWatcher) Stop() {
	close(w.stopCh)
}

func (w *IdleWatcher) execute(ctx context.Context) {
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             w.agent,
		AppName:           w.cfg.Session.AppName,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		log.Printf("[reflect] runner creation failed: %v", err)
		return
	}

	prompt := "Review any pending tasks and check if there's anything that needs attention."
	msg := genai.NewContentFromText(prompt, "user")

	for evt, evtErr := range r.Run(ctx, "reflect-user", "reflect-session", msg, agent.RunConfig{}) {
		if evtErr != nil {
			log.Printf("[reflect] execution error: %v", evtErr)
			return
		}
		if evt != nil && evt.Content != nil {
			for _, part := range evt.Content.Parts {
				if part.Text != "" {
					log.Printf("[reflect] output: %s", truncate(part.Text, 200))
				}
			}
		}
	}
	log.Printf("[reflect] autonomous run completed")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
