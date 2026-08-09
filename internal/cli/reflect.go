package cli

import (
	"fmt"
	"log"
	"sync"

	"github.com/spf13/cobra"
	"google.golang.org/adk/runner"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/reflect"
	supermansession "github.com/ai4next/superman/internal/session"
)

var reflectCmd = &cobra.Command{
	Use:   "reflect",
	Short: "Start autonomous reflection mode",
	Long:  "Start the agent in autonomous mode, monitoring for idle and running scheduled tasks.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cfg := global.Config()

		llm, err := model.New(ctx, cfg.Model)
		if err != nil {
			return fmt.Errorf("create model: %w", err)
		}
		sessionService, err := supermansession.NewService()
		if err != nil {
			return fmt.Errorf("create session service: %w", err)
		}
		a, extraPlugins, err := agent.New(llm, cfg, nil, sessionService, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}
		pluginCfg := runner.PluginConfig{Plugins: extraPlugins}

		watcher := reflect.NewIdleWatcherWithPlugins(a, sessionService, pluginCfg)
		scheduler := reflect.NewSchedulerWithPlugins(a, sessionService, pluginCfg)
		var runWG sync.WaitGroup
		runWG.Add(2)
		go func() {
			defer runWG.Done()
			watcher.Start(ctx)
		}()
		go func() {
			defer runWG.Done()
			scheduler.Start(ctx)
		}()

		log.Printf("[reflect] autonomous mode started with model %s/%s", cfg.Model.Provider, cfg.Model.Name)
		log.Printf("[reflect] idle timeout: %s", cfg.Reflect.Autonomous.IdleTimeout.AsDuration())
		log.Printf("[reflect] tasks dir: %s", cfg.Reflect.Scheduler.TasksDir)
		log.Printf("[reflect] press Ctrl+C to stop")

		<-ctx.Done()
		watcher.Stop()
		scheduler.Stop()
		runWG.Wait()
		log.Printf("[reflect] stopped")
		return nil
	},
}
