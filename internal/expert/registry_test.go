package expert

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryCreateAndGet(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	spec, err := r.Create(Spec{
		Name:           "code-reviewer",
		Summary:        "Reviews Go code for style and correctness",
		Description:    "A detailed code review expert",
		TriggerPattern: "review code|code review",
		ToolAllowlist:  []string{"read", "bash"},
		SystemPrompt:   "You are a code review expert.",
		Status:         StatusActive,
		Frequency:      0,
		Confidence:     0.8,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if spec.Name != "code-reviewer" {
		t.Errorf("got name %q, want %q", spec.Name, "code-reviewer")
	}
	if spec.Summary != "Reviews Go code for style and correctness" {
		t.Errorf("got summary %q, want %q", spec.Summary, "Reviews Go code for style and correctness")
	}
	if spec.Status != StatusActive {
		t.Errorf("got status %q, want %q", spec.Status, StatusActive)
	}
	if len(spec.ToolAllowlist) != 2 {
		t.Errorf("got %d tools, want 2", len(spec.ToolAllowlist))
	}
	if spec.CreatedAt.IsZero() {
		t.Errorf("CreatedAt should be set")
	}
	if spec.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt should be set")
	}
	if !spec.CreatedAt.Equal(spec.UpdatedAt) {
		t.Errorf("CreatedAt and UpdatedAt should be equal on create")
	}

	// Get should return a copy with same fields
	got, err := r.Get("code-reviewer")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Name != "code-reviewer" {
		t.Errorf("Get got name %q, want %q", got.Name, "code-reviewer")
	}
	if got.Confidence != 0.8 {
		t.Errorf("Get got confidence %f, want %f", got.Confidence, 0.8)
	}
}

func TestRegistryCreateDuplicate(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	_, err := r.Create(Spec{Name: "dup"})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err = r.Create(Spec{Name: "dup"})
	if err == nil {
		t.Fatal("expected error on duplicate create")
	}
}

func TestRegistryCreateEmptyName(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	_, err := r.Create(Spec{Name: ""})
	if err == nil {
		t.Fatal("expected error on empty name")
	}
}

func TestRegistryGetNotFound(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent expert")
	}
}

func TestRegistryList(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{Name: "expert-a"})
	r.Create(Spec{Name: "expert-b"})

	experts := r.List()
	if len(experts) != 2 {
		t.Fatalf("List returned %d experts, want 2", len(experts))
	}

	names := make(map[string]bool)
	for _, e := range experts {
		names[e.Name] = true
	}
	if !names["expert-a"] {
		t.Error("expert-a not in list")
	}
	if !names["expert-b"] {
		t.Error("expert-b not in list")
	}
}

func TestRegistryListEmpty(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	experts := r.List()
	if len(experts) != 0 {
		t.Fatalf("expected empty list, got %d", len(experts))
	}
}

func TestRegistryUpdate(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{
		Name:           "reviewer",
		Summary:        "Original summary",
		Description:    "Original description",
		TriggerPattern: "original pattern",
		Status:         StatusDraft,
	})

	err := r.Update("reviewer", Spec{
		Summary:        "Updated summary",
		Description:    "Updated description",
		TriggerPattern: "updated pattern",
		ToolAllowlist:  []string{"read"},
		SystemPrompt:   "Updated prompt",
		Status:         StatusActive,
		Frequency:      5,
		Confidence:     0.9,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got, err := r.Get("reviewer")
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}
	if got.Summary != "Updated summary" {
		t.Errorf("got summary %q, want %q", got.Summary, "Updated summary")
	}
	if got.Description != "Updated description" {
		t.Errorf("got description %q, want %q", got.Description, "Updated description")
	}
	if got.TriggerPattern != "updated pattern" {
		t.Errorf("got trigger %q, want %q", got.TriggerPattern, "updated pattern")
	}
	if got.Status != StatusActive {
		t.Errorf("got status %q, want %q", got.Status, StatusActive)
	}
	if got.Confidence != 0.9 {
		t.Errorf("got confidence %f, want %f", got.Confidence, 0.9)
	}
	if got.UpdatedAt.Equal(got.CreatedAt) {
		t.Errorf("UpdatedAt should have changed after update")
	}
}

