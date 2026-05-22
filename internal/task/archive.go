package task

import (
	"context"
	"log"
	"time"

	"github.com/ai4next/superman/internal/memory"
)

// NewArchiveFn returns a loop body that archives old (L2→L3) memory entries.
func NewArchiveFn(svc *memory.Service, ttl time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		if archived, _ := svc.Archive(ctx, ttl); archived > 0 {
			log.Printf("[memory] archived %d entries", archived)
		}
		return nil
	}
}