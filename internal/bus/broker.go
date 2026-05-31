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
	mu         sync.RWMutex
	nextSubID  int64
	subs       map[chan Event]*subscription
	deliveries uint64
}

func NewMemoryBroker() *MemoryBroker {
	return &MemoryBroker{
		subs: make(map[chan Event]*subscription),
	}
}

type subscription struct {
	mu     sync.RWMutex
	id     int64
	filter EventFilter
	ctx    context.Context
	ch     chan Event
	closed bool
}

type DeliveryStats struct {
	Subscribers int
	Delivered   uint64
}

func (b *MemoryBroker) Publish(ctx context.Context, event Event) error {
	if event.At.IsZero() {
		event.At = nowUTC()
	}
	b.mu.RLock()
	subs := make([]*subscription, 0, len(b.subs))
	for _, sub := range b.subs {
		if !eventMatchesFilter(event, sub.filter) {
			continue
		}
		subs = append(subs, sub)
	}
	b.mu.RUnlock()
	var delivered uint64
	for _, sub := range subs {
		ok, err := sub.deliver(ctx, event)
		if err != nil {
			return err
		}
		if ok {
			delivered++
		}
	}
	if delivered > 0 {
		b.mu.Lock()
		b.deliveries += delivered
		b.mu.Unlock()
	}
	return nil
}

func (b *MemoryBroker) Subscribe(ctx context.Context, filter EventFilter) (<-chan Event, error) {
	ch := make(chan Event, 64)
	b.mu.Lock()
	b.nextSubID++
	sub := &subscription{id: b.nextSubID, filter: filter, ctx: ctx, ch: ch}
	b.subs[ch] = sub
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		if _, ok := b.subs[ch]; ok {
			delete(b.subs, ch)
		}
		b.mu.Unlock()
		sub.close()
	}()
	return ch, nil
}

func (b *MemoryBroker) Stats() DeliveryStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return DeliveryStats{
		Subscribers: len(b.subs),
		Delivered:   b.deliveries,
	}
}

func (s *subscription) deliver(ctx context.Context, event Event) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return false, nil
	}
	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case <-s.ctx.Done():
		return false, nil
	case s.ch <- event:
		return true, nil
	}
}

func (s *subscription) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
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
