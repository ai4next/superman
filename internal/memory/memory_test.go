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

	idx := s.GetL1Index()
	if len(idx) != 1 {
		t.Fatalf("GetL1Index returned %d entries, want 1", len(idx))
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