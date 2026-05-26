package runtime

import (
	"github.com/ai4next/superman/internal/session"
	adksession "google.golang.org/adk/session"
)

func PromptStateDelta(workspace, prompt string) map[string]any {
	actions := adksession.EventActions{StateDelta: make(map[string]any)}
	session.AddPromptReferences(&actions, workspace, prompt)
	if len(actions.StateDelta) == 0 {
		return nil
	}
	return actions.StateDelta
}

func SessionCompactor(sessionService adksession.Service, maxMessages int) Compactor {
	if sessionService == nil {
		return nil
	}
	return session.RuntimeCompactor{
		Service: sessionService,
		Options: session.CompactOptions{
			MaxMessages: maxMessages,
			KeepLast:    20,
		},
	}
}
