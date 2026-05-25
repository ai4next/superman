package reflect

import (
	"context"
	"log"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"

	"github.com/ai4next/superman/internal/global"
	supermansession "github.com/ai4next/superman/internal/session"
)

// IdleWatcher monitors user activity and triggers autonomous reflection
// after a configurable idle timeout.
type IdleWatcher struct {
	agent       agent.Agent
	sessions    *supermansession.Service
	pluginCfg   runner.PluginConfig
	lastActive  time.Time
	mu          sync.Mutex
	idleTimeout time.Duration
	stopCh      chan struct{}
}

// NewIdleWatcher creates a new IdleWatcher with the given agent.
func NewIdleWatcher(a agent.Agent, sessions *supermansession.Service) *IdleWatcher {
	return NewIdleWatcherWithPlugins(a, sessions, runner.PluginConfig{})
}

// NewIdleWatcherWithPlugins creates an idle watcher with ADK plugins preserved.
func NewIdleWatcherWithPlugins(a agent.Agent, sessions *supermansession.Service, pluginCfg runner.PluginConfig) *IdleWatcher {
	cfg := global.Config()
	return &IdleWatcher{
		agent:       a,
		sessions:    sessions,
		pluginCfg:   pluginCfg,
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
	cfg := global.Config()
	prompt := "Review any pending tasks and check if there's anything that needs attention."
	_, err := newExecutor(w.agent, w.sessions, w.pluginCfg).run(ctx, cfg, "reflect-user", "", prompt, "reflect")
	if err != nil {
		log.Printf("[reflect] execution error: %v", err)
		return
	}
	log.Printf("[reflect] autonomous run completed")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
