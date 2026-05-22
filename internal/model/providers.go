package model

import (
	"context"
	"fmt"
	"os"
	"strings"

	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"

	"github.com/achetronic/adk-utils-go/genai/anthropic"
	"github.com/achetronic/adk-utils-go/genai/openai"

	"github.com/ai4next/superman/internal/config"
)

// New creates a model based on the provider configuration.
// Supports:
//   - gemini   → Vertex AI (Gemini models)
//   - openai   → OpenAI API or compatible providers (DeepSeek, Ollama, etc.)
//   - deepseek → OpenAI-compatible (DeepSeek API)
//   - claude   → Anthropic API (Claude models)
//   - ollama   → OpenAI-compatible (local Ollama)
//   - any other → OpenAI-compatible fallback
func New(ctx context.Context, cfg config.ModelConfig) (model.LLM, error) {
	provider := strings.ToLower(cfg.Provider)

	switch provider {
	case "gemini":
		projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
		location := os.Getenv("GOOGLE_CLOUD_AGENT_ENGINE_LOCATION")
		return gemini.NewModel(ctx, cfg.Name, &genai.ClientConfig{
			Backend:  genai.BackendVertexAI,
			Project:  projectID,
			Location: location,
		})

	case "claude":
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for claude provider")
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com"
		}
		cfg := anthropic.Config{
			APIKey:    apiKey,
			BaseURL:   baseURL,
			ModelName: cfg.Name,
		}
		return anthropic.New(cfg), nil

	default:
		// OpenAI-compatible: openai, deepseek, ollama, qwen, kimi, etc.
		apiKey := cfg.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = "https://api.openai.com/v1"
		}
		cfg := openai.Config{
			APIKey:    apiKey,
			BaseURL:   baseURL,
			ModelName: cfg.Name,
		}
		return openai.New(cfg), nil
	}
}

// MustNew calls New and panics on error.
func MustNew(ctx context.Context, cfg config.ModelConfig) model.LLM {
	m, err := New(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("failed to create model: %v", err))
	}
	return m
}
