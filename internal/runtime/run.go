package runtime

import (
	"context"
	"errors"
	"iter"

	"github.com/ai4next/superman/internal/bus"
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
func StreamRun(ctx context.Context, runner *adkrunner.Runner, req RunRequest, broker bus.Broker) iter.Seq2[bus.Event, error] {
	return func(yield func(bus.Event, error) bool) {
		started := bus.RunStarted(req.SessionID, "")
		publish(ctx, broker, started)
		if !yield(started, nil) {
			return
		}

		loopDetector := NewLoopDetector(req.LoopDetection)
		for adkEvent, err := range runner.Run(ctx, req.UserID, req.SessionID, req.Message, req.Config, adkrunner.WithStateDelta(req.StateDelta)) {
			if err != nil {
				if errors.Is(err, context.Canceled) {
					canceled := bus.RunCanceled(req.SessionID, runID(adkEvent))
					publish(ctx, broker, canceled)
					yield(canceled, nil)
					return
				}
				failed := bus.RunFailed(req.SessionID, runID(adkEvent), err)
				publish(ctx, broker, failed)
				yield(failed, err)
				return
			}
			for _, event := range bus.FromADKEvent(req.SessionID, adkEvent) {
				if started.RunID == "" && event.RunID != "" {
					started.RunID = event.RunID
				}
				if err := loopDetector.Observe(event); err != nil {
					failed := bus.RunFailed(req.SessionID, started.RunID, err)
					publish(ctx, broker, failed)
					yield(failed, err)
					return
				}
				publish(ctx, broker, event)
				if !yield(event, nil) {
					return
				}
			}
		}
		if err := ctx.Err(); errors.Is(err, context.Canceled) {
			canceled := bus.RunCanceled(req.SessionID, started.RunID)
			publish(ctx, broker, canceled)
			yield(canceled, nil)
			return
		}

		if req.Compact != nil {
			compacted, count, err := req.Compact.Compact(req.AppName, req.UserID, req.SessionID)
			if err != nil {
				failed := bus.RunFailed(req.SessionID, started.RunID, err)
				publish(ctx, broker, failed)
				yield(failed, err)
				return
			}
			if compacted {
				event := bus.SessionCompacted(req.SessionID, count)
				publish(ctx, broker, event)
				yield(event, nil)
			}
		}
		finished := bus.RunFinished(req.SessionID, started.RunID)
		publish(ctx, broker, finished)
		yield(finished, nil)
	}
}

func publish(ctx context.Context, broker bus.Broker, event bus.Event) {
	if broker == nil {
		return
	}
	_ = broker.Publish(ctx, event)
}

func runID(event *adksession.Event) string {
	if event == nil {
		return ""
	}
	return event.InvocationID
}
