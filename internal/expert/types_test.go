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
	if !s.CreatedAt.IsZero() {
		t.Errorf("expected zero CreatedAt, got %v", s.CreatedAt)
	}
	if !s.UpdatedAt.IsZero() {
		t.Errorf("expected zero UpdatedAt, got %v", s.UpdatedAt)
	}
	if s.Confidence != 0.0 {
		t.Errorf("expected zero Confidence, got %f", s.Confidence)
	}
	if s.Frequency != 0 {
		t.Errorf("expected zero Frequency, got %d", s.Frequency)
	}
}

func TestCallRecord(t *testing.T) {
	now := time.Now()
	r := CallRecord{
		Timestamp:  now,
		TaskDesc:   "review PR #42",
		Mode:       ModeConsult,
		Success:    true,
		DurationMs: 1500,
	}
	if r.Mode != ModeConsult {
		t.Errorf("expected %q, got %q", ModeConsult, r.Mode)
	}
	if !r.Success {
		t.Errorf("expected success to be true")
	}
	if r.TaskDesc != "review PR #42" {
		t.Errorf("got %q, want %q", r.TaskDesc, "review PR #42")
	}
	if r.DurationMs != 1500 {
		t.Errorf("got %d, want %d", r.DurationMs, 1500)
	}
	if !r.Timestamp.Equal(now) {
		t.Errorf("got %v, want %v", r.Timestamp, now)
	}
}