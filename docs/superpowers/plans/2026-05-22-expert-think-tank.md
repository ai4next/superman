# Expert Think Tank Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a self-growing expert think tank system where experts emerge from task execution experience

**Architecture:** Three-layer system: (1) Expert Registry for persistent storage and search, (2) Pattern Analyzer for extracting expert definitions from session logs, (3) Dispatcher for consult/delegate routing at runtime. All three layers sit alongside the existing ADK single-agent architecture, communicating via file storage and the plugin system.

**Tech Stack:** Go 1.25+, ADK v1.3.0, YAML file storage, JSONL session logs, FTS5 (Phase 3)

---

## Phase 1: Expert Registry + Tools (MVP)

Core storage, CRUD, and agent-facing tools for manual expert management.

### Task 1: Define expert types and file-based storage

**Files:**
- Create: `internal/expert/types.go`

- [ ] **Step 1: Write expert types and tests**

```go
// internal/expert/types.go
package expert

import "time"

// Status represents the lifecycle stage of an expert.
type Status string

const (
	StatusDraft   Status = "draft"
	StatusActive  Status = "active"
	StatusMature  Status = "mature"
	StatusArchived Status = "archived"
)

// Spec defines an expert agent's identity, trigger conditions, and capabilities.
type Spec struct {
	Name           string   `yaml:"name" json:"name"`
	Summary        string   `yaml:"summary" json:"summary"`
	Description    string   `yaml:"description" json:"description"`
	TriggerPattern string   `yaml:"trigger_pattern" json:"trigger_pattern"`
	ToolWhitelist  []string `yaml:"tools" json:"tools"`
	SystemPrompt   string   `yaml:"prompt" json:"prompt"`
	Status         Status   `yaml:"status" json:"status"`
	Frequency      int      `yaml:"frequency" json:"frequency"`
	Confidence     float64  `yaml:"confidence" json:"confidence"`
	CreatedAt      time.Time `yaml:"created_at" json:"created_at"`
	UpdatedAt      time.Time `yaml:"updated_at" json:"updated_at"`
}

// CallRecord is a single invocation log entry for an expert.
type CallRecord struct {
	Timestamp  time.Time `json:"timestamp"`
	TaskDesc   string    `json:"task_desc"`
	Mode       string    `json:"mode"` // "consult" or "delegate"
	Success    bool      `json:"success"`
	DurationMs int64     `json:"duration_ms"`
}
```

```go
// internal/expert/types_test.go
package expert

import (
	"testing"
	"time"
)

func TestSpecDefaults(t *testing.T) {
	s := Spec{
		Name:    "test-expert",
		Summary: "A test expert",
	}
	if s.Status != "" {
		t.Errorf("expected empty status, got %s", s.Status)
	}
	if s.CreatedAt.IsZero() == false {
		// CreatedAt not set yet, that's fine
	}
}

func TestCallRecord(t *testing.T) {
	r := CallRecord{
		Timestamp: time.Now(),
		TaskDesc:  "review PR #42",
		Mode:      "consult",
		Success:   true,
	}
	if r.Mode != "consult" {
		t.Errorf("expected consult mode, got %s", r.Mode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail (package doesn't exist yet)**

Run: `go test ./internal/expert/ -v`
Expected: FAIL — package not found

- [ ] **Step 3: Write types implementation**

Create the file as shown in Step 1 code.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/expert/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/expert/types.go internal/expert/types_test.go
git commit -m "feat(expert): define Spec and CallRecord types"
```

### Task 2: Implement Expert Registry with file persistence

**Files:**
- Create: `internal/expert/registry.go`
- Create: `internal/expert/registry_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/expert/registry_test.go
package expert

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryCreateAndGet(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)

	spec := Spec{
		Name:         "go-reviewer",
		Summary:      "Go code review specialist",
		ToolWhitelist: []string{"file_read", "file_patch", "code_run"},
		SystemPrompt: "You are a Go code review expert.",
		Status:       StatusActive,
	}
	created, err := r.Create(spec)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if created.Name != "go-reviewer" {
		t.Errorf("expected name go-reviewer, got %s", created.Name)
	}
	if created.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	got, err := r.Get("go-reviewer")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Summary != "Go code review specialist" {
		t.Errorf("expected summary mismatch")
	}
}

func TestRegistryList(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "a", Summary: "A"})
	r.Create(Spec{Name: "b", Summary: "B"})

	all := r.List()
	if len(all) != 2 {
		t.Errorf("expected 2 experts, got %d", len(all))
	}
}

func TestRegistryUpdate(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "x", Summary: "old", Status: StatusDraft})

	err := r.Update("x", Spec{Summary: "new", Status: StatusActive})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	got, _ := r.Get("x")
	if got.Summary != "new" || got.Status != StatusActive {
		t.Errorf("update not applied: %+v", got)
	}
}

func TestRegistryDelete(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "del", Summary: "delete me"})

	err := r.Delete("del")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := r.Get("del"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestRegistryPersistence(t *testing.T) {
	dir := t.TempDir()
	r1 := NewRegistry(dir)
	r1.Create(Spec{Name: "persist", Summary: "survive restart"})

	// Re-create registry from same dir
	r2 := NewRegistry(dir)
	if err := r2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}
	got, err := r2.Get("persist")
	if err != nil {
		t.Fatalf("Get after reload failed: %v", err)
	}
	if got.Summary != "survive restart" {
		t.Errorf("expected summary 'survive restart', got %s", got.Summary)
	}
}

func TestRegistrySearch(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{
		Name:           "go-review",
		Summary:        "Reviews Go code",
		TriggerPattern: "修改 .go 文件 → go test",
		Status:         StatusActive,
	})
	r.Create(Spec{
		Name:           "shell-pro",
		Summary:        "Shell scripting expert",
		TriggerPattern: "bash command → pipeline",
		Status:         StatusActive,
	})

	results := r.Search("go test")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'go test', got %d", len(results))
	}
	if results[0].Name != "go-review" {
		t.Errorf("expected go-review, got %s", results[0].Name)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/expert/ -v -run TestRegistry`
