package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Show or initialize configuration",
	Long:  "Display current configuration or create config.yaml from the example template.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := "config.yaml"

		// If no config.yaml exists, offer to create one
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			examplePath := "config.example.yaml"
			if _, err := os.Stat(examplePath); err == nil {
				data, err := os.ReadFile(examplePath)
				if err != nil {
					return fmt.Errorf("read example config: %w", err)
				}
				if err := os.WriteFile(configPath, data, 0644); err != nil {
					return fmt.Errorf("write config.yaml: %w", err)
				}
				fmt.Printf("Created %s from %s\n", configPath, examplePath)
				fmt.Println("Edit it to set your API key and preferences.")
			} else {
				return fmt.Errorf("no config.example.yaml found at %s", filepath.Dir(examplePath))
			}
		} else {
			fmt.Printf("Config file exists at %s\n", configPath)
		}

		// Show current config summary
		if cfg != nil {
			fmt.Println()
			fmt.Println("Current configuration:")
			fmt.Printf("  Provider:  %s\n", cfg.Model.Provider)
			fmt.Printf("  Model:     %s\n", cfg.Model.Name)
			fmt.Printf("  Server:    %s\n", cfg.Server.Addr)
			fmt.Printf("  Tools:     code_run=%v file_read=%v file_write=%v file_patch=%v web_scan=%v ask_user=%v\n",
				cfg.Tools.CodeRun.Enabled,
				cfg.Tools.FileRead.Enabled,
				cfg.Tools.FileWrite.Enabled,
				cfg.Tools.FilePatch.Enabled,
				cfg.Tools.WebScan.Enabled,
				cfg.Tools.AskUser.Enabled,
			)
			fmt.Printf("  Max turns: %d\n", cfg.Session.MaxTurns)
		}
		return nil
	},
}