package runtime

import (
	"context"
	"testing"
	"time"
)

func TestBrokerPublishesToSubscribers(t *testing.T) {
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := broker.Subscribe(ctx)
	want := Event{Type: EventRunStarted, SessionID: "s1"}
	broker.Publish(want)

	select {
	case got := <-events:
		if got.Type != want.Type || got.SessionID != want.SessionID {
			t.Fatalf("event = %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBrokerDropsForSlowSubscribers(t *testing.T) {
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := broker.Subscribe(ctx)
	for i := 0; i < cap(events)+10; i++ {
		broker.Publish(Event{Type: EventTextDelta, Text: "x"})
	}

	if got := len(events); got != cap(events) {
		t.Fatalf("buffer len = %d, want capped at %d", got, cap(events))
	}
}
