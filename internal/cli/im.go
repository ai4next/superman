package cli

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"
	adkagent "google.golang.org/adk/agent"
	adkmodel "google.golang.org/adk/model"
	adkplugin "google.golang.org/adk/plugin"
	"google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"

	"github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/expert"
	"github.com/ai4next/superman/internal/global"
	"github.com/ai4next/superman/internal/hook"
	"github.com/ai4next/superman/internal/im"
	"github.com/ai4next/superman/internal/im/weixinsetup"
	"github.com/ai4next/superman/internal/memory"
	supermanmodel "github.com/ai4next/superman/internal/model"
	"github.com/ai4next/superman/internal/plugin"
	supermanruntime "github.com/ai4next/superman/internal/runtime"
	supermansession "github.com/ai4next/superman/internal/session"
	"github.com/ai4next/superman/internal/task"
	"github.com/ai4next/superman/internal/tool"
)

var imCmd = &cobra.Command{
	Use:   "im",
	Short: "Run Superman through instant-messaging platforms",
}

var imServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the IM integration server",
	Long:  "Run a long-lived server process that connects Superman to configured instant-messaging platforms.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveIM(cmd.Context())
	},
}

var imWeixinCmd = &cobra.Command{
	Use:   "weixin",
	Short: "Configure Weixin personal-account integration",
}

var imWeixinSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Scan a Weixin QR code and print integration credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runIMWeixinSetup(cmd)
	},
}

func init() {
	addWeixinSetupFlags(imWeixinSetupCmd)
	imWeixinCmd.AddCommand(imWeixinSetupCmd)
	imCmd.AddCommand(imServeCmd, imWeixinCmd)
}

func addWeixinSetupFlags(cmd *cobra.Command) {
	cmd.Flags().String("api-url", weixinsetup.DefaultAPIURL, "ilink API base URL")
	cmd.Flags().Int("timeout", 480, "QR login timeout in seconds")
	cmd.Flags().String("qr-image", "", "save QR code as PNG")
	cmd.Flags().String("route-tag", "", "optional SKRouteTag header")
	cmd.Flags().String("bot-type", weixinsetup.DefaultBotType, "get_bot_qrcode bot_type")
	cmd.Flags().Bool("debug", false, "print debug HTTP logs")
}

func runIMWeixinSetup(cmd *cobra.Command) error {
	apiURL, _ := cmd.Flags().GetString("api-url")
	timeoutSecs, _ := cmd.Flags().GetInt("timeout")
	qrImage, _ := cmd.Flags().GetString("qr-image")
	routeTag, _ := cmd.Flags().GetString("route-tag")
	botType, _ := cmd.Flags().GetString("bot-type")
	debug, _ := cmd.Flags().GetBool("debug")

	result, err := weixinsetup.RunQRLoginFlow(cmd.Context(), weixinsetup.QRLoginOptions{
		APIBaseURL: apiURL,
		Timeout:    time.Duration(timeoutSecs) * time.Second,
		QRImage:    qrImage,
		RouteTag:   routeTag,
		BotType:    botType,
		Debug:      debug,
		Out:        cmd.OutOrStdout(),
		Err:        cmd.ErrOrStderr(),
	})
	if err != nil {
		return err
	}

	baseURL := result.BaseURL
	if baseURL == "" {
		baseURL = apiURL
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Weixin credentials:")
	fmt.Fprintf(cmd.OutOrStdout(), "token: %s\n", result.BotToken)
	fmt.Fprintf(cmd.OutOrStdout(), "base_url: %s\n", strings.TrimRight(baseURL, "/"))
	if result.IlinkBotID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "account_id: %s\n", result.IlinkBotID)
	}
	if result.IlinkUserID != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "allow_from: %s\n", result.IlinkUserID)
	}
	return nil
}

