package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	supermanagent "github.com/ai4next/superman/internal/agent"
	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
)

var toolsetsJSON bool

var toolsetsCmd = &cobra.Command{
	Use:   "toolsets",
	Short: "List configured ADK Skill and MCP toolsets",
	RunE: func(cmd *cobra.Command, args []string) error {
		return writeToolsets(os.Stdout, global.Config(), toolsetsJSON)
	},
}

func init() {
	toolsetsCmd.Flags().BoolVar(&toolsetsJSON, "json", false, "print JSON")
}

func writeToolsets(w io.Writer, cfg *config.Config, asJSON bool) error {
	toolsets := supermanagent.DescribeConfiguredToolsets(cfg)
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(toolsets)
	}
	if len(toolsets) == 0 {
		_, err := fmt.Fprintln(w, "No ADK Skill or MCP toolsets configured")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tTOOLS\tSOURCE")
	for _, ts := range toolsets {
		tools := "-"
		if len(ts.Tools) > 0 {
			tools = strings.Join(ts.Tools, ",")
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ts.Name, ts.Kind, tools, ts.Source)
	}
	return tw.Flush()
}
