package runtime

import (
	"context"
	"sync"
)

type Broker struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func NewBroker() *Broker {
	return &Broker{subs: make(map[chan Event]struct{})}
}

func (b *Broker) Subscribe(ctx context.Context) <-chan Event {
	ch := make(chan Event, 32)

	b.mu.Lock()
	b.subs[ch] = struct{}{}
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

	return ch
}

func (b *Broker) Publish(event Event) {
	if b == nil {
		return
	}

	b.mu.RLock()
	subs := make([]chan Event, 0, len(b.subs))
	for sub := range b.subs {
		subs = append(subs, sub)
	}
	b.mu.RUnlock()

	for _, sub := range subs {
		select {
		case sub <- event:
		default:
		}
	}
}
