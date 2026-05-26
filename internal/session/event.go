package session

import (
	"iter"

	adksession "google.golang.org/adk/session"
)

type eventsView []*adksession.Event

var _ adksession.Events = (eventsView)(nil)

func (e eventsView) All() iter.Seq[*adksession.Event] {
	return func(yield func(*adksession.Event) bool) {
		for _, event := range e {
			if !yield(event) {
				return
			}
		}
	}
}

func (e eventsView) Len() int { return len(e) }

func (e eventsView) At(i int) *adksession.Event {
	if i < 0 || i >= len(e) {
		return nil
	}
	return e[i]
}
