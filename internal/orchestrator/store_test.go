package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreSaveLoad(t *testing.T) {
	store := FileStore{Dir: t.TempDir()}
	plan := Plan{ID: "p1", Goal: "goal", Tasks: []TaskNode{{ID: "t1", Expert: "expert", Input: TaskInput{Prompt: "do it"}}}}
	plan.SetState("t1", TaskStatusQueued)
	if err := store.Save(plan); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load("p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "p1" || got.State("t1") != TaskStatusQueued {
		t.Fatalf("plan = %#v", got)
	}
}

func TestFileStoreRejectsInvalidPlanIDs(t *testing.T) {
	store := FileStore{Dir: t.TempDir()}
	plan := Plan{ID: "../outside", Goal: "goal"}
	if err := store.Save(plan); err == nil {
		t.Fatal("Save() error = nil, want invalid plan id")
	}
	if err := os.WriteFile(filepath.Join(store.Dir, "p1.json"), []byte(`{"plan_id":"p2"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("p1"); err == nil {
		t.Fatal("Load() error = nil, want plan id mismatch")
	}
}
