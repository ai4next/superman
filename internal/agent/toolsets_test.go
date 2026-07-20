package agent

import (
	"path/filepath"
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func TestBuildSkillToolsetsRespectsEnabledFlag(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Skills: config.SkillsConfig{
			Enabled: false,
		},
	}

	if got := buildSkillToolsets(t.Context(), cfg); len(got) != 0 {
		t.Fatalf("buildSkillToolsets returned %d toolsets, want 0", len(got))
	}
}

func TestBuildSkillToolsetsUsesWorkspaceDefault(t *testing.T) {
	workspace := t.TempDir()
	cfg := &config.Config{
		Workspace: workspace,
		Skills: config.SkillsConfig{
			Enabled: true,
		},
	}

	got := buildSkillToolsets(t.Context(), cfg)
	if len(got) != 1 {
		t.Fatalf("buildSkillToolsets returned %d toolsets, want 1", len(got))
	}
	if want := "skills:skills"; got[0].Name() != want {
		t.Fatalf("toolset name = %q, want %q", got[0].Name(), want)
	}
}

func TestBuildSkillToolsetsUsesConfiguredPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "agent-skills")
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Skills: config.SkillsConfig{
			Enabled: true,
			Paths:   []string{dir},
		},
	}

	got := buildSkillToolsets(t.Context(), cfg)
	if len(got) != 1 {
		t.Fatalf("buildSkillToolsets returned %d toolsets, want 1", len(got))
	}
	if want := "skills:agent-skills"; got[0].Name() != want {
		t.Fatalf("toolset name = %q, want %q", got[0].Name(), want)
	}
}

func TestBuildMCPToolsets(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Servers: []config.MCPServerConfig{
				{Name: "disabled", Enabled: false, Command: "ignored"},
				{Name: "empty-command", Enabled: true},
				{Name: "filesystem", Enabled: true, Command: "mcp-filesystem", Args: []string{"."}, Tools: []string{"read_file"}, RequiresConfirmation: true},
			},
		},
	}

	got := buildMCPToolsets(cfg)
	if len(got) != 1 {
		t.Fatalf("buildMCPToolsets returned %d toolsets, want 1", len(got))
	}
	if want := "mcp:filesystem"; got[0].Name() != want {
		t.Fatalf("toolset name = %q, want %q", got[0].Name(), want)
	}
}

func TestBuildMCPToolsetsUsesCommandAsFallbackName(t *testing.T) {
	cfg := &config.Config{
		MCP: config.MCPConfig{
			Servers: []config.MCPServerConfig{
				{Enabled: true, Command: "mcp-server"},
			},
		},
	}

	got := buildMCPToolsets(cfg)
	if len(got) != 1 {
		t.Fatalf("buildMCPToolsets returned %d toolsets, want 1", len(got))
	}
	if want := "mcp:mcp-server"; got[0].Name() != want {
		t.Fatalf("toolset name = %q, want %q", got[0].Name(), want)
	}
}

func TestDescribeConfiguredToolsets(t *testing.T) {
	workspace := t.TempDir()
	skillsDir := filepath.Join(workspace, "shared-skills")
	cfg := &config.Config{
		Workspace: workspace,
		Skills: config.SkillsConfig{
			Enabled: true,
			Paths:   []string{skillsDir},
		},
		MCP: config.MCPConfig{
			Servers: []config.MCPServerConfig{
				{Name: "filesystem", Enabled: true, Command: "mcp-filesystem", Args: []string{"."}, Tools: []string{"read_file"}, RequiresConfirmation: true},
				{Name: "disabled", Enabled: false, Command: "ignored"},
			},
		},
	}

	got := DescribeConfiguredToolsets(cfg)
	if len(got) != 2 {
		t.Fatalf("DescribeConfiguredToolsets returned %d descriptors, want 2: %#v", len(got), got)
	}
	if got[0].Name != "skills:shared-skills" || got[0].Kind != "skill" || got[0].Source != skillsDir {
		t.Fatalf("skill descriptor = %#v", got[0])
	}
	if got[1].Name != "mcp:filesystem" || got[1].Kind != "mcp" || !got[1].RequiresConfirmation || len(got[1].Tools) != 1 || got[1].Tools[0] != "read_file" {
		t.Fatalf("mcp descriptor = %#v", got[1])
	}
}

func TestDescribeConfiguredToolsetsRespectsDisabledSkills(t *testing.T) {
	cfg := &config.Config{
		Workspace: t.TempDir(),
		Skills:    config.SkillsConfig{Enabled: false},
	}

	if got := DescribeConfiguredToolsets(cfg); len(got) != 0 {
		t.Fatalf("DescribeConfiguredToolsets returned %#v, want none", got)
	}
}
