package task

import (
	"context"
	"log"

	"github.com/ai4next/superman/internal/memory"
)

// NewEvolutionFn returns a loop body that evolves memory entries into candidate insights.
func NewEvolutionFn(svc *memory.Service, candidateDir string) func(context.Context) error {
	return func(ctx context.Context) error {
		candidates, err := svc.Evolve(ctx, candidateDir)
		if err != nil {
			log.Printf("[memory] evolution warning: %v", err)
			return nil
		}
		if len(candidates) > 0 {
			log.Printf("[memory] evolution wrote %d candidates", len(candidates))
		}
		return nil
	}
}