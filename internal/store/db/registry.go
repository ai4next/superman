package db

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type DBRegistry struct {
	mu         sync.Mutex
	GlobalDB   *DB
	SupermanDB *DB
	ExpertDBs  map[string]*DB
}

type RegistryPaths struct {
	GlobalDBPath string
	SupermanPath string
	ExpertsDir   string
}

func NewDBRegistry(paths RegistryPaths) (*DBRegistry, error) {
	globalDB, err := OpenPath(paths.GlobalDBPath)
	if err != nil {
		return nil, fmt.Errorf("open global db: %w", err)
	}
	supermanDB, err := OpenPath(paths.SupermanPath)
	if err != nil {
		return nil, fmt.Errorf("open superman db: %w", err)
	}
	expertDBs, err := openExpertDBs(paths.ExpertsDir)
	if err != nil {
		return nil, err
	}
	return &DBRegistry{
		GlobalDB:   globalDB,
		SupermanDB: supermanDB,
		ExpertDBs:  expertDBs,
	}, nil
}

func (r *DBRegistry) AgentDB(name string) *DB {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" || name == "superman" {
		return r.SupermanDB
	}
	return r.ExpertDBs[name]
}

func (r *DBRegistry) EnsureAgentDB(name, path string) (*DB, error) {
	if r == nil {
		return nil, fmt.Errorf("db registry is unavailable")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if name == "" || name == "superman" {
		return r.SupermanDB, nil
	}
	if existing := r.ExpertDBs[name]; existing != nil {
		return existing, nil
	}
	db, err := OpenPath(path)
	if err != nil {
		return nil, fmt.Errorf("open agent db %s: %w", name, err)
	}
	r.ExpertDBs[name] = db
	return db, nil
}

func openExpertDBs(expertsDir string) (map[string]*DB, error) {
	entries, err := os.ReadDir(expertsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]*DB{}, nil
		}
		return nil, fmt.Errorf("read expert db dirs: %w", err)
	}
	out := make(map[string]*DB)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "superman" {
			continue
		}
		soulPath := filepath.Join(expertsDir, entry.Name(), "soul.md")
		if _, err := os.Stat(soulPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		db, err := OpenPath(filepath.Join(expertsDir, entry.Name(), "state.db"))
		if err != nil {
			return nil, fmt.Errorf("open expert db %s: %w", entry.Name(), err)
		}
		out[entry.Name()] = db
	}
	return out, nil
}
