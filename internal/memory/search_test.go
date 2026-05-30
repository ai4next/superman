package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/global"
)

func TestSearchFindsSupermanAndExpertMemory(t *testing.T) {
	workspace := t.TempDir()
	setTestConfig(t, workspace)
	writeFile(t, filepath.Join(workspace, "memory", "superman", "l1.toml"), "[project]\npolicy = \"cache invalidation uses version keys\"\n")
	writeFile(t, filepath.Join(workspace, "state", "reviewer", "soul.md"), "review")
	writeFile(t, filepath.Join(workspace, "memory", "reviewer", "l2", "review.md"), "Always inspect cache invalidation paths in code review.")

	results, err := NewSearchService(global.Config()).Search(SearchOptions{Query: "cache invalidation", Limit: 10})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	owners := map[string]bool{}
	for _, result := range results {
		owners[result.Owner] = true
		if result.Path == "" || result.Layer == "" || result.Snippet == "" {
			t.Fatalf("incomplete result: %+v", result)
		}
	}
	if !owners[OwnerSuperman] || !owners["reviewer"] {
		t.Fatalf("owners = %#v, want superman and reviewer", owners)
	}
}

func TestSearchFiltersOwnerAndLayer(t *testing.T) {
	workspace := t.TempDir()
	setTestConfig(t, workspace)
	writeFile(t, filepath.Join(workspace, "memory", "superman", "l1.toml"), "[project]\npolicy = \"token budget\"\n")
	writeFile(t, filepath.Join(workspace, "state", "reviewer", "soul.md"), "review")
	writeFile(t, filepath.Join(workspace, "memory", "reviewer", "l2", "review.md"), "token budget review")

	results, err := NewSearchService(global.Config()).Search(SearchOptions{Query: "token", Owners: []string{"reviewer"}, Layers: []string{LayerL2}})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("expected filtered results")
	}
	for _, result := range results {
		if result.Owner != "reviewer" || result.Layer != LayerL2 {
			t.Fatalf("unexpected filtered result: %+v", result)
		}
	}
}

func TestSearchSupportsVectorMode(t *testing.T) {
	workspace := t.TempDir()
	global.SetConfig(&config.Config{
		Workspace: workspace,
		Memory: config.MemoryConfig{
			Search: config.MemorySearchConfig{Enabled: true, VectorEnabled: true, MaxResults: 8},
		},
	})
	t.Cleanup(func() { global.SetConfig(nil) })
	writeFile(t, filepath.Join(workspace, "memory", "superman", "l1.toml"), "[project]\npolicy = \"semantic routing for expert memory\"\n")

	results, err := NewSearchService(global.Config()).Search(SearchOptions{Query: "semantic routing"})
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) == 0 || results[0].MatchType != "vector" {
		t.Fatalf("results = %+v, want vector result", results)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
}

func setTestConfig(t *testing.T, workspace string) {
	t.Helper()
	global.SetConfig(&config.Config{
		Workspace: workspace,
		Memory: config.MemoryConfig{
			Search:  config.MemorySearchConfig{Enabled: true, FTSEnabled: true, ScanEnabled: true, MaxResults: 8},
			Mailbox: config.MemoryMailboxConfig{Enabled: true},
		},
	})
	t.Cleanup(func() { global.SetConfig(nil) })
}
