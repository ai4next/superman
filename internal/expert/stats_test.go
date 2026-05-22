package expert

import (
	"testing"
	"time"
)

func TestComputeStats(t *testing.T) {
	now := time.Now()
	records := []CallRecord{
		{Timestamp: now.Add(-3 * time.Hour), Mode: ModeConsult, Success: true, DurationMs: 1000},
		{Timestamp: now.Add(-2 * time.Hour), Mode: ModeConsult, Success: true, DurationMs: 2000},
		{Timestamp: now.Add(-1 * time.Hour), Mode: ModeDelegate, Success: false, DurationMs: 5000},
	}
	s := ComputeStats(records)
	if s.TotalCalls != 3 {
		t.Errorf("expected 3 calls, got %d", s.TotalCalls)
	}
	if s.SuccessRate != 2.0/3.0 {
		t.Errorf("expected success rate ~0.667, got %f", s.SuccessRate)
	}
	if s.CallsByMode["consult"] != 2 {
		t.Errorf("expected 2 consult calls, got %d", s.CallsByMode["consult"])
	}
	if s.CallsByMode["delegate"] != 1 {
		t.Errorf("expected 1 delegate call, got %d", s.CallsByMode["delegate"])
	}
	if s.AvgDurationMs != 8000.0/3.0 {
		t.Errorf("expected avg duration ~2666.67, got %f", s.AvgDurationMs)
	}
}

func TestComputeStatsEmpty(t *testing.T) {
	s := ComputeStats(nil)
	if s.TotalCalls != 0 {
		t.Errorf("expected 0 calls for empty, got %d", s.TotalCalls)
	}
}

func TestOptimizerPromotesDraft(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "test", Status: StatusDraft})

	// Record 3 successful calls
	for i := 0; i < 3; i++ {
		r.RecordCall("test", CallRecord{Timestamp: time.Now(), Success: true, Mode: ModeConsult})
	}

	o := NewOptimizer(r)
	promoted, archived := o.Run()
	if promoted != 1 {
		t.Errorf("expected 1 promotion, got %d", promoted)
	}
	if archived != 0 {
		t.Errorf("expected 0 archived, got %d", archived)
	}
}

func TestOptimizerDoesNotPromoteLowCalls(t *testing.T) {
	dir := t.TempDir()
	r := NewRegistry(dir)
	r.Create(Spec{Name: "test", Summary: "test", Status: StatusDraft})

	// Only 2 successful calls -- not enough for promotion
	r.RecordCall("test", CallRecord{Timestamp: time.Now(), Success: true, Mode: ModeConsult})
	r.RecordCall("test", CallRecord{Timestamp: time.Now(), Success: true, Mode: ModeConsult})

	o := NewOptimizer(r)
	promoted, _ := o.Run()
	if promoted != 0 {
		t.Errorf("expected 0 promotions (only 2 calls), got %d", promoted)
	}
}
