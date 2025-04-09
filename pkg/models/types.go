package models

import (
	"context"
	"time"
)

// BaseMessage represents a message in the system
type BaseMessage struct {
	Role     string                 `json:"role"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Response represents a model's response from the original API
type Response struct {
	Text       string                 `json:"text"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Usage      *Usage                 `json:"usage,omitempty"`
	FinishTime time.Time              `json:"finish_time"`
}

// Usage tracks token usage for a model response
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// BaseGenerateOptions contains options for text generation
type BaseGenerateOptions struct {
	Temperature      float64    `json:"temperature"`
	MaxTokens        int        `json:"max_tokens"`
	StopSequences    []string   `json:"stop_sequences,omitempty"`
	TopP             float64    `json:"top_p"`
	FrequencyPenalty float64    `json:"frequency_penalty"`
	PresencePenalty  float64    `json:"presence_penalty"`
	Functions        []Function `json:"functions,omitempty"`
}

// Function represents a callable function definition (legacy format)
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// TextGenerationProvider defines the interface for model providers that generate text
type TextGenerationProvider interface {
	// Generate generates text based on the provided messages
	Generate(ctx context.Context, messages []BaseMessage, opts *BaseGenerateOptions) (*Response, error)
}

// CreateDefaultGenerateOptions returns default generation options
func CreateDefaultGenerateOptions() *BaseGenerateOptions {
	return &BaseGenerateOptions{
		Temperature:      0.7,
		MaxTokens:        2000,
		TopP:             1.0,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
	}
}
