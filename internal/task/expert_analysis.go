package task

import (
	"context"
	"log"

	"github.com/ai4next/superman/internal/expert"
)

// NewExpertAnalysisFn returns a loop body that runs pattern analysis and
// auto-evolution for expert discovery.
func NewExpertAnalysisFn(analyzer *expert.Analyzer, candidateDir string) func(context.Context) error {
	return func(ctx context.Context) error {
		candidates, err := analyzer.RunAnalysis()
		if err != nil {
			log.Printf("[expert] pattern analysis: %v", err)
			return nil
		}
		if err := analyzer.WriteCandidates(candidateDir, candidates); err != nil {
			log.Printf("[expert] candidate write warning: %v", err)
			return nil
		}
		if len(candidates) > 0 {
			log.Printf("[expert] pattern analysis wrote %d expert candidates", len(candidates))
			for _, c := range candidates {
				log.Printf("[expert]   candidate: %s (confidence: %.2f)", c.Name, c.Confidence)
			}
		}
		records, err := analyzer.AutoEvolve()
		if err != nil {
			log.Printf("[expert] auto evolution warning: %v", err)
			return nil
		}
		if len(records) > 0 {
			log.Printf("[expert] auto evolution applied %d changes", len(records))
		}
		return nil
	}
}