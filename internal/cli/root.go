package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/global"
)

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ensureDirs creates all runtime directories required by the agent.
func ensureDirs() error {
	cfg := global.Config()
	dirs := []string{
		cfg.Workspace,
		global.SkillsDir(),
		global.HooksDir(),
		global.ExpertsDir(),
		global.MemoryDir(),
		global.L2Dir(),
		global.SessionsDir(),
	}
	for _, d := range dirs {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("create runtime dir %s: %w", d, err)
		}
	}
	return nil
}

var rootCmd = &cobra.Command{
	Use:   "superman",
	Short: "Superman - general-purpose autonomous AI agent",
	Long: `Superman is a general-purpose autonomous AI agent built with Google ADK.
	It supports multiple model providers, 6 built-in tools, layered memory,
	TUI interface, and autonomous reflection modes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		if _, err := global.LoadConfig(configPath); err != nil {
			return err
		}
		return ensureDirs()
	},
	RunE: RunServe,
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "path to config file (default: ./config.yaml)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(reflectCmd)
	rootCmd.AddCommand(toolsetsCmd)
	rootCmd.AddCommand(sessionsCmd)
	rootCmd.AddCommand(runtimeCmd)
	rootCmd.AddCommand(imCmd)
}
