package expert

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeTestSession(t *testing.T, dir, name string, turns []string) {
	t.Helper()
	path := filepath.Join(dir, name+".jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	defer f.Close()
	for _, turn := range turns {
		f.WriteString(turn + "\n")
	}
}

func TestAnalyzerExtractToolChains(t *testing.T) {
	dir := t.TempDir()
	a := NewAnalyzer(dir, nil)
	turns := []string{
		`{"turn":1,"timestamp":"2026-05-22T10:00:00Z","user_message":"fix the broken test","agent_response":"looking...","tool_calls":3}`,
		`{"turn":2,"timestamp":"2026-05-22T10:01:00Z","user_message":"","agent_response":"done","tool_calls":2}`,
	}
	writeTestSession(t, dir, "session-001", turns)
	chains, err := a.ExtractToolChains()
	if err != nil {
		t.Fatalf("ExtractToolChains failed: %v", err)
	}
	if len(chains) != 1 {
		t.Fatalf("expected 1 chain, got %d", len(chains))
	}
	if chains[0].ToolCount != 5 {
		t.Errorf("expected 5 tool calls, got %d", chains[0].ToolCount)
	}
	if chains[0].TaskSummary != "fix the broken test" {
		t.Errorf("expected 'fix the broken test', got %q", chains[0].TaskSummary)
	}
}

func TestAnalyzerClusterByPattern(t *testing.T) {
	a := NewAnalyzer("", nil)
	chains := []ToolChain{
		{TaskSummary: "fix go test", ToolCount: 3, FileTypes: []string{".go"}},
		{TaskSummary: "review go code", ToolCount: 2, FileTypes: []string{".go"}},
		{TaskSummary: "write shell script", ToolCount: 1, FileTypes: []string{".sh"}},
	}
	clusters := a.Cluster(chains)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	if clusters[0].SessionCount != 2 {
		t.Errorf("expected 2 sessions in .go cluster, got %d", clusters[0].SessionCount)
	}
}

func TestAnalyzerGenerateExpertDraft(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	a := NewAnalyzer(dir, reg)

	cluster := ToolChainCluster{
		TaskSummary:  "Go code review and test fixing",
		ToolCount:    15,
		SessionCount: 5,
		FileTypes:    []string{".go"},
		Confidence:   0.85,
	}

	draft, err := a.GenerateDraft(cluster)
	if err != nil {
		t.Fatalf("GenerateDraft failed: %v", err)
	}
	if draft.Name == "" {
		t.Error("expected generated name")
	}
	if draft.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", draft.Confidence)
	}
	if draft.Status != StatusDraft {
		t.Errorf("expected draft status, got %s", draft.Status)
	}

	got, err := reg.Get(draft.Name)
	if err != nil {
		t.Fatalf("saved expert not found: %v", err)
	}
	if got.Summary != "Go code review and test fixing" {
		t.Errorf("summary mismatch")
	}
}

func TestAnalyzerRunAnalysis(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(dir)
	a := NewAnalyzer(dir, reg)

	// Write 10 similar sessions to trigger cluster (need enough for confidence >= 0.5)
	turn := `{"turn":1,"timestamp":"2026-05-22T10:00:00Z","user_message":"fix the go test","agent_response":"ok","tool_calls":3}`
	for i := 0; i < 10; i++ {
		writeTestSession(t, dir, fmt.Sprintf("session-%03d", i), []string{turn})
	}

	created, err := a.RunAnalysis()
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}
	if len(created) == 0 {
		t.Fatal("expected at least 1 expert draft")
	}
	if created[0].Status != StatusDraft {
		t.Errorf("expected StatusDraft, got %s", created[0].Status)
	}
}