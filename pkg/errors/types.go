package errors

import (
	"fmt"
)

// ErrorType represents the type of error
type ErrorType string

// ErrorSeverity represents the severity level of an error
type ErrorSeverity int

// Error types
const (
	SystemError  ErrorType = "system"
	ActorError   ErrorType = "actor"
	NetworkError ErrorType = "network"
	ModelError   ErrorType = "model"
	ToolError    ErrorType = "tool"
)

// Error severities
const (
	Info     ErrorSeverity = 1
	Warning  ErrorSeverity = 2
	Error    ErrorSeverity = 3
	Critical ErrorSeverity = 4
	Fatal    ErrorSeverity = 5
)

// AgentError represents an error in the agent system
type AgentError struct {
	Type        ErrorType     `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	Message     string        `json:"message"`
	Cause       error         `json:"cause,omitempty"`
	Recoverable bool          `json:"recoverable"`
}

// Error implements the error interface
func (e *AgentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s error: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s error: %s", e.Type, e.Message)
}

// NewAgentError creates a new agent error
func NewAgentError(errType ErrorType, severity ErrorSeverity, message string, cause error, recoverable bool) *AgentError {
	return &AgentError{
		Type:        errType,
		Severity:    severity,
		Message:     message,
		Cause:       cause,
		Recoverable: recoverable,
	}
}

// IsRecoverable checks if the error is recoverable
func (e *AgentError) IsRecoverable() bool {
	return e.Recoverable
}

// GetSeverity returns the error severity
func (e *AgentError) GetSeverity() ErrorSeverity {
	return e.Severity
}

// GetType returns the error type
func (e *AgentError) GetType() ErrorType {
	return e.Type
}

// GetCause returns the underlying cause of the error
func (e *AgentError) GetCause() error {
	return e.Cause
}
