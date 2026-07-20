package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FileStore struct {
	Dir string
}

func (s FileStore) Save(plan Plan) error {
	if s.Dir == "" {
		return fmt.Errorf("plan store dir is required")
	}
	if err := validatePlanID(plan.ID); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("create plan store: %w", err)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.Dir, ".plan-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary plan: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary plan: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set temporary plan permissions: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary plan: %w", err)
	}
	if err := os.Rename(tmpPath, s.path(plan.ID)); err != nil {
		return fmt.Errorf("replace plan: %w", err)
	}
	return nil
}

func (s FileStore) Load(id string) (Plan, error) {
	if err := validatePlanID(id); err != nil {
		return Plan{}, err
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return Plan{}, fmt.Errorf("read plan %s: %w", id, err)
	}
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, fmt.Errorf("decode plan %s: %w", id, err)
	}
	if plan.ID != id {
		return Plan{}, fmt.Errorf("plan id mismatch: file %s contains %s", id, plan.ID)
	}
	return plan, nil
}

func (s FileStore) List() ([]Plan, error) {
	if strings.TrimSpace(s.Dir) == "" {
		return nil, fmt.Errorf("plan store dir is required")
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list plans: %w", err)
	}
	plans := make([]Plan, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		plan, err := s.Load(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].ID < plans[j].ID })
	return plans, nil
}

func validatePlanID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || filepath.Base(id) != id {
		return fmt.Errorf("invalid plan id")
	}
	return nil
}

func (s FileStore) path(id string) string {
	return filepath.Join(s.Dir, id+".json")
}