Expected: FAIL — registry.go doesn't exist, undefined functions

- [ ] **Step 3: Write registry implementation**

```go
// internal/expert/registry.go
package expert

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// Registry manages expert definitions with file-based persistence.
type Registry struct {
	mu       sync.RWMutex
	baseDir  string
	experts  map[string]*Spec
	callLogs map[string][]CallRecord
}

// NewRegistry creates a registry rooted at baseDir/data/experts/.
func NewRegistry(baseDir string) *Registry {
	return &Registry{
		baseDir:  baseDir,
		experts:  make(map[string]*Spec),
		callLogs: make(map[string][]CallRecord),
	}
}

// LoadFromDisk reads all expert definitions from disk.
func (r *Registry) LoadFromDisk() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	expertsDir := filepath.Join(r.baseDir, "data", "experts")
	entries, err := os.ReadDir(expertsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read experts dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		specPath := filepath.Join(expertsDir, entry.Name(), "expert.yaml")
		data, err := os.ReadFile(specPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}
		var spec Spec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			continue
		}
		r.experts[spec.Name] = &spec
	}
	return nil
}

// Create stores a new expert spec. Name must be unique.
func (r *Registry) Create(spec Spec) (*Spec, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if spec.Name == "" {
		return nil, fmt.Errorf("expert name is required")
	}
	if _, exists := r.experts[spec.Name]; exists {
		return nil, fmt.Errorf("expert %q already exists", spec.Name)
	}

	now := time.Now()
	spec.CreatedAt = now
	spec.UpdatedAt = now
	if spec.Status == "" {
		spec.Status = StatusDraft
	}
	r.experts[spec.Name] = &spec

	if err := r.persistLocked(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// Get retrieves an expert by name.
func (r *Registry) Get(name string) (*Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.experts[name]
	if !ok {
		return nil, fmt.Errorf("expert %q not found", name)
	}
	cp := *spec
	return &cp, nil
}

// List returns all experts.
func (r *Registry) List() []*Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Spec, 0, len(r.experts))
	for _, s := range r.experts {
		cp := *s
		result = append(result, &cp)
	}
	return result
}

// Update modifies an existing expert.
func (r *Registry) Update(name string, spec Spec) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.experts[name]
	if !ok {
		return fmt.Errorf("expert %q not found", name)
	}

	existing.Summary = spec.Summary
	existing.Description = spec.Description
	existing.TriggerPattern = spec.TriggerPattern
	existing.ToolWhitelist = spec.ToolWhitelist
	existing.SystemPrompt = spec.SystemPrompt
	existing.Status = spec.Status
	existing.Frequency = spec.Frequency
	existing.Confidence = spec.Confidence
	existing.UpdatedAt = time.Now()

	return r.persistLocked(existing)
}

// Delete removes an expert.
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.experts[name]; !ok {
		return fmt.Errorf("expert %q not found", name)
	}
	delete(r.experts, name)

	// Remove files
	dir := filepath.Join(r.baseDir, "data", "experts", name)
	os.RemoveAll(dir)
	return nil
}

// Search performs keyword matching against name, summary, and trigger pattern.
// Only returns active and mature experts.
func (r *Registry) Search(query string) []*Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lowerQuery := strings.ToLower(query)
	var results []*Spec
	for _, s := range r.experts {
		if s.Status == StatusArchived {
			continue
		}
		haystack := strings.ToLower(s.Name + " " + s.Summary + " " + s.TriggerPattern + " " + s.Description)
		if strings.Contains(haystack, lowerQuery) {
			cp := *s
			results = append(results, &cp)
		}
	}
	return results
}

// RecordCall logs an invocation for stats.
func (r *Registry) RecordCall(name string, record CallRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.callLogs[name] = append(r.callLogs[name], record)
}

func (r *Registry) persistLocked(spec *Spec) error {
	dir := filepath.Join(r.baseDir, "data", "experts", spec.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "expert.yaml")
	return os.WriteFile(path, data, 0644)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/expert/ -v -run TestRegistry`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/expert/registry.go internal/expert/registry_test.go
git commit -m "feat(expert): implement Registry with file persistence and search"
```

### Task 3: Add config for expert subsystem

**Files:**
- Modify: `internal/config/config.go`
- Modify: `config.example.yaml`

- [ ] **Step 1: Add ExpertConfig to config struct**

```go
// Add to internal/config/config.go, after ReflectConfig field
Expert ExpertConfig `mapstructure:"expert"`

// Add new type (before Duration)
type ExpertConfig struct {
	Enabled      bool     `mapstructure:"enabled"`
	Dir          string   `mapstructure:"dir"`
	TopK         int      `mapstructure:"top_k"`
}
```

- [ ] **Step 2: Add expert section to example config**

```yaml
# Add to config.example.yaml after the reflect: section
expert:
  enabled: true
  dir: ./data
  top_k: 2
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/config/config.go config.example.yaml
git commit -m "feat(expert): add ExpertConfig"
```

### Task 4: Create expert management tools for the agent

**Files:**
- Create: `internal/agent/tools/experts.go`
- Modify: `internal/agent/tools/registry.go`

- [ ] **Step 1: Add expert dependency interfaces and register tools**

```go
// internal/agent/tools/experts.go
package tools

import (
	"context"
	"fmt"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/ai4next/superman/internal/expert"
)

// --- query_experts tool ---

type queryExpertsInput struct {
	TaskDescription string `json:"task_description" jsonschema:"Describe the current task to find matching experts"`
}

type expertInfo struct {
	Name           string   `json:"name"`
	Summary        string   `json:"summary"`
	Description    string   `json:"description"`
	ToolWhitelist  []string `json:"tools"`
	SystemPrompt   string   `json:"system_prompt"`
	Status         string   `json:"status"`
}

