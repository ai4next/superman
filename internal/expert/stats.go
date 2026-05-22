package expert

import (
	"log"
	"time"
)

// Stats holds aggregated call statistics for an expert.
type Stats struct {
	Name          string
	TotalCalls    int
	SuccessCalls  int
	SuccessRate   float64
	AvgDurationMs float64
	LastUsed      time.Time
	FirstUsed     time.Time
	CallsByMode   map[string]int
}

// ComputeStats aggregates call records into Stats.
func ComputeStats(records []CallRecord) Stats {
	s := Stats{
		CallsByMode: make(map[string]int),
	}
	if len(records) == 0 {
		return s
	}
	var totalDuration int64
	for _, r := range records {
		s.TotalCalls++
		if r.Success {
			s.SuccessCalls++
		}
		totalDuration += r.DurationMs
		mode := string(r.Mode)
		if mode == "" {
			mode = "unknown"
		}
		s.CallsByMode[mode]++
		if s.FirstUsed.IsZero() || r.Timestamp.Before(s.FirstUsed) {
			s.FirstUsed = r.Timestamp
		}
		if r.Timestamp.After(s.LastUsed) {
			s.LastUsed = r.Timestamp
		}
	}
	s.SuccessRate = float64(s.SuccessCalls) / float64(s.TotalCalls)
	s.AvgDurationMs = float64(totalDuration) / float64(s.TotalCalls)
	return s
}

// Optimizer runs automatic promotions and demotions based on call statistics.
type Optimizer struct {
	registry *Registry
}

// NewOptimizer creates a new Optimizer.
func NewOptimizer(registry *Registry) *Optimizer {
	return &Optimizer{registry: registry}
}

// Run evaluates all non-archived experts and adjusts their lifecycle status
// based on accumulated call statistics. Returns counts of promoted/archived experts.
func (o *Optimizer) Run() (promoted int, archived int) {
	for _, spec := range o.registry.List() {
		if spec.Status == StatusArchived {
			continue
		}
		records := o.registry.GetCallRecords(spec.Name)
		stats := ComputeStats(records)

		// Draft -> Active: 3+ successful calls
		if spec.Status == StatusDraft && stats.SuccessCalls >= 3 {
			if err := o.registry.Promote(spec.Name, StatusActive); err == nil {
				log.Printf("[optimizer] promoted %s from draft to active (%d successful calls)", spec.Name, stats.SuccessCalls)
				promoted++
			}
		}

		// Active -> Mature: 10+ calls with >=80% success rate
		if spec.Status == StatusActive && stats.TotalCalls >= 10 && stats.SuccessRate >= 0.8 {
			if err := o.registry.Promote(spec.Name, StatusMature); err == nil {
				log.Printf("[optimizer] promoted %s from active to mature (rate: %.2f)", spec.Name, stats.SuccessRate)
				promoted++
			}
		}
	}

	// Archive experts with no recent successful use (30+ days stale)
	archived = o.registry.ArchiveStale(30)
	if archived > 0 {
		log.Printf("[optimizer] archived %d stale experts", archived)
	}

	return
}
