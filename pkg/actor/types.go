package actor

import (
	"context"
)

// State represents the current state of an actor
type State string

const (
	StateIdle    State = "idle"
	StateActive  State = "active"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
	StateError   State = "error"
)

// Actor defines the interface that all actors must implement
type Actor interface {
	ID() string
	State() State
}

// MessageEnvelope wraps a message with metadata
type MessageEnvelope struct {
	Recipient Actor
	Message   interface{}
	Metadata  map[string]interface{}
}

// ReceiverFunc defines the function signature for message handling
type ReceiverFunc func(ctx context.Context, envelope *MessageEnvelope) error

// MiddlewareFunc defines the function signature for middleware
type MiddlewareFunc func(next ReceiverFunc) ReceiverFunc
