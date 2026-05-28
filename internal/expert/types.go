package expert

// Spec defines an expert agent's identity and system prompt.
type Spec struct {
	Name         string `json:"name"`
	SystemPrompt string `json:"prompt"`
}
