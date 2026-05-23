package expert

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"go.yaml.in/yaml/v3"
)

// Registry manages expert definitions with file-based persistence.
type Registry struct {
	mu      sync.RWMutex
	dir     string
	experts map[string]*Spec
}

// NewRegistry creates a registry rooted at dir, where each expert lives under
// dir/{expert_name}/expert.yaml.
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
	data, err := yaml.Marshal(spec)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "expert.yaml")
	return os.WriteFile(path, data, 0644)
}