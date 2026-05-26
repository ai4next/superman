package reflect

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/global"
)

// ScheduleTask defines a scheduled reflection task loaded from a JSON file.
type ScheduleTask struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	Prompt   string `json:"prompt"`
	Enabled  bool   `json:"enabled"`
}

// Scheduler manages periodic reflection tasks. It reads task definitions
// from JSON files in a configured directory and executes enabled tasks.
type Scheduler struct {
	agent     agent.Agent
	sessions  adksession.Service
	pluginCfg runner.PluginConfig
	stopCh    chan struct{}
}

// NewScheduler creates a new Scheduler with the given agent.
func NewScheduler(a agent.Agent, sessions adksession.Service) *Scheduler {
	return NewSchedulerWithPlugins(a, sessions, runner.PluginConfig{})
}

// NewSchedulerWithPlugins creates a scheduler with ADK plugins preserved.
func NewSchedulerWithPlugins(a agent.Agent, sessions adksession.Service, pluginCfg runner.PluginConfig) *Scheduler {
	return &Scheduler{
		agent:     a,
		sessions:  sessions,
		pluginCfg: pluginCfg,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the scheduler loop. It checks for enabled tasks every 5 minutes.
func (s *Scheduler) Start(ctx context.Context) {
	cfg := global.Config()
	tasksDir := cfg.Reflect.Scheduler.TasksDir
	log.Printf("[scheduler] starting, tasks dir: %s", tasksDir)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			tasks := s.loadTasks(tasksDir)
			for _, task := range tasks {
				if task.Enabled {
					log.Printf("[scheduler] executing task: %s", task.Name)
					s.executeTask(ctx, task)
				}
			}
		}
	}
}

// Stop signals the scheduler to stop.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loadTasks(dir string) []ScheduleTask {
	var tasks []ScheduleTask
	entries, err := os.ReadDir(dir)
	if err != nil {
		return tasks
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var task ScheduleTask
		if err := json.Unmarshal(data, &task); err != nil {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func (s *Scheduler) executeTask(ctx context.Context, task ScheduleTask) {
	cfg := global.Config()
	_, err := newExecutor(s.agent, s.sessions, s.pluginCfg).run(ctx, cfg, "scheduler-user", "", task.Prompt, "scheduler")
	if err != nil {
		log.Printf("[scheduler] task %s error: %v", task.Name, err)
		return
	}
	log.Printf("[scheduler] task %s completed", task.Name)
}
