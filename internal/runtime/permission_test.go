package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/ai4next/superman/internal/permission"
)

func TestBridgePermissionNotifications(t *testing.T) {
	service := permission.NewService(permission.NewPolicy(false, []string{permission.ToolWrite}, nil))
	broker := NewBroker()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := broker.Subscribe(ctx)

	BridgePermissionNotifications(ctx, service, broker)
	_, err := service.Request(context.Background(), permission.CreateRequest{
		SessionID:  "s1",
		ToolCallID: "call-1",
		ToolName:   permission.ToolWrite,
	})
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	select {
	case event := <-events:
		if event.Type != EventPermissionGranted || event.ToolID != "call-1" || !event.Auto {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for runtime permission event")
	}
}