func TestRegistryUpdateNotFound(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	err := r.Update("nonexistent", Spec{})
	if err == nil {
		t.Fatal("expected error updating nonexistent expert")
	}
}

func TestRegistryDelete(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{Name: "to-delete"})

	err := r.Delete("to-delete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = r.Get("to-delete")
	if err == nil {
		t.Fatal("expected error getting deleted expert")
	}
}

func TestRegistryDeleteNotFound(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	err := r.Delete("nonexistent")
	if err == nil {
		t.Fatal("expected error deleting nonexistent expert")
	}
}

func TestRegistryPersistence(t *testing.T) {
	baseDir := t.TempDir()

	// Create and save an expert
	r1 := NewRegistry(baseDir)
	created, err := r1.Create(Spec{
		Name:           "persistent-expert",
		Summary:        "Survives restarts",
		Description:    "This expert persists to disk",
		TriggerPattern: "persist|survive",
		ToolAllowlist:  []string{"read", "write"},
		SystemPrompt:   "You persist.",
		Status:         StatusActive,
		Confidence:     0.95,
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify the file was written to disk
	filePath := filepath.Join(baseDir, "persistent-expert", "expert.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expert.yaml was not written to disk: %s", filePath)
	}

	// Load into a new registry
	r2 := NewRegistry(baseDir)
	if err := r2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}

	loaded, err := r2.Get("persistent-expert")
	if err != nil {
		t.Fatalf("Get after LoadFromDisk failed: %v", err)
	}
	if loaded.Name != "persistent-expert" {
		t.Errorf("got name %q, want %q", loaded.Name, "persistent-expert")
	}
	if loaded.Summary != "Survives restarts" {
		t.Errorf("got summary %q, want %q", loaded.Summary, "Survives restarts")
	}
	if loaded.Status != StatusActive {
		t.Errorf("got status %q, want %q", loaded.Status, StatusActive)
	}
	if loaded.Confidence != 0.95 {
		t.Errorf("got confidence %f, want %f", loaded.Confidence, 0.95)
	}
	if len(loaded.ToolAllowlist) != 2 {
		t.Errorf("got %d tools, want 2", len(loaded.ToolAllowlist))
	}
	// Verify timestamps are preserved
	if !loaded.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v, want %v", loaded.CreatedAt, created.CreatedAt)
	}
}

func TestRegistryPersistenceEmptyDir(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	if err := r.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk on empty dir should not error: %v", err)
	}
	if len(r.List()) != 0 {
		t.Errorf("expected empty list, got %d", len(r.List()))
	}
}

func TestRegistrySearch(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{
		Name:           "go-linter",
		Summary:        "Checks Go code style",
		Description:    "Reviews Go code for lint issues",
		TriggerPattern: "lint|golang",
		Status:         StatusActive,
	})
	r.Create(Spec{
		Name:           "python-linter",
		Summary:        "Checks Python code style",
		Description:    "Reviews Python code for lint issues",
		TriggerPattern: "python|flake8",
		Status:         StatusActive,
	})

	results := r.Search("go")
	if len(results) == 0 {
		t.Fatal("expected search results for 'go'")
	}
	// Should match go-linter (name), "Go" in summary, "golang" in trigger
	found := false
	for _, s := range results {
		if s.Name == "go-linter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("search 'go' should return go-linter")
	}

	results = r.Search("Python")
	if len(results) == 0 {
		t.Fatal("expected search results for 'Python'")
	}
	found = false
	for _, s := range results {
		if s.Name == "python-linter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("search 'Python' should return python-linter")
	}
}

func TestRegistrySearchArchived(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{
		Name:           "old-expert",
		Summary:        "This is archived",
		TriggerPattern: "old",
		Status:         StatusArchived,
	})
	r.Create(Spec{
		Name:           "active-expert",
		Summary:        "This is active",
		TriggerPattern: "active",
		Status:         StatusActive,
	})

	results := r.Search("expert")
	if len(results) != 1 {
		t.Fatalf("expected 1 result (archived excluded), got %d", len(results))
	}
	if results[0].Name != "active-expert" {
		t.Errorf("got %q, want %q", results[0].Name, "active-expert")
	}
}

