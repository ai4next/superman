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
	"google.golang.org/adk/session"
	"google.golang.org/genai"

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
	agent  agent.Agent
	stopCh chan struct{}
}

// NewScheduler creates a new Scheduler with the given agent.
func NewScheduler(a agent.Agent) *Scheduler {
	return &Scheduler{
		agent:  a,
		stopCh: make(chan struct{}),
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
	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             s.agent,
		AppName:           global.Config().Session.AppName,
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		log.Printf("[scheduler] runner creation failed: %v", err)
		return
	}

	msg := genai.NewContentFromText(task.Prompt, "user")

	for evt, evtErr := range r.Run(ctx, "scheduler-user", "scheduler-"+task.Name, msg, agent.RunConfig{}) {
		if evtErr != nil {
			log.Printf("[scheduler] task %s error: %v", task.Name, evtErr)
			return
		}
		if evt != nil && evt.Content != nil {
			for _, part := range evt.Content.Parts {
				if part.Text != "" {
					log.Printf("[scheduler] %s output: %s", task.Name, truncate(part.Text, 200))
				}
			}
		}
	}
	log.Printf("[scheduler] task %s completed", task.Name)
}
