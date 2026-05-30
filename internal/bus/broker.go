package bus

import (
	"context"
	"sync"
)

type EventFilter struct {
	Types  []EventType
	TaskID string
}

type Broker interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error)
}

type MemoryBroker struct {
	mu   sync.RWMutex
	subs map[chan Event]EventFilter
}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{subs: make(map[chan Event]EventFilter)}
}

func (b *MemoryBroker) Publish(ctx context.Context, event Event) error {
	if event.At.IsZero() {
		event.At = nowUTC()
	}
	b.mu.RLock()
	subs := make(map[chan Event]EventFilter, len(b.subs))
	for ch, filter := range b.subs {
		subs[ch] = filter
	}
	b.mu.RUnlock()
	for ch, filter := range subs {
		if !eventMatchesFilter(event, filter) {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- event:
		default:
		}
	}
	return nil
}

func (b *MemoryBroker) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	ch := make(chan Event, 32)
	b.mu.Lock()
	b.subs[ch] = filter
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
			close(ch)
		}
		b.mu.Unlock()
	}()
	return ch, nil
}

func eventMatchesFilter(event Event, filter EventFilter) bool {
	if filter.TaskID != "" && event.TaskID != filter.TaskID {
		return false
	}
	if len(filter.Types) == 0 {
		return true
	}
	for _, typ := range filter.Types {
		if event.Type == typ {
			return true
		}
	}
	return false
}
