package expert

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Registry manages expert definitions with file-based persistence.
type Registry struct {
	mu      sync.RWMutex
	dir     string
	experts map[string]*Spec
}

// NewRegistry creates a registry rooted at dir, where each expert lives under
// dir/{expert_name}/soul.md.
func NewRegistry(dir string) *Registry {
	return &Registry{
		dir:     dir,
		experts: make(map[string]*Spec),
	}
}

// LoadFromDisk reads all expert definitions from disk into the registry.
func (r *Registry) LoadFromDisk() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			r.experts = make(map[string]*Spec)
			return nil
		}
		return fmt.Errorf("read experts dir: %w", err)
	}

	experts := make(map[string]*Spec)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		specPath := filepath.Join(r.dir, name, "soul.md")
		data, err := os.ReadFile(specPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Printf("[expert] skip %s: %v", entry.Name(), err)
			continue
		}
		prompt := string(data)
		if prompt == "" {
			log.Printf("[expert] skip %s: empty soul.md", entry.Name())
			continue
		}
		spec := Spec{Name: name, SystemPrompt: prompt}
		experts[spec.Name] = &spec
	}
	r.experts = experts
	return nil
}

// Get retrieves a copy of an expert by name.
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

// List returns copies of all experts.
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

// persistLocked writes a single expert spec to disk. Must be called while
// holding r.mu write lock.
func (r *Registry) persistLocked(spec *Spec) error {
	dir := filepath.Join(r.dir, spec.Name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "soul.md")
	return os.WriteFile(path, []byte(spec.SystemPrompt), 0644)
}
