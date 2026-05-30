package orchestrator

import "testing"

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
