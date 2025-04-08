package models

import (
	"context"
	"fmt"
)

// ModelError represents an error from the model provider
type ModelError struct {
	Message string
	Code    int
}

func (e *ModelError) Error() string {
	return fmt.Sprintf("model error (code %d): %s", e.Code, e.Message)
}

// BaseModelProvider provides a base implementation of ModelProvider
type BaseModelProvider struct {
	defaultOptions *GenerateOptions
}

// NewBaseModelProvider creates a new base model provider
func NewBaseModelProvider(defaultOptions *GenerateOptions) *BaseModelProvider {
	if defaultOptions == nil {
		defaultOptions = DefaultGenerateOptions()
	}
	return &BaseModelProvider{
		defaultOptions: defaultOptions,
	}
}

// Generate implements the ModelProvider interface
func (p *BaseModelProvider) Generate(ctx context.Context, messages []Message, opts *GenerateOptions) (*Response, error) {
	if len(messages) == 0 {
		return nil, &ModelError{
			Message: "no messages provided",
			Code:    400,
		}
	}

	// Merge options with defaults
	merged := *p.defaultOptions
	if opts != nil {
		if opts.Temperature != 0 {
			merged.Temperature = opts.Temperature
		}
		if opts.MaxTokens != 0 {
			merged.MaxTokens = opts.MaxTokens
		}
		if opts.TopP != 0 {
			merged.TopP = opts.TopP
		}
		if opts.FrequencyPenalty != 0 {
			merged.FrequencyPenalty = opts.FrequencyPenalty
		}
		if opts.PresencePenalty != 0 {
			merged.PresencePenalty = opts.PresencePenalty
		}
		if len(opts.Stop) > 0 {
			merged.Stop = opts.Stop
		}
		if len(opts.Tools) > 0 {
			merged.Tools = opts.Tools
		}
	}

	// Validate messages
	for _, msg := range messages {
		if msg.Role == "" {
			return nil, &ModelError{
				Message: "message role cannot be empty",
				Code:    400,
			}
		}
		if msg.Content == "" {
			return nil, &ModelError{
				Message: "message content cannot be empty",
				Code:    400,
			}
		}
	}

	// This is a base implementation that should be overridden
	return nil, &ModelError{
		Message: "Generate method not implemented",
		Code:    501,
	}
}

// MergeOptions merges default options with provided options
func (p *BaseModelProvider) MergeOptions(opts *GenerateOptions) *GenerateOptions {
	if opts == nil {
		return p.defaultOptions
	}

	merged := *p.defaultOptions
	if opts.Temperature != 0 {
		merged.Temperature = opts.Temperature
	}
	if opts.MaxTokens != 0 {
		merged.MaxTokens = opts.MaxTokens
	}
	if opts.TopP != 0 {
		merged.TopP = opts.TopP
	}
	if opts.FrequencyPenalty != 0 {
		merged.FrequencyPenalty = opts.FrequencyPenalty
	}
	if opts.PresencePenalty != 0 {
		merged.PresencePenalty = opts.PresencePenalty
	}
	if len(opts.Stop) > 0 {
		merged.Stop = opts.Stop
	}
	if len(opts.Tools) > 0 {
		merged.Tools = opts.Tools
	}

	return &merged
}

// ValidateMessages validates the message format
func (p *BaseModelProvider) ValidateMessages(messages []Message) error {
	if len(messages) == 0 {
		return NewModelError("no messages provided", nil)
	}

	for i, msg := range messages {
		if msg.Role == "" {
			return NewModelError(fmt.Sprintf("message %d has no role", i), nil)
		}
		if msg.Content == "" {
			return NewModelError(fmt.Sprintf("message %d has no content", i), nil)
		}
	}

	return nil
}

// NewModelError creates a new model error
func NewModelError(message string, cause error) error {
	return &ModelError{
		Message: message,
		Code:    500,
	}
}
