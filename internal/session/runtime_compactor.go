package session

import (
	"context"

	adksession "google.golang.org/adk/session"
)

type RuntimeCompactor struct {
	Service adksession.Service
	Options CompactOptions
}

func (c RuntimeCompactor) Compact(ctx context.Context, appName, userID, sessionID string) (bool, int, error) {
	if c.Service == nil {
		return false, 0, nil
	}
	result, err := CompactContext(ctx, c.Service, appName, userID, sessionID, c.Options)
	if err != nil {
		return false, result.Scanned, err
	}
	return result.Compacted, result.Scanned, nil
}
