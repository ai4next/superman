package expert

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// Registry manages expert definitions with file-based persistence.
type Registry struct {
	mu       sync.RWMutex
	dir      string
	experts  map[string]*Spec
	callLogs map[string][]CallRecord
	idx      *invertedIndex
}

// NewRegistry creates a registry rooted at dir, where each expert lives under
// dir/{expert_name}/expert.yaml.
func NewRegistry(dir string) *Registry {
	return &Registry{
		dir:      dir,
		experts:  make(map[string]*Spec),
		callLogs: make(map[string][]CallRecord),
	}
}

// LoadFromDisk reads all expert definitions from disk into the registry.
func (r *Registry) LoadFromDisk() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dir)
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
		specPath := filepath.Join(r.dir, entry.Name(), "expert.yaml")
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

	// Update inverted index if enabled
	if r.idx != nil {
		r.idx.addDoc(spec.Name, spec.Summary+" "+spec.Description+" "+spec.TriggerPattern+" "+spec.Name)
	}

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

	dir := filepath.Join(r.dir, name)
	if err := os.RemoveAll(dir); err != nil {
		log.Printf("[expert] error removing disk dir for %s: %v", name, err)
	}
	return nil
}

// Promote changes an expert's status with forward-progression validation.
func (r *Registry) Promote(name string, to Status) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	spec, ok := r.experts[name]
	if !ok {
		return fmt.Errorf("expert %q not found", name)
	}

	order := map[Status]int{
		StatusDraft:    0,
		StatusActive:   1,
		StatusMature:   2,
		StatusArchived: 3,
	}
	if order[to] <= order[spec.Status] {
		return fmt.Errorf("cannot promote from %s to %s", spec.Status, to)
	}

	spec.Status = to
	spec.UpdatedAt = time.Now()
	return r.persistLocked(spec)
}

// ArchiveStale archives experts that haven't been successfully used in maxAgeDays.
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
		if !lastSuccess.IsZero() && lastSuccess.Before(cutoff) {
			spec.Status = StatusArchived
			spec.UpdatedAt = time.Now()
			r.persistLocked(spec)
			archived++
		}
	}
	return archived
}

// Search performs keyword matching against name, summary, trigger pattern,
// and description. Uses inverted index when enabled, falls back to substring match.
func (r *Registry) Search(query string) []*Spec {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.idx != nil {
		names := r.idx.search(query)
		var results []*Spec
		for _, name := range names {
			if spec, ok := r.experts[name]; ok && spec.Status != StatusArchived {
				results = append(results, copySpec(spec))
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

// GetVersionHistory returns all versions of an expert, sorted by version.
func (r *Registry) GetVersionHistory(name string) ([]*Spec, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var versions []*Spec
	for _, s := range r.experts {
		if s.Name == name || strings.HasPrefix(s.Name, name+"-v") {
			versions = append(versions, copySpec(s))
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for %q", name)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].Version < versions[j].Version
	})
	return versions, nil
}

// CreateVersion creates a new version of an existing expert.
func (r *Registry) CreateVersion(name string, updated Spec) (*Spec, error) {
	existing, err := r.Get(name)
	if err != nil {
		return nil, err
	}

	updated.Name = fmt.Sprintf("%s-v%d", name, existing.Version+1)
	updated.Version = existing.Version + 1
	updated.PreviousID = existing.Name
	updated.CreatedAt = time.Now()
	updated.UpdatedAt = time.Now()
	if updated.Status == "" {
		updated.Status = existing.Status
	}

	return r.Create(updated)
}

// persistLocked writes a single expert spec to disk. Must be called while
// holding r.mu write lock.
func (r *Registry) persistLocked(spec *Spec) error {
	dir := filepath.Join(r.dir, spec.Name)
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

// invertedIndex provides TF-scored full-text search over expert documents.
type invertedIndex struct {
	docIDs   map[string]int   // expert name → internal doc ID
	postings map[string][]int // term → list of doc IDs
	docs     map[int]string   // doc ID → expert name
	nextID   int
}

// EnableFTS5 initializes the inverted index and rebuilds it from current experts.
func (r *Registry) EnableFTS5() {
	r.mu.Lock()
	defer r.mu.Unlock()

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
	scores := make(map[int]float64)
	for _, term := range terms {
		for _, docID := range idx.postings[term] {
			scores[docID]++
		}
	}
	type scored struct {
		id    int
		score float64
	}
	var ranked []scored
	for id, score := range scores {
		ranked = append(ranked, scored{id, score / float64(len(terms))})
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
	parts := strings.Fields(s)
	var tokens []string
	for _, p := range parts {
		p = strings.Trim(p, ".,;:!?\"'()[]{}/\\")
		if len(p) >= 2 {
			tokens = append(tokens, p)
		}
	}
	return tokens
}