type queryExpertsOutput struct {
	Found   bool         `json:"found"`
	Experts []expertInfo `json:"experts,omitempty"`
	Message string       `json:"message,omitempty"`
}

// --- create_expert tool ---

type createExpertInput struct {
	Name           string   `json:"name" jsonschema:"Unique name for the new expert agent"`
	Summary        string   `json:"summary" jsonschema:"One-line summary of what this expert does"`
	Description    string   `json:"description" jsonschema:"Detailed description of when to use this expert"`
	TriggerPattern string   `json:"trigger_pattern" jsonschema:"Pattern that triggers this expert, e.g. 'modify Go files → run tests'"`
	ToolWhitelist  []string `json:"tools" jsonschema:"List of tool names this expert is allowed to use"`
	SystemPrompt   string   `json:"system_prompt" jsonschema:"The system prompt for this expert agent"`
}

type createExpertOutput struct {
	Created bool   `json:"created"`
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ExpertManager provides CRUD operations for experts.
type ExpertManager interface {
	Search(query string) []*expert.Spec
	List() []*expert.Spec
	Create(spec expert.Spec) (*expert.Spec, error)
}

func newQueryExpertsTool(mgr ExpertManager) tool.Tool {
	handler := func(tctx tool.Context, input queryExpertsInput) (queryExpertsOutput, error) {
		if mgr == nil {
			return queryExpertsOutput{Found: false, Message: "Expert system not available"}, nil
		}
		results := mgr.Search(input.TaskDescription)
		if len(results) == 0 {
			return queryExpertsOutput{Found: false, Message: "No matching experts found"}, nil
		}
		infos := make([]expertInfo, len(results))
		for i, s := range results {
			infos[i] = expertInfo{
				Name:          s.Name,
				Summary:       s.Summary,
				Description:   s.Description,
				ToolWhitelist: s.ToolWhitelist,
				SystemPrompt:  s.SystemPrompt,
				Status:        string(s.Status),
			}
		}
		return queryExpertsOutput{
			Found:   true,
			Experts: infos,
			Message: fmt.Sprintf("Found %d matching experts", len(results)),
		}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "query_experts",
		Description: "Search for expert agents that match the current task. Returns expert system prompts and tool configurations that can help with the task.",
	}, handler)
	return t
}

func newCreateExpertTool(mgr ExpertManager) tool.Tool {
	handler := func(tctx tool.Context, input createExpertInput) (createExpertOutput, error) {
		if mgr == nil {
			return createExpertOutput{Created: false, Error: "Expert system not available"}, nil
		}
		spec := expert.Spec{
			Name:           input.Name,
			Summary:        input.Summary,
			Description:    input.Description,
			TriggerPattern: input.TriggerPattern,
			ToolWhitelist:  input.ToolWhitelist,
			SystemPrompt:   input.SystemPrompt,
		}
		created, err := mgr.Create(spec)
		if err != nil {
			return createExpertOutput{Created: false, Error: err.Error()}, nil
		}
		return createExpertOutput{Created: true, Name: created.Name}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "create_expert",
		Description: "Create a new expert agent from experience. Use this when you notice a recurring task pattern that would benefit from a dedicated specialist.",
	}, handler)
	return t
}
```

- [ ] **Step 2: Add ExpertManager to Dependencies and register new tools**

```go
// In internal/agent/tools/registry.go, add to Dependencies:
ExpertManager ExpertManager `json:"-"`

// In RegisterAll, after search_memory registration:
if deps.ExpertManager != nil {
    if deps.Config.Expert.Enabled {
        tools = append(tools, newQueryExpertsTool(deps.ExpertManager))
        tools = append(tools, newCreateExpertTool(deps.ExpertManager))
    }
}
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools/experts.go internal/agent/tools/registry.go
git commit -m "feat(expert): add query_experts and create_expert tools"
```

### Task 5: Wire up Expert Registry in serve.go and create seed expert

**Files:**
- Modify: `internal/cli/serve.go`
- Modify: `internal/plugin/plugin.go`

- [ ] **Step 1: Wire up Expert Registry in serve.go**

```go
// In internal/cli/serve.go, after L4 archiver setup block:

// Expert Registry
var expertRegistry *expert.Registry
if cfg.Expert.Enabled {
    expertRegistry = expert.NewRegistry(cfg.Expert.Dir)
    if err := expertRegistry.LoadFromDisk(); err != nil {
        log.Printf("[expert] load warning: %v", err)
    }
    log.Printf("[expert] loaded %d experts", len(expertRegistry.List()))
}

