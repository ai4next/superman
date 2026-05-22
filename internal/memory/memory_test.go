package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestService_StoreAndGetL1(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	entry, err := s.Store(ctx, "The user prefers Python for data analysis", "preferences")
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if entry.ID == "" {
		t.Error("Store returned empty ID")
	}
	if entry.Layer != 2 {
		t.Errorf("layer = %d, want 2", entry.Layer)
	}
	if entry.Category != "preference" {
		t.Errorf("category = %q, want preference", entry.Category)
	}
	if entry.Scope != "user" {
		t.Errorf("scope = %q, want user", entry.Scope)
	}
	if entry.Source != "long_term_memory" {
		t.Errorf("source = %q, want long_term_memory", entry.Source)
	}
	if entry.Importance == 0 || entry.Confidence == 0 {
		t.Errorf("expected importance/confidence metadata, got %.2f/%.2f", entry.Importance, entry.Confidence)
	}

	idx := s.GetL1Index()
	if len(idx) != 1 {
		t.Fatalf("GetL1Index returned %d entries, want 1", len(idx))
	}
}

func TestService_DeduplicatesSimilarMemory(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	first, err := s.Store(ctx, "The user prefers Go for backend services", "preference")
	if err != nil {
		t.Fatalf("Store first: %v", err)
	}
	second, err := s.Store(ctx, "The user prefers Go for backend services", "preference")
	if err != nil {
		t.Fatalf("Store duplicate: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate created new ID %s, want %s", second.ID, first.ID)
	}
	if second.AccessCount != 1 {
		t.Errorf("duplicate should increase access count, got %d", second.AccessCount)
	}
	if got := len(s.GetL2Entries()); got != 1 {
		t.Errorf("L2 entries = %d, want 1", got)
	}
}

func TestService_ConflictSupersedesOldMemory(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	oldEntry, err := s.Store(ctx, "The user prefers Python for backend services", "preference")
	if err != nil {
		t.Fatalf("Store old: %v", err)
	}
	newEntry, err := s.Store(ctx, "The user now prefers Go for backend services instead", "preference")
	if err != nil {
		t.Fatalf("Store new: %v", err)
	}
	if len(newEntry.Supersedes) != 1 || newEntry.Supersedes[0] != oldEntry.ID {
		t.Fatalf("Supersedes = %#v, want [%s]", newEntry.Supersedes, oldEntry.ID)
	}
	if got := len(s.GetL2Entries()); got != 1 {
		t.Errorf("active L2 entries = %d, want 1", got)
	}

	results, err := s.Search(ctx, "backend services")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 || results[0].ID != newEntry.ID {
		t.Fatalf("top search result = %#v, want new entry first", results)
	}
}

func TestService_SearchUpdatesAccessMetadata(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	if _, err := s.Store(ctx, "User prefers Go for backend APIs", "preference"); err != nil {
		t.Fatalf("Store: %v", err)
	}
	results, err := s.Search(ctx, "backend")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].AccessCount != 1 {
		t.Errorf("access_count = %d, want 1", results[0].AccessCount)
	}
	if results[0].LastAccessedAt.IsZero() {
		t.Error("last_accessed_at should be set")
	}

	s2 := New(10, dir)
	if err := s2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}
	reloaded, err := s2.Search(ctx, "backend")
	if err != nil {
		t.Fatalf("Search after reload: %v", err)
	}
	if len(reloaded) != 1 || reloaded[0].AccessCount < 2 {
		t.Fatalf("access metadata was not persisted, got %#v", reloaded)
	}
}

func TestService_Search(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	s.Store(ctx, "Python is a programming language", "lang")
	s.Store(ctx, "Go is good for concurrent systems", "lang")

	results, err := s.Search(ctx, "python")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Search('python') returned %d results, want 1", len(results))
	}
}

func TestService_Archive(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	s.Store(ctx, "Some old fact", "test")
	// Archive everything older than 0 duration (immediate)
	count, err := s.Archive(ctx, 0)
	if err != nil {
		t.Fatalf("Archive: %v", err)
	}
	if count != 1 {
		t.Errorf("archived %d entries, want 1", count)
	}
	// All archived entries should be layer 3 now
	for _, e := range s.GetL2Entries() {
		t.Errorf("expected no L2 entries, got %s", e.ID)
	}
}

func TestService_MaxL1(t *testing.T) {
	dir := t.TempDir()
	s := New(3, dir)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.Store(ctx, "entry", "test")
	}

	idx := s.GetL1Index()
	if len(idx) > 3 {
		t.Errorf("L1 index size = %d, want <= 3", len(idx))
	}
}

func TestL0Store(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "coding.md"), []byte("Always write tests"), 0644); err != nil {
		t.Fatalf("write SOP: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "review.txt"), []byte("Review all PRs"), 0644); err != nil {
		t.Fatalf("write SOP: %v", err)
	}

	l0, err := NewL0Store(dir)
	if err != nil {
		t.Fatalf("NewL0Store: %v", err)
	}

	rules := l0.All()
	if len(rules) != 2 {
		t.Errorf("loaded %d rules, want 2", len(rules))
	}

	content, ok := l0.Get("coding")
	if !ok {
		t.Error("Get('coding') = false")
	}
	if content != "Always write tests" {
		t.Errorf("content = %q", content)
	}
}

