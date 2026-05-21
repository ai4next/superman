package cli

import (
	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/config"
)

var cfg *config.Config

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
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
		return err
	},
}

func init() {
	rootCmd.PersistentFlags().String("config", "", "path to config file (default: ./config.yaml)")
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(reflectCmd)
}