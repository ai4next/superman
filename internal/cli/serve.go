package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/memory"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	"github.com/ai4next/superman/internal/session"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the TUI chat interface",
	Long:  "Start the interactive terminal UI for chatting with the Superman agent.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Model
		llm := model.MustNew(ctx, cfg.Model)

		// Memory service (L0-L3)
		memSvc := memory.New(cfg.Memory.L1.MaxEntries)

		// L0 SOP store — load templates and inject into agent prompt
		var sopContent string
		l0, err := memory.NewL0Store(cfg.Memory.L0.SOPDir)
		if err != nil {
			log.Printf("[cli] L0Store warning: %v", err)
		}
		if l0 != nil {
			rules := l0.All()
			if len(rules) > 0 {
				log.Printf("[cli] loaded %d L0 SOP rules", len(rules))
				for name, content := range rules {
					sopContent += "\n### " + name + "\n" + content + "\n"
					log.Printf("[cli]   SOP: %s", name)
				}
			}
		}

		// Periodic archiving (L2 → L3)
		go func() {
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					if archived, _ := memSvc.Archive(ctx, 48*time.Hour); archived > 0 {
						log.Printf("[memory] archived %d entries", archived)
					}
				}
			}
		}()

		// Session manager with JSONL persistence
		sessMgr := session.New(adksession.InMemoryService(), cfg.Session.HistoryPath, cfg.Session.MaxTurns)

		// Plugins
		var adkPlugins []*adkplugin.Plugin
		for _, pc := range cfg.Plugins {
			if !pc.Enabled {
				continue
			}
			p, err := plugin.Create(pc.Name)
			if err != nil {
				log.Printf("[cli] plugin %s skipped: %v", pc.Name, err)
				continue
			}
			if p != nil {
				adkPlugins = append(adkPlugins, p)
			}
		}

		// Agent with memory service and SOP templates
		a, err := agent.New(llm, cfg, memSvc, sopContent)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}

		log.Printf("[cli] starting TUI with model %s/%s (%d plugins, sessions: %s)",
			cfg.Model.Provider, cfg.Model.Name, len(adkPlugins), cfg.Session.HistoryPath)
		return runTUI(ctx, a, cfg, runner.PluginConfig{Plugins: adkPlugins}, sessMgr)
	},
}