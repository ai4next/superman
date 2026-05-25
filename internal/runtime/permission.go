package runtime

import (
	"context"

	"github.com/ai4next/superman/internal/permission"
)

func BridgePermissionNotifications(ctx context.Context, service *permission.Service, broker *Broker) {
	if service == nil || broker == nil {
		return
	}
	notifications := service.SubscribeNotifications(ctx)
	go func() {
		for notification := range notifications {
			broker.Publish(FromPermissionNotification(notification))
		}
	}()
}
