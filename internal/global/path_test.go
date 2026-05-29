package global

import (
	"path/filepath"
	"testing"

	"github.com/ai4next/superman/internal/config"
)

func TestRuntimeEventsPathAliasesBusEventsPath(t *testing.T) {
	workspace := t.TempDir()
	SetConfig(&config.Config{Workspace: workspace})
	t.Cleanup(func() { SetConfig(nil) })

	want := filepath.Join(workspace, "bus", "events.jsonl")
	if got := BusEventsPath(); got != want {
		t.Fatalf("BusEventsPath() = %q, want %q", got, want)
	}
	if got := RuntimeEventsPath(); got != want {
		t.Fatalf("RuntimeEventsPath() = %q, want %q", got, want)
	}
}
