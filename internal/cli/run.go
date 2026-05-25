package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"

	superman "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
)

var (
	runPrompt  string
	runFile    string
	runUser    string
	runSession string
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a single prompt and print the response",
	Long:  "Execute a one-shot agent invocation with the given prompt.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg := global.Config()

		prompt, err := runPromptInput(args, os.Stdin)
		if err != nil {
			return err
		}

		llm, err := model.New(ctx, cfg.Model)
		if err != nil {
			return fmt.Errorf("create model: %w", err)
		}
		sessionService, err := supermansession.NewService()
		if err != nil {
			return fmt.Errorf("create session service: %w", err)
		}
		a, extraPlugins, err := superman.New(llm, cfg, nil, sessionService, "", nil, nil, nil)
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

		req := buildRunRequest(cfg, sessionService, prompt)
		if err := ensureRunSession(ctx, sessionService, &req); err != nil {
			return err
		}
		supermansession.RecordPromptReferences(sessionService, req.AppName, req.UserID, req.SessionID, cfg.Workspace, prompt)

		auditLogger := supermanruntime.NewAuditLogger(global.RuntimeEventsPath())
		for event, err := range supermanruntime.StreamRun(ctx, r, req, nil) {
			if err != nil {
				return fmt.Errorf("run error: %w", err)
			}
			if err := writeRunEvent(os.Stdout, auditLogger, event); err != nil {
				return err
			}
		}
		fmt.Println()
		return nil
	},
}

func writeRunEvent(w io.Writer, auditLogger *supermanruntime.AuditLogger, event supermanruntime.Event) error {
	if err := auditLogger.Write(event); err != nil {
		return err
	}
	if event.Type == supermanruntime.EventTextDelta {
		_, err := fmt.Fprint(w, event.Text)
		return err
	}
	return nil
}

func ensureRunSession(ctx context.Context, sessionService *supermansession.Service, req *supermanruntime.RunRequest) error {
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

func init() {
	runCmd.Flags().StringVarP(&runPrompt, "prompt", "p", "", "prompt text")
	runCmd.Flags().StringVarP(&runFile, "file", "f", "", "read prompt from file")
	runCmd.Flags().StringVar(&runUser, "user", "cli-user", "session user id")
	runCmd.Flags().StringVar(&runSession, "session", "", "session id")
}

func runPromptInput(args []string, stdin io.Reader) (string, error) {
	switch {
	case runFile != "":
		data, err := os.ReadFile(runFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(data), nil
	case runPrompt != "":
		return runPrompt, nil
	case len(args) > 0:
		return args[0], nil
	default:
		data, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		if len(data) == 0 {
			return "", fmt.Errorf("no prompt provided: use --prompt, --file, args, or stdin")
		}
		return string(data), nil
	}
}

func buildRunRequest(cfg *config.Config, sessionService *supermansession.Service, prompt string) supermanruntime.RunRequest {
	return supermanruntime.RunRequest{
		AppName:   cfg.Session.AppName,
		UserID:    firstNonEmpty(runUser, "cli-user"),
		SessionID: runSession,
		Message:   genai.NewContentFromText(prompt, genai.RoleUser),
		LoopDetection: supermanruntime.LoopDetectionConfig{
			Enabled:    cfg.Session.LoopDetection.Enabled,
			WindowSize: cfg.Session.LoopDetection.WindowSize,
			MaxRepeats: cfg.Session.LoopDetection.MaxRepeats,
		},
		Compact: supermansession.RuntimeCompactor{
			Service: sessionService,
			Options: supermansession.CompactOptions{
				MaxMessages: cfg.Session.MaxTurns,
				KeepLast:    20,
			},
		},
	}
}
