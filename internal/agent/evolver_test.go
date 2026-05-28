package agent

import (
	"testing"

	"github.com/ai4next/superman/internal/config"
	"google.golang.org/adk/tool"
)

func TestEvolverToolsFollowRuntimeConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Read.Enabled = true
	cfg.Tools.Write.Enabled = false
	cfg.Tools.Patch.Enabled = true
	cfg.Tools.Exec.Enabled = false
	cfg.Tools.Ask.Enabled = false

	names := toolNameSet(evolverTools(cfg))
	if !names["read"] || !names["patch"] {
		t.Fatalf("enabled tools missing: %#v", names)
	}
	for _, disabled := range []string{"write", "exec", "ask", "delegate"} {
		if names[disabled] {
			t.Fatalf("disabled/unavailable tool %q should not be registered: %#v", disabled, names)
		}
	}
}

func toolNameSet(tools []tool.Tool) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, t := range tools {
		names[t.Name()] = true
	}
	return names
}
