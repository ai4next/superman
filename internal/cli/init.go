package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ai4next/superman/internal/config"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create config.yaml from the embedded example template",
	Long:  "Create config.yaml from the embedded config.example.yaml template.",
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")
		if configPath == "" {
			configPath = "config.yaml"
		}
		return writeConfigTemplate(configPath, config.ExampleYAML())
	},
}

func writeConfigTemplate(configPath string, template string) error {
	if template == "" {
		return fmt.Errorf("embedded config.example.yaml is empty")
	}
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("%s already exists", configPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check config file %s: %w", configPath, err)
	}
	if dir := filepath.Dir(configPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create config directory %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(configPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("write config file %s: %w", configPath, err)
	}
	fmt.Printf("Created %s from embedded config.example.yaml\n", configPath)
	return nil
}
