package permission

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

type Decision string

const (
	DecisionPending Decision = "pending"
	DecisionGranted Decision = "granted"
	DecisionDenied  Decision = "denied"
)

type NotificationType string

const (
	NotificationRequested NotificationType = "requested"
	NotificationGranted   NotificationType = "granted"
	NotificationDenied    NotificationType = "denied"
)

type CreateRequest struct {
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Action      string `json:"action,omitempty"`
	Description string `json:"description,omitempty"`
	Input       any    `json:"input,omitempty"`
	Path        string `json:"path,omitempty"`
}

type PermissionRequest struct {
	ID          string `json:"id"`
	SessionID   string `json:"session_id"`
	ToolCallID  string `json:"tool_call_id"`
	ToolName    string `json:"tool_name"`
	Action      string `json:"action,omitempty"`
	Description string `json:"description,omitempty"`
	Input       any    `json:"input,omitempty"`
	Path        string `json:"path,omitempty"`
}

type Notification struct {
	Type        NotificationType  `json:"type"`
	RequestID   string            `json:"request_id,omitempty"`
	SessionID   string            `json:"session_id,omitempty"`
	ToolCallID  string            `json:"tool_call_id,omitempty"`
	ToolName    string            `json:"tool_name,omitempty"`
	Action      string            `json:"action,omitempty"`
	Decision    Decision          `json:"decision"`
	Auto        bool              `json:"auto,omitempty"`
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Service struct {
	mu                 sync.RWMutex
	policy             Policy
	pending            map[string]chan bool
	sessionPermissions map[permissionKey]bool
	requestSubs        map[chan PermissionRequest]struct{}
	notificationSubs   map[chan Notification]struct{}
}

type permissionKey struct {
	SessionID string
	ToolName  string
	Action    string
	Path      string
}

func NewService(policy Policy) *Service {
	return &Service{
		policy:             policy,
		pending:            make(map[string]chan bool),
		sessionPermissions: make(map[permissionKey]bool),
		requestSubs:        make(map[chan PermissionRequest]struct{}),
		notificationSubs:   make(map[chan Notification]struct{}),
	}
}

func (s *Service) RequiresConfirmation(req Request) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.policy.RequiresConfirmation(req)
}

func (s *Service) Request(ctx context.Context, opts CreateRequest) (bool, error) {
	if s == nil {
		return false, nil
	}
	if !s.RequiresConfirmation(Request{ToolName: opts.ToolName, Action: opts.Action, Input: opts.Input}) {
		s.publishNotification(Notification{
			Type:       NotificationGranted,
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			ToolName:   opts.ToolName,
			Action:     opts.Action,
			Decision:   DecisionGranted,
			Auto:       true,
		})
		return true, nil
	}

	key := permissionKey{
		SessionID: opts.SessionID,
		ToolName:  opts.ToolName,
		Action:    opts.Action,
		Path:      opts.Path,
	}
	s.mu.RLock()
	if s.sessionPermissions[key] {
		s.mu.RUnlock()
		s.publishNotification(Notification{
			Type:       NotificationGranted,
			SessionID:  opts.SessionID,
			ToolCallID: opts.ToolCallID,
			ToolName:   opts.ToolName,
			Action:     opts.Action,
			Decision:   DecisionGranted,
			Auto:       true,
		})
		return true, nil
	}
	s.mu.RUnlock()

	permission := PermissionRequest{
		ID:          uuid.NewString(),
		SessionID:   opts.SessionID,
		ToolCallID:  opts.ToolCallID,
		ToolName:    opts.ToolName,
		Action:      opts.Action,
		Description: opts.Description,
		Input:       opts.Input,
		Path:        opts.Path,
	}

	resp := make(chan bool, 1)
	s.mu.Lock()
	s.pending[permission.ID] = resp
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, permission.ID)
		s.mu.Unlock()
	}()

	s.publishRequest(permission)
	s.publishNotification(Notification{
		Type:        NotificationRequested,
		RequestID:   permission.ID,
		SessionID:   permission.SessionID,
		ToolCallID:  permission.ToolCallID,
		ToolName:    permission.ToolName,
		Action:      permission.Action,
		Decision:    DecisionPending,
		Description: permission.Description,
	})

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case granted := <-resp:
		return granted, nil
	}
}

func (s *Service) Grant(permission PermissionRequest) {
	s.resolve(permission, true, false)
}

func (s *Service) GrantSession(permission PermissionRequest) {
	s.resolve(permission, true, true)
}

func (s *Service) Deny(permission PermissionRequest) {
	s.resolve(permission, false, false)
}

func (s *Service) SubscribeRequests(ctx context.Context) <-chan PermissionRequest {
	ch := make(chan PermissionRequest, 8)
	s.mu.Lock()
	s.requestSubs[ch] = struct{}{}
	s.mu.Unlock()
	go s.unsubscribe(ctx, ch, nil)
	return ch
}

func (s *Service) SubscribeNotifications(ctx context.Context) <-chan Notification {
	ch := make(chan Notification, 16)
	s.mu.Lock()
	s.notificationSubs[ch] = struct{}{}
	s.mu.Unlock()
	go s.unsubscribe(ctx, nil, ch)
	return ch
}

func (s *Service) resolve(permission PermissionRequest, granted bool, persist bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if persist && granted {
		s.sessionPermissions[permissionKey{
			SessionID: permission.SessionID,
			ToolName:  permission.ToolName,
			Action:    permission.Action,
			Path:      permission.Path,
		}] = true
	}
	resp := s.pending[permission.ID]
	s.mu.Unlock()

	if resp != nil {
		resp <- granted
	}

	decision := DecisionDenied
	notificationType := NotificationDenied
	if granted {
		decision = DecisionGranted
		notificationType = NotificationGranted
	}
	s.publishNotification(Notification{
		Type:       notificationType,
		RequestID:  permission.ID,
		SessionID:  permission.SessionID,
		ToolCallID: permission.ToolCallID,
		ToolName:   permission.ToolName,
		Action:     permission.Action,
		Decision:   decision,
	})
}

func (s *Service) publishRequest(req PermissionRequest) {
	s.mu.RLock()
	subs := make([]chan PermissionRequest, 0, len(s.requestSubs))
	for sub := range s.requestSubs {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()
	for _, sub := range subs {
		select {
		case sub <- req:
		default:
		}
	}
}

func (s *Service) publishNotification(notification Notification) {
	s.mu.RLock()
	subs := make([]chan Notification, 0, len(s.notificationSubs))
	for sub := range s.notificationSubs {
		subs = append(subs, sub)
	}
	s.mu.RUnlock()
	for _, sub := range subs {
		select {
		case sub <- notification:
		default:
		}
	}
}

func (s *Service) unsubscribe(ctx context.Context, reqCh chan PermissionRequest, notificationCh chan Notification) {
	<-ctx.Done()
	s.mu.Lock()
	if reqCh != nil {
		if _, ok := s.requestSubs[reqCh]; ok {
			delete(s.requestSubs, reqCh)
			close(reqCh)
		}
	}
	if notificationCh != nil {
		if _, ok := s.notificationSubs[notificationCh]; ok {
			delete(s.notificationSubs, notificationCh)
			close(notificationCh)
		}
	}
	s.mu.Unlock()
}
