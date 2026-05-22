package task

import (
	"context"
	"log"
	"time"

	"github.com/ai4next/superman/internal/memory"
)

// NewL4ArchiveFn returns a loop body that compresses old session files into L4 storage.
func NewL4ArchiveFn(historyPath, memoryDir string, ttl time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		if archived, _ := memory.ArchiveSessions(ctx, historyPath, memoryDir, ttl); archived > 0 {
			log.Printf("[memory] L4 archived %d sessions", archived)
		}
		return nil
	}
}