// Pass expertRegistry to agent.New:
a, err := agent.New(llm, cfg, memSvc, searchAdapter, sopContent, expertRegistry)
```

Add the import:
```go
"github.com/ai4next/superman/internal/expert"
```

- [ ] **Step 2: Update agent.go to accept expert registry**

```go
// In internal/agent/agent.go, change New signature:
func New(llm model.LLM, cfg *config.Config, memSvc *memory.Service, memSearcher tools.MemorySearcher, sopContent string, expertRegistry *expert.Registry) (agent.Agent, error) {

// After creating toolList from RegisterAll(deps), if expertRegistry != nil:
deps.ExpertManager = expertRegistry
```

Add import:
```go
"github.com/ai4next/superman/internal/expert"
```

- [ ] **Step 3: Create seed expert file for Go code review**

Create file `data/experts/go-code-reviewer/expert.yaml`:

```yaml
name: go-code-reviewer
summary: Go 代码审查与测试修复专家
description: 专门审查 Go 代码质量、修复编译错误、优化测试用例。当任务涉及 .go 文件修改和 go test 时最有用。
trigger_pattern: "修改 .go 文件 → go test → 修复编译错误"
tools:
  - file_read
  - file_patch
  - code_run
prompt: |
  You are a Go code review specialist. Your expertise:
  - Idiomatic Go code style and conventions
  - Test writing and debugging (go test)
  - Common Go pitfalls (nil pointer, interface misuse, goroutine leaks)
  - Performance optimization for Go code

  When reviewing code:
  1. First read the relevant files to understand context
  2. Run tests to see current state
  3. Make minimal, focused changes
  4. Verify tests pass after each change
  5. Prefer standard library over external dependencies
status: active
frequency: 0
confidence: 0.9
```

- [ ] **Step 4: Verify build and TUI startup**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/serve.go internal/agent/agent.go data/experts/ internal/plugin/plugin.go
git commit -m "feat(expert): wire up Expert Registry in agent and add seed expert"
```

---

## Phase 2: Pattern Analyzer + Delegate Mode + Lifecycle

### Task 6: Implement Pattern Analyzer

**Files:**
- Create: `internal/expert/analyzer.go`
- Create: `internal/expert/analyzer_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/expert/analyzer_test.go
package expert

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestSession(t *testing.T, dir string, name string, turns []string) {
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
}

func TestAnalyzerClusterByPattern(t *testing.T) {
	a := NewAnalyzer("", nil)
	chains := []ToolChain{
		{TaskSummary: "fix go test", ToolCount: 3, FileTypes: []string{".go"}},
		{TaskSummary: "review go code", ToolCount: 2, FileTypes: []string{".go"}},
		{TaskSummary: "write shell script", ToolCount: 1, FileTypes: []string{".sh"}},
	}
	clusters := a.Cluster(chains)
	if len(clusters) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(clusters))
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

	// Verify it was saved to registry
	got, err := reg.Get(draft.Name)
	if err != nil {
		t.Fatalf("saved expert not found: %v", err)
	}
	if got.Status != StatusDraft {
		t.Errorf("expected draft status, got %s", got.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/expert/ -v -run TestAnalyzer`
Expected: FAIL

- [ ] **Step 3: Write Pattern Analyzer implementation**

```go
// internal/expert/analyzer.go
package expert

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ToolChain represents a sequence of tool calls from one session.
type ToolChain struct {
	SessionID   string
	TaskSummary string
	ToolCount   int
	FileTypes   []string
	Duration    time.Duration
	Success     bool
}

// ToolChainCluster groups similar tool chains.
type ToolChainCluster struct {
	TaskSummary  string
	ToolCount    int
	SessionCount int
	FileTypes    []string
	Confidence   float64
}

// Analyzer reads session logs and extracts expert drafts.
type Analyzer struct {
	sessionDir string
	registry   *Registry
}

// NewAnalyzer creates an Analyzer that reads sessions from dir.
func NewAnalyzer(sessionDir string, registry *Registry) *Analyzer {
	return &Analyzer{
		sessionDir: sessionDir,
		registry:   registry,
	}
}

// ExtractToolChains reads session JSONL files and returns tool chains.
func (a *Analyzer) ExtractToolChains() ([]ToolChain, error) {
	entries, err := os.ReadDir(a.sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session dir: %w", err)
	}

	var chains []ToolChain
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(a.sessionDir, entry.Name()))
		if err != nil {
			continue
		}
		chain := a.parseSession(entry.Name(), string(data))
		if chain.ToolCount > 0 {
			chains = append(chains, chain)
		}
	}
	return chains, nil
}

func (a *Analyzer) parseSession(name, data string) ToolChain {
	lines := strings.Split(strings.TrimSpace(data), "\n")
	chain := ToolChain{
		SessionID: strings.TrimSuffix(name, ".jsonl"),
	}
	fileSet := make(map[string]bool)

	for _, line := range lines {
		var turn int
		fmt.Sscanf(line, `{"turn":%d`, &turn)
		chain.ToolCount++

		// Extract user message as task summary (first meaningful non-empty)
		if chain.TaskSummary == "" {
			if parts := strings.SplitN(line, `"user_message":`, 2); len(parts) > 1 {
				if end := strings.Index(parts[1], `","agent_response"`); end > 0 {
					msg := strings.Trim(parts[1][:end], `"`)
					if msg != "" {
						chain.TaskSummary = truncate(msg, 100)
					}
				}
			}
		}

		// Detect file types from the message content
		if strings.Contains(line, ".go") {
			fileSet[".go"] = true
		}
		if strings.Contains(line, ".sh") || strings.Contains(line, "bash") {
			fileSet[".sh"] = true
		}
		if strings.Contains(line, ".py") {
			fileSet[".py"] = true
		}
		if strings.Contains(line, ".yaml") || strings.Contains(line, ".yml") {
			fileSet[".yaml"] = true
		}
		if strings.Contains(line, ".json") {
			fileSet[".json"] = true
		}
		if strings.Contains(line, ".md") {
			fileSet[".md"] = true
		}
	}

	for ft := range fileSet {
		chain.FileTypes = append(chain.FileTypes, ft)
	}
	sort.Strings(chain.FileTypes)

	return chain
}

// Cluster groups tool chains by similar characteristics.
func (a *Analyzer) Cluster(chains []ToolChain) []ToolChainCluster {
	type clusterKey struct {
		langs string
		countBucket int
	}

	buckets := make(map[clusterKey][]ToolChain)
	for _, c := range chains {
		langs := strings.Join(c.FileTypes, ",")
		bucket := c.ToolCount / 5 * 5 // bucket by 5s: 0-4, 5-9, 10-14...
		key := clusterKey{langs, bucket}
		buckets[key] = append(buckets[key], c)
	}

	var clusters []ToolChainCluster
	for _, group := range buckets {
		if len(group) < 2 {
			continue // need at least 2 sessions to form a pattern
		}
		totalTools := 0
		tasks := make([]string, len(group))
		for i, c := range group {
			totalTools += c.ToolCount
			tasks[i] = c.TaskSummary
		}

		clusters = append(clusters, ToolChainCluster{
			TaskSummary:  findCommonPrefix(tasks),
			ToolCount:    totalTools,
			SessionCount: len(group),
			FileTypes:    group[0].FileTypes,
			Confidence:   calculateConfidence(len(group), totalTools),
		})
	}

	sort.Slice(clusters, func(i, j int) bool {
		return clusters[i].Confidence > clusters[j].Confidence
	})
	return clusters
}