func serveIM(ctx context.Context) error {
	cfg := global.Config()
	if len(cfg.IM.Platforms) == 0 {
		return fmt.Errorf("no IM platforms configured; add im.platforms to config.yaml")
	}

	llm, err := supermanmodel.New(ctx, cfg.Model)
	if err != nil {
		return fmt.Errorf("create model: %w", err)
	}
	memSvc := memory.New()
	if err := memSvc.LoadFromDisk(); err != nil {
		log.Printf("[im] memory load warning: %v", err)
	}
	sessionService, err := supermansession.NewService()
	if err != nil {
		return fmt.Errorf("create session service: %w", err)
	}

	evolution, err := task.NewEvolution(llm, sessionService)
	if err != nil {
		return fmt.Errorf("create evolution agent: %w", err)
	}
	evolutionBroker := supermanruntime.NewBroker()
	evolution.SetBroker(evolutionBroker)
	supermanruntime.NewAuditLogger(global.RuntimeEventsPath()).Subscribe(ctx, evolutionBroker)
	go evolution.Loop(ctx)

	expertRegistry, delegateRunner := loadIMExperts(llm)
	a, adkPlugins, err := buildIMAgent(llm, cfg, memSvc, sessionService, expertRegistry, delegateRunner, evolution.SignalCh())
	if err != nil {
		return err
	}

	run, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           cfg.Session.AppName,
		SessionService:    sessionService,
		PluginConfig:      runner.PluginConfig{Plugins: adkPlugins},
		AutoCreateSession: true,
	})
	if err != nil {
		return fmt.Errorf("create runner: %w", err)
	}

	logger := slog.Default()
	bridge, err := im.NewAgentBridge(run, sessionService, cfg, logger)
	if err != nil {
		return err
	}
	client, err := im.NewClientFromConfig(imConfigFromApp(cfg), bridge.Handler(), im.WithLogger(logger))
	if err != nil {
		return err
	}
	log.Printf("[im] serving %d platform(s): %s", len(client.Platforms()), strings.Join(platformNames(client.Platforms()), ", "))
	return client.Run(ctx)
}

func loadIMExperts(llm adkmodel.LLM) (*expert.Registry, tool.DelegateRunner) {
	cfg := global.Config()
	if !cfg.Expert.Enabled {
		return nil, nil
	}
	registry := expert.NewRegistry(global.ExpertsDir())
	if err := registry.LoadFromDisk(); err != nil {
		log.Printf("[expert] load warning: %v", err)
	}
	log.Printf("[expert] loaded %d experts", len(registry.List()))
	return registry, newDelegateService(llm, registry)
}

func buildIMAgent(llm adkmodel.LLM, cfg *config.Config, memSvc *memory.Service, sessionService adksession.Service, expertRegistry *expert.Registry, delegateRunner tool.DelegateRunner, evolutionCh chan<- hook.EvolutionSignal) (adkagent.Agent, []*adkplugin.Plugin, error) {
	a, extraPlugins, err := agent.New(llm, cfg, memSvc, sessionService, expertRegistry, delegateRunner, evolutionCh)
	if err != nil {
		return nil, nil, fmt.Errorf("create agent: %w", err)
	}
	adkPlugins := append([]*adkplugin.Plugin(nil), extraPlugins...)
	for _, pc := range cfg.Plugins {
		if !pc.Enabled {
			continue
		}
		p, err := plugin.Create(pc.Name)
		if err != nil {
			log.Printf("[im] plugin %s skipped: %v", pc.Name, err)
			continue
		}
		if p != nil {
			adkPlugins = append(adkPlugins, p)
		}
	}
	return a, adkPlugins, nil
}

func imConfigFromApp(cfg *config.Config) im.Config {
	out := im.Config{Platforms: make([]im.PlatformConfig, 0, len(cfg.IM.Platforms))}
	for _, platform := range cfg.IM.Platforms {
		out.Platforms = append(out.Platforms, im.PlatformConfig{
			Name:    platform.Name,
			Enabled: platform.Enabled,
			Options: mapAny(platform.Options),
		})
	}
	return out
}

func mapAny(in map[string]interface{}) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func platformNames(platforms map[string]im.Platform) []string {
	names := make([]string, 0, len(platforms))
	for name := range platforms {
		names = append(names, name)
	}
	return names
}
