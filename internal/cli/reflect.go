package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/reflect"
)

var reflectCmd = &cobra.Command{
	Use:   "reflect",
	Short: "Start autonomous reflection mode",
	Long:  "Start the agent in autonomous mode, monitoring for idle and running scheduled tasks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfg := global.Config()

		llm, err := model.New(ctx, cfg.Model)
		if err != nil {
			return fmt.Errorf("create model: %w", err)
		}
		a, _, err := agent.NewWithoutMemory(llm, cfg)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}

		// Start idle watcher
		watcher := reflect.NewIdleWatcher(a)
		go watcher.Start(ctx)

		// Start task scheduler
		scheduler := reflect.NewScheduler(a)
		go scheduler.Start(ctx)

		log.Printf("[reflect] autonomous mode started with model %s/%s", cfg.Model.Provider, cfg.Model.Name)
		log.Printf("[reflect] idle timeout: %s", cfg.Reflect.Autonomous.IdleTimeout.AsDuration())
		log.Printf("[reflect] tasks dir: %s", cfg.Reflect.Scheduler.TasksDir)
		log.Printf("[reflect] press Ctrl+C to stop")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { watcher.Stop(); wg.Done() }()
		go func() { scheduler.Stop(); wg.Done() }()
		wg.Wait()
		log.Printf("[reflect] stopped")
		return nil
	},
}
