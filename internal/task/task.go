package task

import (
	"context"
	"log"
	"sync"
	"time"
)

// Loop runs fn on every tick until ctx is cancelled. Errors from fn are logged.
func Loop(ctx context.Context, interval time.Duration, fn func(context.Context) error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := fn(ctx); err != nil {
				log.Printf("[task] %v", err)
			}
		}
	}
}

// TickerGroup manages concurrent background loops with shared cancellation.
type TickerGroup struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewTickerGroup creates a TickerGroup from a parent context.
func NewTickerGroup(ctx context.Context) *TickerGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &TickerGroup{ctx: ctx, cancel: cancel}
}

// Go starts fn in a background loop at the given interval.
func (g *TickerGroup) Go(interval time.Duration, fn func(context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		Loop(g.ctx, interval, fn)
	}()
}

// Stop cancels the group context and waits for all goroutines to finish.
func (g *TickerGroup) Stop() {
	g.cancel()
	g.wg.Wait()
}