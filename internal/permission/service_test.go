package permission

import (
	"context"
	"testing"
	"time"
)

func TestServiceRequestGrantAndDeny(t *testing.T) {
	service := NewService(NewPolicy(false, nil, nil))
	requests := service.SubscribeRequests(context.Background())
	notifications := service.SubscribeNotifications(context.Background())

	resultCh := make(chan bool, 1)
	errCh := make(chan error, 1)
	go func() {
		granted, err := service.Request(context.Background(), CreateRequest{
			SessionID:   "s1",
			ToolCallID:  "call-1",
			ToolName:    ToolWrite,
			Action:      "overwrite",
			Description: "write file",
		})
		resultCh <- granted
		errCh <- err
	}()

	req := receiveRequest(t, requests)
	if req.ToolName != ToolWrite || req.Action != "overwrite" {
		t.Fatalf("request = %+v", req)
	}
	if notification := receiveNotification(t, notifications); notification.Type != NotificationRequested {
		t.Fatalf("notification = %+v, want requested", notification)
	}

	service.Grant(req)
	if err := <-errCh; err != nil {
		t.Fatalf("request error: %v", err)
	}
	if granted := <-resultCh; !granted {
		t.Fatal("request should be granted")
	}
	if notification := receiveNotification(t, notifications); notification.Type != NotificationGranted {
		t.Fatalf("notification = %+v, want granted", notification)
	}

	resultCh = make(chan bool, 1)
	errCh = make(chan error, 1)
	go func() {
		granted, err := service.Request(context.Background(), CreateRequest{
			SessionID:  "s1",
			ToolCallID: "call-2",
			ToolName:   ToolPatch,
			Action:     "replace",
		})
		resultCh <- granted
		errCh <- err
	}()

	req = receiveRequest(t, requests)
	receiveNotification(t, notifications)
	service.Deny(req)
	if err := <-errCh; err != nil {
		t.Fatalf("request error: %v", err)
	}
	if granted := <-resultCh; granted {
		t.Fatal("request should be denied")
	}
	if notification := receiveNotification(t, notifications); notification.Type != NotificationDenied {
		t.Fatalf("notification = %+v, want denied", notification)
	}
}

func TestServiceGrantSessionPersistsPermission(t *testing.T) {
	service := NewService(NewPolicy(false, nil, nil))
	requests := service.SubscribeRequests(context.Background())

	resultCh := make(chan bool, 1)
	go func() {
		granted, _ := service.Request(context.Background(), CreateRequest{
			SessionID:  "s1",
			ToolCallID: "call-1",
			ToolName:   ToolCodeRun,
			Action:     "python",
			Path:       "/tmp",
		})
		resultCh <- granted
	}()

	req := receiveRequest(t, requests)
	service.GrantSession(req)
	if granted := <-resultCh; !granted {
		t.Fatal("first request should be granted")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	granted, err := service.Request(ctx, CreateRequest{
		SessionID:  "s1",
		ToolCallID: "call-2",
		ToolName:   ToolCodeRun,
		Action:     "python",
		Path:       "/tmp",
	})
	if err != nil {
		t.Fatalf("second request error: %v", err)
	}
	if !granted {
		t.Fatal("session permission should auto-grant matching request")
	}
}

func TestServiceAllowlistAutoGrants(t *testing.T) {
	service := NewService(NewPolicy(false, []string{ToolWrite}, nil))
	granted, err := service.Request(context.Background(), CreateRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   ToolWrite,
	})
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	if !granted {
		t.Fatal("allowlisted tool should be granted")
	}
}

func receiveRequest(t *testing.T, ch <-chan PermissionRequest) PermissionRequest {
	t.Helper()
	select {
	case req := <-ch:
		return req
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for permission request")
		return PermissionRequest{}
	}
}

func receiveNotification(t *testing.T, ch <-chan Notification) Notification {
	t.Helper()
	select {
	case notification := <-ch:
		return notification
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for permission notification")
		return Notification{}
	}
}