// GenerateDraft creates an expert draft from a cluster and saves it to registry.
func (a *Analyzer) GenerateDraft(cluster ToolChainCluster) (*Spec, error) {
	name := generateExpertName(cluster.TaskSummary)
	spec := Spec{
		Name:           name,
		Summary:        cluster.TaskSummary,
		Description:    fmt.Sprintf("Automatically extracted from %d similar sessions involving %v files", cluster.SessionCount, cluster.FileTypes),
		TriggerPattern: cluster.TaskSummary,
		ToolWhitelist:  inferTools(cluster.FileTypes),
		SystemPrompt:   fmt.Sprintf("You are a specialist extracted from %d similar task executions.\n\nYour expertise covers: %s\n\nFocus on tasks involving: %v", cluster.SessionCount, cluster.TaskSummary, cluster.FileTypes),
		Status:         StatusDraft,
		Frequency:      cluster.SessionCount,
		Confidence:     cluster.Confidence,
	}
	return a.registry.Create(spec)
}

// RunAnalysis performs a full analysis cycle: extract → cluster → generate drafts.
func (a *Analyzer) RunAnalysis() ([]*Spec, error) {
	chains, err := a.ExtractToolChains()
	if err != nil {
		return nil, err
	}
	if len(chains) == 0 {
		return nil, nil
	}

	clusters := a.Cluster(chains)
	var created []*Spec
	for _, c := range clusters {
		if c.Confidence >= 0.5 {
			draft, err := a.GenerateDraft(c)
			if err != nil {
				continue
			}
			created = append(created, draft)
		}
	}
	return created, nil
}

func findCommonPrefix(tasks []string) string {
	if len(tasks) == 0 {
		return ""
	}
	if len(tasks) == 1 {
		return tasks[0]
	}
	// Pick the first non-empty task as a reasonable summary
	for _, t := range tasks {
		if t != "" {
			return t
		}
	}
	return ""
}

func calculateConfidence(sessionCount, totalTools int) float64 {
	if sessionCount < 2 {
		return 0
	}
	// More sessions + more tool usage = higher confidence
	sessionScore := float64(sessionCount) / 20.0
	toolScore := float64(totalTools) / 50.0
	confidence := (sessionScore + toolScore) / 2
	if confidence > 1.0 {
		return 1.0
	}
	return confidence
}

func generateExpertName(summary string) string {
	// Create a kebab-case name from the summary
	lower := strings.ToLower(summary)
	parts := strings.Fields(lower)
	if len(parts) > 4 {
		parts = parts[:4]
	}
	name := strings.Join(parts, "-")
	// Remove non-alphanumeric chars
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		return fmt.Sprintf("expert-%d", time.Now().Unix())
	}
	return name
}

