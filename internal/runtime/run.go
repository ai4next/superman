package runtime

import (
	"context"
	"errors"
	"iter"

	"google.golang.org/adk/agent"
	adkrunner "google.golang.org/adk/runner"
	adksession "google.golang.org/adk/session"
	"google.golang.org/genai"
)

type RunRequest struct {
	AppName       string
	UserID        string
	SessionID     string
	Message       *genai.Content
	StateDelta    map[string]any
	Config        agent.RunConfig
	Compact       Compactor
	LoopDetection LoopDetectionConfig
}

type Compactor interface {
	Compact(appName, userID, sessionID string) (bool, int, error)
}

// StreamRun converts an ADK runner iterator into Superman runtime events.
func StreamRun(ctx context.Context, runner *adkrunner.Runner, req RunRequest, broker *Broker) iter.Seq2[Event, error] {
	return func(yield func(Event, error) bool) {
		started := RunStarted(req.SessionID, "")
		if broker != nil {
			broker.Publish(started)
		}
		if !yield(started, nil) {
			return
		}

		loopDetector := NewLoopDetector(req.LoopDetection)
		for adkEvent, err := range runner.Run(ctx, req.UserID, req.SessionID, req.Message, req.Config, adkrunner.WithStateDelta(req.StateDelta)) {
			if err != nil {
				if errors.Is(err, context.Canceled) {
					canceled := RunCanceled(req.SessionID, runID(adkEvent))
					if broker != nil {
						broker.Publish(canceled)
					}
					yield(canceled, nil)
					return
				}
				failed := RunFailed(req.SessionID, runID(adkEvent), err)
				if broker != nil {
					broker.Publish(failed)
				}
				yield(failed, err)
				return
			}
			for _, event := range FromADKEvent(req.SessionID, adkEvent) {
				if started.RunID == "" && event.RunID != "" {
					started.RunID = event.RunID
				}
				if err := loopDetector.Observe(event); err != nil {
					failed := RunFailed(req.SessionID, started.RunID, err)
					if broker != nil {
						broker.Publish(failed)
					}
					yield(failed, err)
					return
				}
				if broker != nil {
					broker.Publish(event)
				}
				if !yield(event, nil) {
					return
				}
			}
		}
		if err := ctx.Err(); errors.Is(err, context.Canceled) {
			canceled := RunCanceled(req.SessionID, started.RunID)
			if broker != nil {
				broker.Publish(canceled)
			}
			yield(canceled, nil)
			return
		}

		if req.Compact != nil {
			compacted, count, err := req.Compact.Compact(req.AppName, req.UserID, req.SessionID)
			if err != nil {
				failed := RunFailed(req.SessionID, started.RunID, err)
				if broker != nil {
					broker.Publish(failed)
				}
				yield(failed, err)
				return
			}
			if compacted {
				event := SessionCompacted(req.SessionID, count)
				if broker != nil {
					broker.Publish(event)
				}
				yield(event, nil)
			}
		}
		finished := RunFinished(req.SessionID, started.RunID)
		if broker != nil {
			broker.Publish(finished)
		}
		yield(finished, nil)
	}
}

func runID(event *adksession.Event) string {
	if event == nil {
		return ""
	}
	return event.InvocationID
}
