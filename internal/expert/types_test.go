package expert

import (
	"testing"
	"time"
)

func TestSpecDefaults(t *testing.T) {
	s := Spec{
		Name:    "test-expert",
		Summary: "A test expert",
	}
	if s.Status != "" {
		t.Errorf("expected empty status, got %s", s.Status)
	}
}

func TestCallRecord(t *testing.T) {
	r := CallRecord{
		Timestamp: time.Now(),
		TaskDesc:  "review PR #42",
		Mode:      "consult",
		Success:   true,
	}
	if r.Mode != "consult" {
		t.Errorf("expected consult mode, got %s", r.Mode)
	}
	if !r.Success {
		t.Errorf("expected success to be true")
	}
}