package session

import (
	"sync"
	"time"

	adksession "google.golang.org/adk/session"
)

type sessionView struct {
	mu        sync.RWMutex
	appName   string
	userID    string
	sessionID string
	state     map[string]any
	events    []*adksession.Event
	updatedAt time.Time
}

var _ adksession.Session = (*sessionView)(nil)

func (s *sessionView) ID() string              { return s.sessionID }
func (s *sessionView) AppName() string         { return s.appName }
func (s *sessionView) UserID() string          { return s.userID }
func (s *sessionView) State() adksession.State { return &stateView{state: s.state} }

func (s *sessionView) Events() adksession.Events {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return eventsView(cloneEvents(s.events))
}

func (s *sessionView) LastUpdateTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.updatedAt
}

func (s *sessionView) appendEvent(event *adksession.Event, state map[string]any, updatedAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.events = append(s.events, cloneEvent(event))
	s.updatedAt = updatedAt
}
