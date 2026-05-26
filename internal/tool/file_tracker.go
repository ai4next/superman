package tool

import (
	supermansession "github.com/ai4next/superman/internal/session"
	adktool "google.golang.org/adk/tool"
)

func recordFileRead(tctx adktool.Context, path string) {
	if tctx == nil {
		return
	}
	supermansession.AddFileRead(tctx.Actions(), path)
}

func recordFileWrite(tctx adktool.Context, path string) {
	if tctx == nil {
		return
	}
	supermansession.AddFileWrite(tctx.Actions(), path)
}

func recordFileRevision(tctx adktool.Context, path, action, before, after string, beforeMissing bool) {
	if tctx == nil {
		return
	}
	supermansession.AddFileRevision(tctx.Actions(), supermansession.FileRevisionNote{
		Path:          path,
		Action:        action,
		Before:        before,
		After:         after,
		BeforeMissing: beforeMissing,
	})
}