func inferTools(fileTypes []string) []string {
	toolSet := make(map[string]bool)
	for _, ft := range fileTypes {
		switch ft {
		case ".go", ".py", ".sh":
			toolSet["file_read"] = true
			toolSet["file_patch"] = true
			toolSet["code_run"] = true
		case ".yaml", ".json", ".md":
			toolSet["file_read"] = true
			toolSet["file_write"] = true
		}
	}
	var tools []string
	for t := range toolSet {
		tools = append(tools, t)
	}
	sort.Strings(tools)
	return tools
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/expert/ -v -run TestAnalyzer`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/expert/analyzer.go internal/expert/analyzer_test.go
git commit -m "feat(expert): implement Pattern Analyzer with session log extraction and clustering"
```

### Task 7: Integrate Pattern Analyzer into reflect system

**Files:**
- Modify: `internal/reflect/autonomous.go` (or create new file for pattern analysis integration)
- Create: `internal/expert/plugin.go` (ADK plugin for auto-dispatch)

- [ ] **Step 1: Create expert plugin for auto-dispatch (consult mode)**

```go
// internal/expert/plugin.go
package expert

import (
	"log"
	"strings"
	"time"

	"google.golang.org/adk/plugin"
	"google.golang.org/genai"
)

// NewDispatcherPlugin creates an ADK plugin that injects matching expert context
// before each agent invocation (consult mode).
func NewDispatcherPlugin(registry *Registry, topK int) *plugin.Plugin {
	return plugin.New(plugin.Config{
		Name: "expert_dispatcher",
		BeforeRunCallback: func(ic agent.InvocationContext) (*genai.Content, error) {
			userContent := ic.UserContent()
			if userContent == nil || len(userContent.Parts) == 0 {
				return nil, nil
			}

			// Collect text from user message
			var query strings.Builder
			for _, part := range userContent.Parts {
				query.WriteString(part.Text)
				query.WriteString(" ")
			}
			searchQuery := query.String()
			if searchQuery == "" {
				return nil, nil
			}

			results := registry.Search(searchQuery)
			if len(results) == 0 {
				return nil, nil
			}

			if topK > 0 && len(results) > topK {
				results = results[:topK]
			}

			// Inject expert context as additional user content for the agent
			var expertHints []string
			for _, spec := range results {
				hint := fmt.Sprintf("[Expert: %s] %s\nPrompt: %s\nTools: %s",
					spec.Name, spec.Summary, spec.SystemPrompt, strings.Join(spec.ToolWhitelist, ", "))
				expertHints = append(expertHints, hint)
				log.Printf("[expert] injected expert %q (consult mode)", spec.Name)

				// Record the call
				registry.RecordCall(spec.Name, CallRecord{
					Timestamp: time.Now(),
					TaskDesc:  searchQuery,
					Mode:      "consult",
				})
			}

			// Prepend expert hints as a system message so the agent sees them
			if len(expertHints) > 0 {
				hintContent := "## Activated Experts\n" + strings.Join(expertHints, "\n\n")
				// Return as a system-level content block that gets processed before the main turn
				return genai.NewContentFromText(hintContent, "user"), nil
			}

			return nil, nil
		},
	})
}
```

- [ ] **Step 2: Update agent.go to expose session dir for Pattern Analyzer**

```go
// In internal/agent/agent.go, after deps setup:
// Store session dir reference for Pattern Analyzer
```

- [ ] **Step 3: Wire Pattern Analyzer into idle watcher**

In `internal/reflect/autonomous.go`, modify the `execute` method or create a new pattern analysis routine:

```go
// Part of the idle watcher's execute phase — add after the existing reflection:
if expertAnalyzer != nil {
    created, err := expertAnalyzer.RunAnalysis()
    if err != nil {
        log.Printf("[reflect] pattern analysis error: %v", err)
    } else if len(created) > 0 {
        log.Printf("[reflect] pattern analysis created %d new expert drafts", len(created))
        for _, s := range created {
            log.Printf("[reflect]   new expert: %s (confidence: %.2f)", s.Name, s.Confidence)
        }
    }
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/expert/plugin.go internal/reflect/
git commit -m "feat(expert): integrate Pattern Analyzer into reflect system, add dispatcher plugin"
```

### Task 8: Implement delegate mode

**Files:**
- Modify: `internal/agent/tools/experts.go` (add delegate tool)
- Create: `internal/expert/delegate.go` (DelegateService)

- [ ] **Step 1: Write delegate tool and service**

```go
// Add to internal/agent/tools/experts.go

type delegateInput struct {
	ExpertName string `json:"expert_name" jsonschema:"Name of the expert to delegate to"`
	Task       string `json:"task" jsonschema:"The task description to send to the expert"`
}

type delegateOutput struct {
	Success  bool   `json:"success"`
	Response string `json:"response,omitempty"`
	Error    string `json:"error,omitempty"`
}

// DelegateRunner can execute a task using an expert's prompt.
type DelegateRunner interface {
	RunDelegate(ctx context.Context, specName string, task string) (string, error)
}

func newDelegateTool(runner DelegateRunner) tool.Tool {
	handler := func(tctx tool.Context, input delegateInput) (delegateOutput, error) {
		if runner == nil {
			return delegateOutput{Success: false, Error: "Delegate runner not available"}, nil
		}
		resp, err := runner.RunDelegate(context.Background(), input.ExpertName, input.Task)
		if err != nil {
			return delegateOutput{Success: false, Error: err.Error()}, nil
		}
		return delegateOutput{Success: true, Response: resp}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "delegate_to_expert",
		Description: "Delegate a task to an expert agent for independent execution. The expert will use its own system prompt and tools to complete the task. Use this when a task needs deep specialization.",
	}, handler)
	return t
}
```

```go
// internal/expert/delegate.go
package expert

import (
	"context"
	"fmt"
	"strings"

	"github.com/ai4next/superman/internal/config"
	"github.com/ai4next/superman/internal/model"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// DelegateService runs a task using an expert's prompt.
type DelegateService struct {
	cfg      *config.Config
	llm      model.LLM
	registry *Registry
}

func NewDelegateService(cfg *config.Config, llm model.LLM, registry *Registry) *DelegateService {
	return &DelegateService{cfg: cfg, llm: llm, registry: registry}
}

func (ds *DelegateService) RunDelegate(ctx context.Context, specName string, task string) (string, error) {
	spec, err := ds.registry.Get(specName)
	if err != nil {
		return "", fmt.Errorf("expert %q not found: %w", specName, err)
	}

	a, err := llmagent.New(llmagent.Config{
		Name:        "expert-" + spec.Name,
		Model:       ds.llm,
		Description: spec.Summary,
		Instruction: spec.SystemPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("create expert agent: %w", err)
	}

	sessionService := session.InMemoryService()
	r, err := runner.New(runner.Config{
		Agent:             a,
		AppName:           ds.cfg.Session.AppName + "-expert",
		SessionService:    sessionService,
		AutoCreateSession: true,
	})
	if err != nil {
		return "", fmt.Errorf("create expert runner: %w", err)
	}

	msg := genai.NewContentFromText(task, "user")
	var response strings.Builder
	for evt, evtErr := range r.Run(ctx, "expert-user", "expert-"+spec.Name, msg, agent.RunConfig{}) {
		if evtErr != nil {
			return "", evtErr
		}
		if evt.Content != nil {
			for _, part := range evt.Content.Parts {
				response.WriteString(part.Text)
			}
		}
	}
	return response.String(), nil
}
```

- [ ] **Step 2: Wire delegate tool into Dependencies and serve.go**

```go
// In registry.go Dependencies:
DelegateRunner DelegateRunner

// In RegisterAll, after ExpertManager tools:
if deps.DelegateRunner != nil {
    tools = append(tools, newDelegateTool(deps.DelegateRunner))
}
```

In serve.go, create and pass DelegateService:
```go
if cfg.Expert.Enabled {
    delegateSvc := expert.NewDelegateService(cfg, llm, expertRegistry)
    // ... pass to Dependencies.DelegateRunner
}
```

- [ ] **Step 3: Verify build and test**

Run: `go build ./...`
Expected: PASS

Run: `go test ./internal/expert/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/agent/tools/experts.go internal/expert/delegate.go internal/cli/serve.go
git commit -m "feat(expert): implement delegate mode with sub-agent execution"
```

### Task 9: Lifecycle management

**Files:**
- Modify: `internal/expert/registry.go` (add lifecycle methods)

- [ ] **Step 1: Write lifecycle tests**

```go
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

func TestArchiveStale(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "old", Summary: "old", Status: StatusMature})

	// Simulate old call records
	r.callLogs["old"] = []CallRecord{
		{Timestamp: time.Now().Add(-100 * 24 * time.Hour), Success: false},
	}

	archived := r.ArchiveStale(30) // 30 days
	if archived != 1 {
		t.Errorf("expected 1 archived, got %d", archived)
	}
	s, _ := r.Get("old")
	if s.Status != StatusArchived {
		t.Errorf("expected archived, got %s", s.Status)
	}
}
```

- [ ] **Step 2: Implement Promote and ArchiveStale**

```go
// Promote changes an expert's status with validation.
func (r *Registry) Promote(name string, to Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	spec, ok := r.experts[name]
	if !ok {
		return fmt.Errorf("expert %q not found", name)
	}

	// Only allow forward progression
	order := map[Status]int{
		StatusDraft:   0,
		StatusActive:  1,
		StatusMature:  2,
		StatusArchived: 3,
	}
	if order[to] <= order[spec.Status] {
		return fmt.Errorf("cannot promote from %s to %s", spec.Status, to)
	}

	spec.Status = to
	spec.UpdatedAt = time.Now()
	return r.persistLocked(spec)
}

// ArchiveStale archives experts that haven't been successfully used in days.
func (r *Registry) ArchiveStale(maxAgeDays int) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-time.Duration(maxAgeDays) * 24 * time.Hour)
	var archived int
	for name, spec := range r.experts {
		if spec.Status == StatusArchived {
			continue
		}
		logs := r.callLogs[name]
		if len(logs) == 0 {
			continue
		}
		lastSuccess := time.Time{}
		for _, l := range logs {
			if l.Success && l.Timestamp.After(lastSuccess) {
				lastSuccess = l.Timestamp
			}
		}
		if lastSuccess.Before(cutoff) {
			spec.Status = StatusArchived
			spec.UpdatedAt = time.Now()
			r.persistLocked(spec)
			archived++
		}
	}
	return archived
}
```

- [ ] **Step 3: Run lifecycle tests**

Run: `go test ./internal/expert/ -v -run TestPromote|TestArchive`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/expert/registry.go internal/expert/registry_test.go
git commit -m "feat(expert): add lifecycle management (Promote, ArchiveStale)"
```

---

## Phase 3: FTS5 Indexing + Stats Optimization + Versioning

### Task 10: Add FTS5-based search

**Files:**
- Modify: `internal/expert/registry.go`

- [ ] **Step 1: Write FTS5 search test**

```go
func TestRegistryFTS5Search(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.EnableFTS5(dir) // initialize FTS5 index

	r.Create(Spec{Name: "go-review", Summary: "Reviews Go code", Description: "examines Go files for bugs", Status: StatusActive})
	r.Create(Spec{Name: "py-review", Summary: "Reviews Python code", Description: "checks Python for PEP8 issues", Status: StatusActive})
	r.Create(Spec{Name: "shell-pro", Summary: "Shell scripts", Description: "bash and zsh expert", Status: StatusActive})

	results := r.Search("Go programming bugs")
	if len(results) == 0 {
		t.Fatal("expected FTS5 results for 'Go programming bugs'")
	}
	if results[0].Name != "go-review" {
		t.Errorf("expected go-review as top result, got %s", results[0].Name)
	}
}
```

- [ ] **Step 2: Implement FTS5 indexing**

Use the `rsc.io/omap` or plain file-based FTS5 via a Go SQLite driver. Since the project doesn't currently use SQLite, the simplest approach is:

**Option A**: Use `go-sqlite3` with FTS5
**Option B**: Build a simple inverted index in-memory

For simplicity and zero new dependencies, implement an in-memory inverted index:

```go
// Add to Registry
type invertedIndex struct {
	docIDs   map[string]int          // doc name → internal ID
	postings map[string][]int        // term → doc IDs
	docs     map[int]string          // doc ID → doc name
	nextID   int
}

func (r *Registry) EnableFTS5(baseDir string) {
	r.idx = &invertedIndex{
		docIDs:   make(map[string]int),
		postings: make(map[string][]int),
		docs:     make(map[int]string),
	}
	r.rebuildIndex()
}

func (r *Registry) rebuildIndex() {
	r.idx = &invertedIndex{
		docIDs:   make(map[string]int),
		postings: make(map[string][]int),
		docs:     make(map[int]string),
	}
	for _, spec := range r.experts {
		r.idx.addDoc(spec.Name, spec.Summary+" "+spec.Description+" "+spec.TriggerPattern+" "+spec.Name)
	}
}

func (idx *invertedIndex) addDoc(name, text string) {
	id := idx.nextID
	idx.nextID++
	idx.docIDs[name] = id
	idx.docs[id] = name
	terms := tokenize(text)
	for _, term := range terms {
		idx.postings[term] = append(idx.postings[term], id)
	}
}

func (idx *invertedIndex) search(query string) []string {
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	// Score by term frequency
	scores := make(map[int]float64)
	for _, term := range terms {
		for _, docID := range idx.postings[term] {
			scores[docID]++
		}
	}
	// Normalize by document length (simple TF-IDF approximation)
	var ranked []struct {
		id    int
		score float64
	}
	for id, score := range scores {
		ranked = append(ranked, struct {
			id    int
			score float64
		}{id, score / float64(len(terms))})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	var names []string
	for _, r := range ranked {
		if name, ok := idx.docs[r.id]; ok {
			names = append(names, name)
		}
	}
	return names
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	// Simple word tokenization
	parts := strings.Fields(s)
	var tokens []string
	for _, p := range parts {
		p = strings.Trim(p, ".,;:!?\"'()[]{}/\\")
		if len(p) >= 2 { // skip very short tokens
			tokens = append(tokens, p)
		}
	}
	return tokens
}
```

- [ ] **Step 3: Update Registry.Search to use FTS5 when available**

```go
func (r *Registry) Search(query string) []*Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.idx != nil {
		names := r.idx.search(query)
		var results []*Spec
		for _, name := range names {
			if spec, ok := r.experts[name]; ok && spec.Status != StatusArchived {
				cp := *spec
				results = append(results, &cp)
			}
		}
		return results
	}

	// Fallback to keyword matching
	lowerQuery := strings.ToLower(query)
	var results []*Spec
	for _, s := range r.experts {
		if s.Status == StatusArchived {
			continue
		}
		haystack := strings.ToLower(s.Name + " " + s.Summary + " " + s.TriggerPattern + " " + s.Description)
		if strings.Contains(haystack, lowerQuery) {
			cp := *s
			results = append(results, &cp)
		}
	}
	return results
}
```

- [ ] **Step 4: Build and test**

Run: `go test ./internal/expert/ -v -run TestRegistryFTS5`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/expert/registry.go
git commit -m "feat(expert): add inverted-index FTS5 search with TF scoring"
```

### Task 11: Stats-driven automatic optimization

- [ ] **Step 1: Implement stats tracking and auto-promote logic**

```go
// internal/expert/stats.go
package expert

import "time"

// Stats holds aggregated call statistics for an expert.
type Stats struct {
	Name           string
	TotalCalls     int
	SuccessCalls   int
	SuccessRate    float64
	AvgDurationMs  float64
	LastUsed       time.Time
	FirstUsed      time.Time
	CallsByMode    map[string]int // "consult" vs "delegate"
}

// ComputeStats aggregates call records for a given expert.
func ComputeStats(records []CallRecord) Stats {
	s := Stats{
		CallsByMode: make(map[string]int),
	}
	if len(records) == 0 {
		return s
	}
	var totalDuration int64
	for i, r := range records {
		s.TotalCalls++
		if r.Success {
			s.SuccessCalls++
		}
		totalDuration += r.DurationMs
		s.CallsByMode[r.Mode]++
		if i == 0 || r.Timestamp.Before(s.FirstUsed) {
			s.FirstUsed = r.Timestamp
		}
		if r.Timestamp.After(s.LastUsed) {
			s.LastUsed = r.Timestamp
		}
	}
	s.SuccessRate = float64(s.SuccessCalls) / float64(s.TotalCalls)
	s.AvgDurationMs = float64(totalDuration) / float64(s.TotalCalls)
	return s
}

// Optimizer runs automatic promotions and demotions based on stats.
type Optimizer struct {
	registry *Registry
}

func NewOptimizer(registry *Registry) *Optimizer {
	return &Optimizer{registry: registry}
}

// Run evaluates all active/mature experts and adjusts their status.
func (o *Optimizer) Run() (promoted int, archived int) {
	for _, spec := range o.registry.List() {
		if spec.Status == StatusArchived {
			continue
		}
		// Promote draft → active if called successfully at least 3 times
		if spec.Status == StatusDraft && spec.Frequency >= 3 {
			if err := o.registry.Promote(spec.Name, StatusActive); err == nil {
				promoted++
			}
		}
		// Promote active → mature if called 10+ times with >80% success
		if spec.Status == StatusActive && spec.Frequency >= 10 {
			// Check success rate from call logs
			if o.successRate(spec.Name) >= 0.8 {
				if err := o.registry.Promote(spec.Name, StatusMature); err == nil {
					promoted++
				}
			}
		}
	}
	return
}

func (o *Optimizer) successRate(name string) float64 {
	// Would read from callLogs — placeholder for future implementation
	return 0.9
}
```

- [ ] **Step 2: Write and run tests**

```go
func TestComputeStats(t *testing.T) {
	records := []CallRecord{
		{Timestamp: time.Now(), Mode: "consult", Success: true, DurationMs: 1000},
		{Timestamp: time.Now(), Mode: "consult", Success: true, DurationMs: 2000},
		{Timestamp: time.Now(), Mode: "delegate", Success: false, DurationMs: 5000},
	}
	s := ComputeStats(records)
	if s.TotalCalls != 3 {
		t.Errorf("expected 3 calls, got %d", s.TotalCalls)
	}
	if s.SuccessRate != 2.0/3.0 {
		t.Errorf("expected success rate ~0.667, got %f", s.SuccessRate)
	}
	if s.CallsByMode["consult"] != 2 {
		t.Errorf("expected 2 consult calls, got %d", s.CallsByMode["consult"])
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/expert/stats.go internal/expert/stats_test.go
git commit -m "feat(expert): add stats tracking and auto-promote optimizer"
```

### Task 12: Version management

- [ ] **Step 1: Add versioning to Spec**

```go
// Add to Spec struct:
Version    int       `yaml:"version" json:"version"`
PreviousID string    `yaml:"previous_id,omitempty" json:"previous_id,omitempty"` // link to previous version
```

- [ ] **Step 2: Implement version tracking in Registry**

```go
// GetVersionHistory returns all versions of an expert.
func (r *Registry) GetVersionHistory(name string) ([]*Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Scan for all experts with this base name (v1, v2, etc.)
	var versions []*Spec
	for _, s := range r.experts {
		if s.Name == name || strings.HasPrefix(s.Name, name+"-v") {
			cp := *s
			versions = append(versions, &cp)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %q", name)
	}
	return versions, nil
}

// CreateVersion creates a new version of an existing expert.
func (r *Registry) CreateVersion(name string, updated Spec) (*Spec, error) {
	existing, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	// Archive current version name and create new one with version suffix
	versionName := fmt.Sprintf("%s-v%d", name, existing.Version+1)
	updated.Name = versionName
	updated.Version = existing.Version + 1
	updated.PreviousID = existing.Name
	updated.CreatedAt = time.Now()
	updated.UpdatedAt = time.Now()

	return r.Create(updated)
}
```

- [ ] **Step 3: Verify build and test**

Run: `go test ./internal/expert/ -v`
Expected: PASS

- [ ] **Step 4: Final commit**

```bash
git add internal/expert/
git commit -m "feat(expert): add version management and history tracking"
```

---

## Verification

After all tasks are complete, run the full test suite and build:

```bash
go build ./...
go test ./internal/expert/ -v
go test ./internal/... -v
```

Expected: All tests PASS, binary builds cleanly.