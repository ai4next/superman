package session

import (
	"iter"
	"maps"
	"sync"

	adksession "google.golang.org/adk/session"
)

type stateView struct {
	mu    sync.RWMutex
	state map[string]any
}

var _ adksession.State = (*stateView)(nil)

func (s *stateView) Get(key string) (any, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.state[key]
	if !ok {
		return nil, adksession.ErrStateKeyNotExist
	}
	return val, nil
}

func (s *stateView) Set(key string, val any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == nil {
		s.state = make(map[string]any)
	}
	s.state[key] = val
	return nil
}

func (s *stateView) All() iter.Seq2[string, any] {
	s.mu.RLock()
	cp := maps.Clone(s.state)
	s.mu.RUnlock()
	return func(yield func(string, any) bool) {
		for k, v := range cp {
			if !yield(k, v) {
				return
			}
		}
	}
}
