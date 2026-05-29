package runtime

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ai4next/superman/internal/bus"
)

func TestLoopDetectorDetectsRepeatedToolInteractions(t *testing.T) {
	detector := NewLoopDetector(LoopDetectionConfig{
		Enabled:    true,
		WindowSize: 10,
		MaxRepeats: 5,
	})
	for i := 0; i < 9; i++ {
		if err := observeTool(detector, i, "read", `{"path":"a.go"}`, `{"content":"same"}`); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}
	err := observeTool(detector, 9, "read", `{"path":"a.go"}`, `{"content":"same"}`)
	if !errors.Is(err, ErrLoopDetected) {
		t.Fatalf("err = %v, want ErrLoopDetected", err)
	}
}

func TestLoopDetectorIgnoresDifferentInteractions(t *testing.T) {
	detector := NewLoopDetector(LoopDetectionConfig{
		Enabled:    true,
		WindowSize: 10,
		MaxRepeats: 5,
	})
	for i := 0; i < 10; i++ {
		if err := observeTool(detector, i, "read", fmt.Sprintf(`{"path":"%d.go"}`, i), `{"content":"same"}`); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}
}

func TestLoopDetectorWaitsForFullWindow(t *testing.T) {
	detector := NewLoopDetector(LoopDetectionConfig{
		Enabled:    true,
		WindowSize: 10,
		MaxRepeats: 5,
	})
	for i := 0; i < 6; i++ {
		if err := observeTool(detector, i, "read", `{"path":"a.go"}`, `{"content":"same"}`); err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}
}

func TestNewLoopDetectorDisabled(t *testing.T) {
	if got := NewLoopDetector(LoopDetectionConfig{}); got != nil {
		t.Fatalf("detector = %#v, want nil", got)
	}
}

func observeTool(detector *LoopDetector, i int, name, args, result string) error {
	toolID := fmt.Sprintf("tool-%d", i)
	if err := detector.Observe(bus.Event{
		Type:     bus.EventToolCallStarted,
		ToolID:   toolID,
		ToolName: name,
		Args:     args,
	}); err != nil {
		return err
	}
	return detector.Observe(bus.Event{
		Type:     bus.EventToolCallFinished,
		ToolID:   toolID,
		ToolName: name,
		Result:   result,
		Status:   "done",
	})
}
