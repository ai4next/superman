package session

import (
	"context"
	"fmt"

	adksession "google.golang.org/adk/session"
)

func Import(svc adksession.Service, appName, userID string, data ImportData) (Metadata, error) {
	extended, err := persistentService(svc)
	if err != nil {
		return Metadata{}, err
	}
	return extended.Import(appName, userID, data)
}

func StorageStatsFor(svc adksession.Service) (StorageStats, error) {
	extended, err := persistentService(svc)
	if err != nil {
		return StorageStats{}, err
	}
	return extended.StorageStats()
}

func CleanupOrphanSnapshots(svc adksession.Service, dryRun bool) (SnapshotCleanupResult, error) {
	extended, err := persistentService(svc)
	if err != nil {
		return SnapshotCleanupResult{}, err
	}
	return extended.CleanupOrphanSnapshots(dryRun)
}

func RecordFileRevisionWithMissing(svc adksession.Service, appName, userID, sessionID, path, action, before, after string, beforeMissing, afterMissing bool) (FileRevision, error) {
	if extended, ok := svc.(*Service); ok {
		return extended.RecordFileRevisionWithMissing(appName, userID, sessionID, path, action, before, after, beforeMissing, afterMissing)
	}
	resp, err := svc.Get(context.Background(), &adksession.GetRequest{AppName: appName, UserID: userID, SessionID: sessionID})
	if err != nil {
		return FileRevision{}, err
	}
	if path == "" {
		return FileRevision{}, fmt.Errorf("path is required")
	}
	note := FileRevisionNote{
		Path:          path,
		Action:        action,
		Before:        before,
		After:         after,
		BeforeMissing: beforeMissing,
		AfterMissing:  afterMissing,
	}
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	AddFileRevision(&actions, note)
	if err := appendStateEvent(svc, resp.Session, "file-revision", actions.StateDelta); err != nil {
		return FileRevision{}, err
	}
	revisions, err := FileRevisions(svc, appName, userID, sessionID)
	if err != nil {
		return FileRevision{}, err
	}
	for i := len(revisions) - 1; i >= 0; i-- {
		if revisions[i].Path == note.Path || revisions[i].Path == cleanContextPath(note.Path) {
			return revisions[i], nil
		}
	}
	return buildFileRevision(note.Path, note.Action, note.Before, note.After, note.BeforeMissing, note.AfterMissing)
}

func persistentService(svc adksession.Service) (*Service, error) {
	extended, ok := svc.(*Service)
	if !ok {
		return nil, fmt.Errorf("session service does not support persistent session management")
	}
	return extended, nil
}
