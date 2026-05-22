package expert

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAnalyzerBuildExpertCandidate(t *testing.T) {
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

	candidate := a.BuildCandidate(cluster)
	if candidate.Name == "" {
		t.Error("expected generated name")
	}
	if candidate.Confidence != 0.85 {
		t.Errorf("expected confidence 0.85, got %f", candidate.Confidence)
	}
	if candidate.Type != "create" {
		t.Errorf("expected create candidate, got %s", candidate.Type)
	}
	if len(reg.List()) != 0 {
		t.Fatal("BuildCandidate should not mutate the registry")
	}
	if candidate.Summary != "Go code review and test fixing" {
		t.Errorf("summary mismatch")
	}
}

func TestAnalyzerRunAnalysis(t *testing.T) {
	dir := t.TempDir()
	regDir := filepath.Join(dir, "registry")
	candidateDir := filepath.Join(dir, "candidates", "experts")
	reg := NewRegistry(regDir)
	a := NewAnalyzer(dir, reg)

	// Write 10 similar sessions to trigger cluster (need enough for confidence >= 0.5)
	turn := `{"turn":1,"timestamp":"2026-05-22T10:00:00Z","user_message":"fix the go test","agent_response":"ok","tool_calls":3}`
	for i := 0; i < 10; i++ {
		writeTestSession(t, dir, fmt.Sprintf("session-%03d", i), []string{turn})
	}

	candidates, err := a.RunAnalysis()
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least 1 expert candidate")
	}
	if len(reg.List()) != 0 {
		t.Fatal("RunAnalysis should not create registry experts")
	}
	if err := a.WriteCandidates(candidateDir, candidates); err != nil {
		t.Fatalf("WriteCandidates failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(candidateDir, "candidates.jsonl")); err != nil {
		t.Fatalf("expert candidates not written: %v", err)
	}
}

func TestAnalyzerSkipsCoveredExpert(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "registry"))
	if _, err := reg.Create(Spec{
		Name:           "fix-the-go-test",
		Summary:        "fix the go test",
		TriggerPattern: "fix the go test",
		Status:         StatusActive,
	}); err != nil {
		t.Fatalf("Create expert: %v", err)
	}
	a := NewAnalyzer(dir, reg)

	turn := `{"turn":1,"timestamp":"2026-05-22T10:00:00Z","user_message":"fix the go test","agent_response":"ok","tool_calls":3}`
	for i := 0; i < 10; i++ {
		writeTestSession(t, dir, fmt.Sprintf("session-%03d", i), []string{turn})
	}

	candidates, err := a.RunAnalysis()
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}
	if len(candidates) != 0 {
		t.Fatalf("expected covered pattern to be skipped, got %d candidates", len(candidates))
	}
}

func TestAnalyzerBuildsCallLogOptimizationCandidate(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "registry"))
	if _, err := reg.Create(Spec{
		Name:           "flaky-expert",
		Summary:        "Handles flaky tasks",
		TriggerPattern: "flaky",
		Status:         StatusActive,
		SystemPrompt:   "Try to help.",
	}); err != nil {
		t.Fatalf("Create expert: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := reg.RecordCall("flaky-expert", CallRecord{
			Timestamp:  time.Now(),
			TaskDesc:   fmt.Sprintf("failed task %d", i),
			Mode:       ModeDelegate,
			Success:    false,
			DurationMs: 100,
		}); err != nil {
			t.Fatalf("RecordCall: %v", err)
		}
	}

	a := NewAnalyzer(dir, reg)
	candidates, err := a.RunAnalysis()
	if err != nil {
		t.Fatalf("RunAnalysis failed: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 optimization candidate, got %d", len(candidates))
	}
	if candidates[0].Type != "optimize" {
		t.Fatalf("candidate type = %q, want optimize", candidates[0].Type)
	}
	if candidates[0].Source != "call_log_analysis" {
		t.Fatalf("candidate source = %q", candidates[0].Source)
	}
}

func TestAnalyzerAutoEvolveCreatesExpert(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "registry"))
	reg.SetRuntimeDir(filepath.Join(dir, "runtime"))
	a := NewAnalyzer(dir, reg)

	turn := `{"turn":1,"timestamp":"2026-05-22T10:00:00Z","user_message":"fix the go test","agent_response":"ok","tool_calls":3}`
	for i := 0; i < 10; i++ {
		writeTestSession(t, dir, fmt.Sprintf("session-%03d", i), []string{turn})
	}

	records, err := a.AutoEvolve()
	if err != nil {
		t.Fatalf("AutoEvolve: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected evolution records")
	}
	if records[0].Action != EvolutionCreate {
		t.Fatalf("action = %s, want create", records[0].Action)
	}
	if len(reg.List()) != 1 {
		t.Fatalf("registry experts = %d, want 1", len(reg.List()))
	}
	created := reg.List()[0]
	if created.Status != StatusActive {
		t.Fatalf("status = %s, want active", created.Status)
	}
	if created.CreatedBy != "auto_evolution" {
		t.Fatalf("created_by = %s", created.CreatedBy)
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "evolution.jsonl")); err != nil {
		t.Fatalf("evolution log not written: %v", err)
	}
}

func TestAnalyzerAutoEvolveOptimizesFlakyExpert(t *testing.T) {
	dir := t.TempDir()
	reg := NewRegistry(filepath.Join(dir, "registry"))
	if _, err := reg.Create(Spec{
		Name:           "flaky-expert",
		Summary:        "Handles flaky tasks",
		TriggerPattern: "flaky",
		Status:         StatusActive,
		SystemPrompt:   "Try to help.",
	}); err != nil {
		t.Fatalf("Create expert: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := reg.RecordCall("flaky-expert", CallRecord{
			Timestamp:  time.Now(),
			TaskDesc:   fmt.Sprintf("failed task %d", i),
			Mode:       ModeDelegate,
			Success:    false,
			DurationMs: 100,
		}); err != nil {
			t.Fatalf("RecordCall: %v", err)
		}
	}

	records, err := NewAnalyzer(dir, reg).AutoEvolve()
	if err != nil {
		t.Fatalf("AutoEvolve: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records = %d, want 1", len(records))
	}
	if records[0].Action != EvolutionOptimize {
		t.Fatalf("action = %s, want optimize", records[0].Action)
	}
	if _, err := reg.Get("flaky-expert-v1"); err != nil {
		t.Fatalf("expected optimized version: %v", err)
	}
}
