package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/config"
)

var cfg *config.Config

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

// ensureDirs creates all runtime directories required by the agent.
func ensureDirs(cfg *config.Config) error {
	dirs := []string{
		cfg.Dir,
		filepath.Join(cfg.Dir, "skills"),
		filepath.Join(cfg.Dir, "hooks"),
		cfg.Expert.Dir,
		cfg.Tools.CodeRun.Workspace,
		cfg.Session.HistoryPath,
		filepath.Join(cfg.Dir, "experts"),
		filepath.Join(cfg.Dir, "superman", "memory"),
		filepath.Join(cfg.Dir, "superman", "memory", "l0"),
		filepath.Join(cfg.Dir, "superman", "memory", "l1"),
		filepath.Join(cfg.Dir, "superman", "memory", "l2"),
		filepath.Join(cfg.Dir, "superman", "memory", "l3"),
		filepath.Join(cfg.Dir, "superman", "memory", "l4"),
		filepath.Join(cfg.Dir, "superman", "memory", "candidates", "sop"),
		filepath.Join(cfg.Dir, "superman", "memory", "candidates", "experts"),
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
	It supports multiple model providers, 9 built-in tools, layered memory,
	TUI interface, and autonomous reflection modes.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		var err error
		cfg, err = config.Load(configPath)
		if err != nil {
			return err
		}
		return ensureDirs(cfg)
	},
	RunE: RunServe,
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "path to config file (default: ./config.yaml)")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(reflectCmd)
}
