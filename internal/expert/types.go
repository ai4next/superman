package expert

// Spec defines an expert agent's identity and system prompt.
type Spec struct {
	Name         string    `yaml:"name" json:"name"`
	SystemPrompt string    `yaml:"prompt" json:"prompt"`
}