func TestRegistryRecordCall(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{Name: "call-logger"})

	record := CallRecord{
		Timestamp:  time.Now(),
		TaskDesc:   "review PR #42",
		Mode:       ModeConsult,
		Success:    true,
		DurationMs: 1500,
	}
	r.RecordCall("call-logger", record)

	// RecordCall does not expose logs publicly in this implementation,
	// but it should not panic. We verify by calling it without error.
	r.RecordCall("call-logger", CallRecord{
		Timestamp:  time.Now(),
		TaskDesc:   "another call",
		Mode:       ModeDelegate,
		Success:    false,
		DurationMs: 500,
	})
}

func TestRegistryRecordCallPersistence(t *testing.T) {
	defDir := filepath.Join(t.TempDir(), "defs")
	runtimeDir := filepath.Join(t.TempDir(), "runtime")

	r1 := NewRegistry(defDir)
	r1.SetRuntimeDir(runtimeDir)
	if _, err := r1.Create(Spec{Name: "call-logger"}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	record := CallRecord{
		Timestamp:  time.Now(),
		TaskDesc:   "review PR #42",
		Mode:       ModeDelegate,
		Success:    false,
		DurationMs: 1500,
	}
	if err := r1.RecordCall("call-logger", record); err != nil {
		t.Fatalf("RecordCall failed: %v", err)
	}

	path := filepath.Join(runtimeDir, "call-logger", "calls.jsonl")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("calls.jsonl was not written: %v", err)
	}

	r2 := NewRegistry(defDir)
	r2.SetRuntimeDir(runtimeDir)
	if err := r2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}
	records := r2.GetCallRecords("call-logger")
	if len(records) != 1 {
		t.Fatalf("loaded %d call records, want 1", len(records))
	}
	if records[0].TaskDesc != "review PR #42" {
		t.Errorf("task desc = %q", records[0].TaskDesc)
	}
	if records[0].Success {
		t.Error("expected persisted failed call")
	}
}

func TestRegistryUpdateAndDeleteRemovesFile(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	created := Spec{
		Name:           "temp-expert",
		Summary:        "Temporary expert",
		Description:    "Will be deleted",
		TriggerPattern: "temp",
		Status:         StatusDraft,
	}
	r.Create(created)

	// Verify file exists
	filePath := filepath.Join(baseDir, "temp-expert", "expert.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("expert.yaml should exist: %s", filePath)
	}

	// Delete
	r.Delete("temp-expert")

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("expert.yaml should be deleted, but still exists")
	}
}

func TestRegistryUpdateReplacesAllFields(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{
		Name:           "keeper",
		Summary:        "Original",
		Description:    "Original desc",
		TriggerPattern: "original",
		ToolAllowlist:  []string{"read"},
		SystemPrompt:   "Original prompt",
		Status:         StatusDraft,
		Frequency:      10,
		Confidence:     0.5,
	})

	// Update replaces ALL mutable fields
	r.Update("keeper", Spec{
		Summary:        "Updated",
		Description:    "Updated desc",
		TriggerPattern: "updated pattern",
		ToolAllowlist:  []string{"read", "write"},
		SystemPrompt:   "Updated prompt",
		Status:         StatusActive,
		Frequency:      20,
		Confidence:     0.9,
	})

	got, _ := r.Get("keeper")
	if got.Summary != "Updated" {
		t.Errorf("got summary %q, want %q", got.Summary, "Updated")
	}
	if got.Description != "Updated desc" {
		t.Errorf("got description %q, want %q", got.Description, "Updated desc")
	}
	if len(got.ToolAllowlist) != 2 {
		t.Errorf("got %d tools, want 2", len(got.ToolAllowlist))
	}
	if got.Frequency != 20 {
		t.Errorf("got frequency %d, want %d", got.Frequency, 20)
	}
	if got.Confidence != 0.9 {
		t.Errorf("got confidence %f, want %f", got.Confidence, 0.9)
	}
}

func TestRegistryGetReturnsCopy(t *testing.T) {
	baseDir := t.TempDir()
	r := NewRegistry(baseDir)

	r.Create(Spec{
		Name:   "protected",
		Status: StatusActive,
	})

	got1, _ := r.Get("protected")
	got2, _ := r.Get("protected")

	// Modify the returned slice (should not affect the registry)
	got1.ToolAllowlist = append(got1.ToolAllowlist, "hacked")
	if len(r.List()[0].ToolAllowlist) != 0 {
		t.Error("Get did not return a copy - modifying ToolAllowlist leaked back")
	}

	// Modify the returned spec (should not affect the second Get)
	got1.Summary = "hacked"
	got2again, _ := r.Get("protected")
	if got2again.Summary != "" {
		t.Errorf("Get did not return a copy - summary was %q, want empty", got2again.Summary)
	}
	_ = got2
}

