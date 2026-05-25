package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func TestWriteToolsetsTable(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Skills:    config.SkillsConfig{Enabled: true},
		MCP: config.MCPConfig{Servers: []config.MCPServerConfig{{
			Name:                 "filesystem",
			Enabled:              true,
			Command:              "mcp-filesystem",
			Tools:                []string{"read_file"},
			RequiresConfirmation: true,
		}}},
	}
	var buf bytes.Buffer
	if err := writeToolsets(&buf, cfg, false); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"NAME", "skills:skills", "mcp:filesystem", "read_file", "yes"} {
		if !strings.Contains(out, want) {
			t.Fatalf("toolsets output missing %q:\n%s", want, out)
		}
	}
}

func TestWriteToolsetsJSON(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Skills:    config.SkillsConfig{Enabled: true},
	}
	var buf bytes.Buffer
	if err := writeToolsets(&buf, cfg, true); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"name": "skills:skills"`) || !strings.Contains(out, `"kind": "skill"`) {
		t.Fatalf("toolsets json = %s", out)
	}
}
