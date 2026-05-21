package tools

import (
	"fmt"
	"sync"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

var (
	checkpointStore = make(map[string]string)
	checkpointMu    sync.RWMutex
)

type checkpointInput struct {
	Key   string `json:"key" jsonschema:"Key for this checkpoint"`
	Value string `json:"value" jsonschema:"Value to store. Empty to delete the key."`
}

type checkpointOutput struct {
	Key     string `json:"key"`
	Stored  bool   `json:"stored"`
	Message string `json:"message"`
}

func newCheckpointTool(deps Dependencies) tool.Tool {
	handler := func(tctx tool.Context, input checkpointInput) (checkpointOutput, error) {
		checkpointMu.Lock()
		defer checkpointMu.Unlock()

		if input.Value == "" {
			delete(checkpointStore, input.Key)
			return checkpointOutput{
				Key:     input.Key,
				Stored:  false,
				Message: fmt.Sprintf("checkpoint '%s' deleted", input.Key),
			}, nil
		}

		checkpointStore[input.Key] = input.Value
		return checkpointOutput{
			Key:     input.Key,
			Stored:  true,
			Message: fmt.Sprintf("checkpoint '%s' saved (%d bytes)", input.Key, len(input.Value)),
		}, nil
	}
	t, _ := functiontool.New(functiontool.Config{
		Name:        "checkpoint",
		Description: "Save or delete working notes (key-value pairs) during a task. Use an empty value to delete a key.",
	}, handler)
	return t
}

// GetCheckpoints returns all current checkpoints for injection into prompts.
func GetCheckpoints() map[string]string {
	checkpointMu.RLock()
	defer checkpointMu.RUnlock()
	result := make(map[string]string, len(checkpointStore))
	for k, v := range checkpointStore {
		result[k] = v
	}
	return result
}