func TestPromoteToActive(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "test", Status: StatusDraft})

	err := r.Promote("test", StatusActive)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	s, _ := r.Get("test")
	if s.Status != StatusActive {
		t.Errorf("expected active, got %s", s.Status)
	}
}

func TestPromoteToMature(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "test", Status: StatusActive})

	err := r.Promote("test", StatusMature)
	if err != nil {
		t.Fatalf("Promote failed: %v", err)
	}
	s, _ := r.Get("test")
	if s.Status != StatusMature {
		t.Errorf("expected mature, got %s", s.Status)
	}
}

func TestPromoteBackward(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "test", Status: StatusMature})

	err := r.Promote("test", StatusActive)
	if err == nil {
		t.Fatal("expected error promoting backward, got nil")
	}
}

func TestPromoteNotFound(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	err := r.Promote("nonexistent", StatusActive)
	if err == nil {
		t.Fatal("expected error for nonexistent expert")
	}
}

func TestArchiveStale(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "old", Summary: "old", Status: StatusMature})

	r.RecordCall("old", CallRecord{
		Timestamp: time.Now().Add(-100 * 24 * time.Hour),
		Success:   false,
	})
	r.RecordCall("old", CallRecord{
		Timestamp: time.Now().Add(-50 * 24 * time.Hour),
		Success:   false,
	})

	archived := r.ArchiveStale(30)
	if archived != 0 {
		t.Errorf("expected 0 archived (no success records), got %d", archived)
	}
}

func TestArchiveStaleWithSuccess(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "good", Summary: "good", Status: StatusActive})

	r.RecordCall("good", CallRecord{
		Timestamp: time.Now().Add(-100 * 24 * time.Hour),
		Success:   true,
	})

	archived := r.ArchiveStale(30)
	if archived != 1 {
		t.Fatalf("expected 1 archived, got %d", archived)
	}
	s, _ := r.Get("good")
	if s.Status != StatusArchived {
		t.Errorf("expected archived, got %s", s.Status)
	}
}

func TestCreateVersion(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "original", Status: StatusActive})

	v2, err := r.CreateVersion("test", Spec{
		Summary:      "updated version",
		Description:  "better version",
		SystemPrompt: "improved prompt",
	})
	if err != nil {
		t.Fatalf("CreateVersion failed: %v", err)
	}
	if v2.Version != 1 {
		t.Errorf("expected version 1, got %d", v2.Version)
	}
	if v2.PreviousID != "test" {
		t.Errorf("expected PreviousID 'test', got %s", v2.PreviousID)
	}
	if v2.Name != "test-v1" {
		t.Errorf("expected name test-v1, got %s", v2.Name)
	}
}

func TestGetVersionHistory(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "v1", Status: StatusActive})
	r.CreateVersion("test", Spec{Summary: "v2"})

	versions, err := r.GetVersionHistory("test")
	if err != nil {
		t.Fatalf("GetVersionHistory failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 0 {
		t.Errorf("expected version 0 first, got %d", versions[0].Version)
	}
	if versions[1].Version != 1 {
		t.Errorf("expected version 1 second, got %d", versions[1].Version)
	}
}

func TestGetVersionHistoryNotFound(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	_, err := r.GetVersionHistory("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent expert")
	}
}
func TestRegistryFTS5Search(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.EnableFTS5()

	r.Create(Spec{Name: "go-review", Summary: "Reviews Go code", Description: "examines Go files for bugs", Status: StatusActive})
	r.Create(Spec{Name: "py-review", Summary: "Reviews Python code", Description: "checks Python for PEP8 issues", Status: StatusActive})

	results := r.Search("Go programming bugs")
	if len(results) == 0 {
		t.Fatal("expected FTS5 results for 'Go programming bugs'")
	}
	if results[0].Name != "go-review" {
		t.Errorf("expected go-review as top result, got %s", results[0].Name)
	}
}