func TestL0Store_MissingDir(t *testing.T) {
	l0, err := NewL0Store("/nonexistent/path")
	if err != nil {
		t.Fatal("expected no error for missing dir")
	}
	rules := l0.All()
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestSummarize(t *testing.T) {
	short := "hello"
	if s := summarize(short, 100); s != short {
		t.Errorf("summarize short = %q", s)
	}

	long := "this is a very long string that should definitely be truncated at the word boundary when we call summarize with a small max length"
	s := summarize(long, 30)
	if len(s) > 33 {
		t.Errorf("summarized length = %d, want <= 33", len(s))
	}
	if s[len(s)-3:] != "..." {
		t.Errorf("summarized = %q, want ... suffix", s)
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	if !containsIgnoreCase("Hello World", "world") {
		t.Error("containsIgnoreCase('Hello World', 'world') = false")
	}
	if !containsIgnoreCase("Hello World", "HELLO") {
		t.Error("containsIgnoreCase('Hello World', 'HELLO') = false")
	}
	if containsIgnoreCase("Hello World", "xyz") {
		t.Error("containsIgnoreCase('Hello World', 'xyz') = true")
	}
}

func TestStoreString(t *testing.T) {
	s := New(10, "")
	ctx := context.Background()

	id, err := s.StoreString(ctx, "test memory", "test")
	if err != nil {
		t.Fatalf("StoreString: %v", err)
	}
	if id == "" {
		t.Error("StoreString returned empty ID")
	}
}

func TestService_Persistence(t *testing.T) {
	dir := t.TempDir()
	s := New(10, dir)
	ctx := context.Background()

	s.Store(ctx, "user prefers Go for backend", "preference")
	s.Store(ctx, "project uses ADK framework", "tech")

	s2 := New(10, dir)
	if err := s2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk: %v", err)
	}

	results, _ := s2.Search(ctx, "Go")
	if len(results) != 1 {
		t.Errorf("Search('Go') after reload = %d results, want 1", len(results))
	}
	if results[0].Content != "user prefers Go for backend" {
		t.Errorf("Content = %q, want %q", results[0].Content, "user prefers Go for backend")
	}

	idx := s2.GetL1Index()
	if len(idx) != 2 {
		t.Errorf("L1 index after reload = %d entries, want 2", len(idx))
	}
}

func TestService_GetL1Content(t *testing.T) {
	dir := t.TempDir()
	s := New(5, dir)
	ctx := context.Background()

	s.Store(ctx, "user prefers Python for ML tasks", "preference")
	s.Store(ctx, "deploy target is Kubernetes", "ops")

	content := s.GetL1Content()
	if !strings.Contains(content, "Python") {
		t.Errorf("L1 content should contain 'Python', got:\n%s", content)
	}
	if !strings.Contains(content, "Kubernetes") {
		t.Errorf("L1 content should contain 'Kubernetes', got:\n%s", content)
	}
	if !strings.Contains(content, "Memory Index") {
		t.Errorf("L1 content should contain header, got:\n%s", content)
	}
}

func TestArchiveSessions(t *testing.T) {
	sessionDir := t.TempDir()
	memDir := t.TempDir()
	ctx := context.Background()

	sessionID := "test-session-123"
	sessionPath := filepath.Join(sessionDir, sessionID+".jsonl")
	sessionContent := `{"turn":1,"timestamp":"2026-05-19T10:00:00Z","user_message":"hello","agent_response":"hi","tool_calls":0}
{"turn":2,"timestamp":"2026-05-19T10:01:00Z","user_message":"what is Go?","agent_response":"Go is a language","tool_calls":1}
`
	if err := os.WriteFile(sessionPath, []byte(sessionContent), 0644); err != nil {
		t.Fatalf("write session file: %v", err)
	}

	count, err := ArchiveSessions(ctx, sessionDir, memDir, 0)
	if err != nil {
		t.Fatalf("ArchiveSessions: %v", err)
	}
	if count != 1 {
		t.Errorf("archived %d sessions, want 1", count)
	}

	l4Dir := filepath.Join(memDir, "l4")
	entries, err := os.ReadDir(l4Dir)
	if err != nil {
		t.Fatalf("read l4_archive: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("l4 has %d files, want 1", len(entries))
	}

	if _, err := os.Stat(sessionPath); !os.IsNotExist(err) {
		t.Errorf("original session file should be deleted, got: %v", err)
	}

	archivePath := filepath.Join(l4Dir, entries[0].Name())
	data, _ := os.ReadFile(archivePath)
	if !strings.Contains(string(data), "hello") {
		t.Errorf("archive should contain user message context")
	}
}

func TestService_EvolveWritesCandidatesOnly(t *testing.T) {
	dir := t.TempDir()
	candidateDir := filepath.Join(dir, "candidates")
	s := New(10, dir)
	ctx := context.Background()

	if _, err := s.Store(ctx, "User workflow is to read related tests before editing code", "workflow"); err != nil {
		t.Fatalf("Store workflow 1: %v", err)
	}
	if _, err := s.Store(ctx, "User workflow is to run targeted tests after editing code", "workflow"); err != nil {
		t.Fatalf("Store workflow 2: %v", err)
	}
	if _, err := s.Store(ctx, "User prefers Python for backend services", "preference"); err != nil {
		t.Fatalf("Store old preference: %v", err)
	}
	if _, err := s.Store(ctx, "User now prefers Go for backend services instead", "preference"); err != nil {
		t.Fatalf("Store new preference: %v", err)
	}

	before := len(s.GetL2Entries())
	candidates, err := s.Evolve(ctx, candidateDir)
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	after := len(s.GetL2Entries())
	if before != after {
		t.Fatalf("Evolve changed official L2 entries: before=%d after=%d", before, after)
	}
	if len(candidates) == 0 {
		t.Fatal("expected memory candidates")
	}
	if _, err := os.Stat(filepath.Join(candidateDir, "memory.jsonl")); err != nil {
		t.Fatalf("memory candidates not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(candidateDir, "sop", "workflow-memory-candidate.md")); err != nil {
		t.Fatalf("SOP candidate not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(candidateDir, "experts")); err != nil {
		t.Fatalf("experts candidate dir not created: %v", err)
	}
}
