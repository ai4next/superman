package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	superman "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
)

var (
	runPrompt string
	runFile   string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a single prompt and print the response",
	Long:  "Execute a one-shot agent invocation with the given prompt.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		var prompt string
		if runFile != "" {
			data, err := os.ReadFile(runFile)
			if err != nil {
				return fmt.Errorf("read prompt file: %w", err)
			}
			prompt = string(data)
		} else if runPrompt != "" {
			prompt = runPrompt
		} else if len(args) > 0 {
			prompt = args[0]
		} else {
			data, err := io.ReadAll(os.Stdin)
			if err != nil || len(data) == 0 {
				return fmt.Errorf("no prompt provided: use --prompt, --file, args, or stdin")
			}
			prompt = string(data)
		}

		llm := model.MustNew(ctx, cfg.Model)
		a, extraPlugins, err := superman.NewWithoutMemory(llm, cfg)
		if err != nil {
			return fmt.Errorf("create agent: %w", err)
		}

		adkPlugins := extraPlugins
		for _, pc := range cfg.Plugins {
			if !pc.Enabled {
				continue
			}
			p, err := plugin.Create(pc.Name)
			if err != nil {
				continue
			}
			if p != nil {
				adkPlugins = append(adkPlugins, p)
			}
		}

		sessionService := session.InMemoryService()
		r, err := runner.New(runner.Config{
			Agent:             a,
			AppName:           cfg.Session.AppName,
			SessionService:    sessionService,
			PluginConfig:      runner.PluginConfig{Plugins: adkPlugins},
			AutoCreateSession: true,
		})
		if err != nil {
			return fmt.Errorf("create runner: %w", err)
		}

		msg := genai.NewContentFromText(prompt, "user")
		for evt, err := range r.Run(ctx, "cli-user", "cli-session", msg, adkagent.RunConfig{}) {
			if err != nil {
				return fmt.Errorf("run error: %w", err)
			}
			if evt.Content != nil {
				for _, part := range evt.Content.Parts {
					if part.Text != "" {
						fmt.Print(part.Text)
					}
				}
			}
		}
		fmt.Println()
		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runPrompt, "prompt", "p", "", "prompt text")
	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "read prompt from file")
}
