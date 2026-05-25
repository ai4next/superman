package tool

import (
	"log"

	adktool "google.golang.org/adk/tool"
)

func recordFileRead(tctx adktool.Context, deps Dependencies, path string) {
	if deps.FileTracker == nil || tctx == nil {
		return
	}
	if err := deps.FileTracker.RecordFileRead(tctx.AppName(), tctx.UserID(), tctx.SessionID(), path); err != nil {
		log.Printf("[tool] record file read %s: %v", path, err)
	}
}

func recordFileWrite(tctx adktool.Context, deps Dependencies, path string) {
	if deps.FileTracker == nil || tctx == nil {
		return
	}
	if err := deps.FileTracker.RecordFileWrite(tctx.AppName(), tctx.UserID(), tctx.SessionID(), path); err != nil {
		log.Printf("[tool] record file write %s: %v", path, err)
	}
}

func recordFileRevision(tctx adktool.Context, deps Dependencies, path, action, before, after string, beforeMissing bool) {
	if deps.FileTracker == nil || tctx == nil {
		return
	}
	if _, err := deps.FileTracker.RecordFileRevision(tctx.AppName(), tctx.UserID(), tctx.SessionID(), path, action, before, after, beforeMissing); err != nil {
		log.Printf("[tool] record file revision %s: %v", path, err)
	}
}
