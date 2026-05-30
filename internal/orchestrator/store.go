package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type FileStore struct {
	Dir string
}

func (s FileStore) Save(plan Plan) error {
	if s.Dir == "" {
		return fmt.Errorf("plan store dir is required")
	}
	if plan.ID == "" {
		return fmt.Errorf("plan id is required")
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create plan store: %w", err)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path(plan.ID), append(data, '\n'), 0o644)
}

func (s FileStore) Load(id string) (Plan, error) {
	if id == "" {
		return Plan{}, fmt.Errorf("plan id is required")
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return Plan{}, fmt.Errorf("read plan %s: %w", id, err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("decode plan %s: %w", id, err)
	}
	return plan, nil
}

func (s FileStore) path(id string) string {
	return filepath.Join(s.Dir, id+".json")
}
