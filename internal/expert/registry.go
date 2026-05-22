package expert

import (
	"fmt"
	"log"
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

// LoadFromDisk reads all expert definitions from disk into the registry.
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
			log.Printf("[expert] skip %s: %v", entry.Name(), err)
			continue
		}
		var spec Spec
		if err := yaml.Unmarshal(data, &spec); err != nil {
			log.Printf("[expert] skip %s: bad YAML: %v", entry.Name(), err)
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

	// Persist to disk first, then add to memory
	if err := r.persistLocked(&spec); err != nil {
		return nil, err
	}
	r.experts[spec.Name] = &spec
	return &spec, nil
}

// Get retrieves a defensive copy of an expert by name.
func (r *Registry) Get(name string) (*Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.experts[name]
	if !ok {
		return nil, fmt.Errorf("expert %q not found", name)
	}
	return copySpec(spec), nil
}

// List returns defensive copies of all experts.
func (r *Registry) List() []*Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Spec, 0, len(r.experts))
	for _, s := range r.experts {
		result = append(result, copySpec(s))
	}
	return result
}

// Update modifies an existing expert's mutable fields.
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
	existing.ToolAllowlist = spec.ToolAllowlist
	existing.SystemPrompt = spec.SystemPrompt
	existing.Status = spec.Status
	existing.Frequency = spec.Frequency
	existing.Confidence = spec.Confidence
	existing.UpdatedAt = time.Now()

	return r.persistLocked(existing)
}

// Delete removes an expert from the registry and its disk directory.
func (r *Registry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.experts[name]; !ok {
		return fmt.Errorf("expert %q not found", name)
	}
	delete(r.experts, name)
	delete(r.callLogs, name)

	dir := filepath.Join(r.baseDir, "data", "experts", name)
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("[expert] error removing disk dir for %s: %v", name, err)
	}
	return nil
}

// Search performs keyword matching against name, summary, trigger pattern,
// and description. Only returns active and mature experts (excludes archived).
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
			results = append(results, copySpec(s))
		}
	}
	return results
}

// RecordCall logs an invocation for an expert. Returns error if expert doesn't exist.
func (r *Registry) RecordCall(name string, record CallRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.experts[name]; !ok {
		return fmt.Errorf("expert %q not found", name)
	}
	r.callLogs[name] = append(r.callLogs[name], record)
	return nil
}

// GetCallRecords returns call records for an expert.
func (r *Registry) GetCallRecords(name string) []CallRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	records := r.callLogs[name]
	result := make([]CallRecord, len(records))
	copy(result, records)
	return result
}

// persistLocked writes a single expert spec to disk. Must be called while
// holding r.mu write lock.
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

// copySpec returns a deep copy of a Spec.
func copySpec(s *Spec) *Spec {
	cp := *s
	if s.ToolAllowlist != nil {
		cp.ToolAllowlist = make([]string, len(s.ToolAllowlist))
		copy(cp.ToolAllowlist, s.ToolAllowlist)
	}
	return &cp
}