package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ai4next/superman/internal/config"
	"github.com/spf13/cobra"

	"github.com/ai4next/superman/internal/global"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Show or initialize configuration",
	Long:  "Display current configuration or create config.yaml from the example template.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := "config.yaml"
		homeDir, _ := os.UserHomeDir()
		if homeDir != "" {
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				smPath := filepath.Join(homeDir, ".sm", "config.yaml")
				if _, err := os.Stat(smPath); err == nil {
					configPath = smPath
				}
			}
		}

		// If no config.yaml exists, offer to create one
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			data := config.ExampleYAML()
			if data == "" {
				return fmt.Errorf("embedded config.example.yaml is empty")
			}
			if err := os.WriteFile(configPath, []byte(data), 0644); err != nil {
				return fmt.Errorf("write config.yaml: %w", err)
			}
			fmt.Printf("Created %s from embedded config.example.yaml\n", configPath)
			fmt.Println("Edit it to set your API key and preferences.")
		} else {
			fmt.Printf("Config file exists at %s\n", configPath)
		}

		// Show current config summary
		if cfg := global.Config(); cfg != nil {
			fmt.Println()
			fmt.Println("Current configuration:")
			fmt.Printf("  Provider:  %s\n", cfg.Model.Provider)
			fmt.Printf("  Model:     %s\n", cfg.Model.Name)
			fmt.Printf("  Server:    %s\n", cfg.Server.Addr)
			fmt.Printf("  Tools:     exec=%v read=%v write=%v patch=%v ask=%v\n",
				cfg.Tools.Exec.Enabled,
				cfg.Tools.Read.Enabled,
				cfg.Tools.Write.Enabled,
				cfg.Tools.Patch.Enabled,
				cfg.Tools.Ask.Enabled,
			)
			fmt.Printf("  Max turns: %d\n", cfg.Session.MaxTurns)
		}
		return nil
	},
}
