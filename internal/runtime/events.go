package runtime

import (
	"encoding/json"
	"fmt"
	"time"

	adksession "google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

type EventType string

const (
	EventRunStarted          EventType = "run_started"
	EventRunFinished         EventType = "run_finished"
	EventRunFailed           EventType = "run_failed"
	EventRunCanceled         EventType = "run_canceled"
	EventTextDelta           EventType = "text_delta"
	EventToolCallStarted     EventType = "tool_call_started"
	EventToolCallFinished    EventType = "tool_call_finished"
	EventPermissionRequested EventType = "permission_requested"
	EventPermissionGranted   EventType = "permission_granted"
	EventPermissionDenied    EventType = "permission_denied"
	EventEvolutionStarted    EventType = "evolution_started"
	EventEvolutionFinished   EventType = "evolution_finished"
	EventEvolutionFailed     EventType = "evolution_failed"
	EventSessionCompacted    EventType = "session_compacted"
)

type Event struct {
	Type      EventType `json:"type"`
	SessionID string    `json:"session_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	At        time.Time `json:"at"`

	Text       string `json:"text,omitempty"`
	ToolID     string `json:"tool_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Args       string `json:"args,omitempty"`
	Result     string `json:"result,omitempty"`
	Status     string `json:"status,omitempty"`
	Error      string `json:"error,omitempty"`
	Permission bool   `json:"permission,omitempty"`
	Auto       bool   `json:"auto,omitempty"`
	Role       string `json:"role,omitempty"`
	Path       string `json:"path,omitempty"`
	Count      int    `json:"count,omitempty"`
}

func RunStarted(sessionID, runID string) Event {
	return Event{Type: EventRunStarted, SessionID: sessionID, RunID: runID, At: time.Now()}
}

func RunFinished(sessionID, runID string) Event {
	return Event{Type: EventRunFinished, SessionID: sessionID, RunID: runID, At: time.Now()}
}

func RunFailed(sessionID, runID string, err error) Event {
	ev := Event{Type: EventRunFailed, SessionID: sessionID, RunID: runID, At: time.Now()}
	if err != nil {
		ev.Error = err.Error()
	}
	return ev
}

func RunCanceled(sessionID, runID string) Event {
	return Event{Type: EventRunCanceled, SessionID: sessionID, RunID: runID, At: time.Now()}
}

func EvolutionStarted(sessionID, role string) Event {
	return Event{Type: EventEvolutionStarted, SessionID: sessionID, Role: role, At: time.Now()}
}

func EvolutionFinished(sessionID, role, path string) Event {
	return Event{Type: EventEvolutionFinished, SessionID: sessionID, Role: role, Path: path, At: time.Now()}
}

func EvolutionFailed(sessionID, role string, err error) Event {
	event := Event{Type: EventEvolutionFailed, SessionID: sessionID, Role: role, At: time.Now()}
	if err != nil {
		event.Error = err.Error()
	}
	return event
}

func SessionCompacted(sessionID string, count int) Event {
	return Event{Type: EventSessionCompacted, SessionID: sessionID, Count: count, At: time.Now()}
}

func FromADKEvent(sessionID string, event *adksession.Event) []Event {
	if event == nil {
		return nil
	}
	var out []Event
	if event.Content != nil {
		for _, part := range event.Content.Parts {
			if part.Text != "" {
				out = append(out, Event{
					Type:      EventTextDelta,
					SessionID: sessionID,
					RunID:     event.InvocationID,
					At:        event.Timestamp,
					Text:      part.Text,
				})
			}
			if part.FunctionCall != nil {
				if part.FunctionCall.Name == toolconfirmation.FunctionCallName {
					out = append(out, confirmationEvent(sessionID, event, part.FunctionCall))
					continue
				}
				toolID := firstNonEmpty(part.FunctionCall.ID, part.FunctionCall.Name)
				out = append(out, Event{
					Type:       EventToolCallStarted,
					SessionID:  sessionID,
					RunID:      event.InvocationID,
					At:         event.Timestamp,
					ToolID:     toolID,
					ToolName:   part.FunctionCall.Name,
					Args:       marshalString(part.FunctionCall.Args),
					Permission: hasRequestedConfirmation(event, toolID),
				})
			}
			if part.FunctionResponse != nil {
				out = append(out, Event{
					Type:      EventToolCallFinished,
					SessionID: sessionID,
					RunID:     event.InvocationID,
					At:        event.Timestamp,
					ToolID:    firstNonEmpty(part.FunctionResponse.ID, part.FunctionResponse.Name),
					ToolName:  part.FunctionResponse.Name,
					Result:    marshalString(part.FunctionResponse.Response),
					Status:    toolStatus(part.FunctionResponse.Response),
				})
			}
		}
	}
	for toolID, confirmation := range event.Actions.RequestedToolConfirmations {
		out = append(out, Event{
			Type:       EventPermissionRequested,
			SessionID:  sessionID,
			RunID:      event.InvocationID,
			At:         event.Timestamp,
			ToolID:     toolID,
			Args:       marshalString(confirmation),
			Permission: true,
		})
	}
	return out
}

func confirmationEvent(sessionID string, event *adksession.Event, functionCall *genai.FunctionCall) Event {
	if functionCall == nil {
		return Event{}
	}
	toolID := firstNonEmpty(functionCall.ID, functionCall.Name)
	toolName := toolconfirmation.FunctionCallName
	args := marshalString(functionCall.Args)
	if original, err := toolconfirmation.OriginalCallFrom(functionCall); err == nil && original != nil {
		toolName = original.Name
		args = marshalString(original.Args)
	}
	return Event{
		Type:       EventPermissionRequested,
		SessionID:  sessionID,
		RunID:      event.InvocationID,
		At:         event.Timestamp,
		ToolID:     toolID,
		ToolName:   toolName,
		Args:       args,
		Permission: true,
	}
}

func hasRequestedConfirmation(event *adksession.Event, toolID string) bool {
	if event == nil || toolID == "" {
		return false
	}
	_, ok := event.Actions.RequestedToolConfirmations[toolID]
	return ok
}

func marshalString(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

func toolStatus(resp map[string]any) string {
	if resp == nil {
		return "done"
	}
	if value, ok := resp["status"].(string); ok && value != "" {
		return value
	}
	if value, ok := resp["error"].(string); ok && value != "" {
		return "error"
	}
	return "done"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
