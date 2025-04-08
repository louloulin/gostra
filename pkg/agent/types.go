package agent

import (
	"time"

	"github.com/asynkron/protoactor-go/actor"
)

// Actor defines the interface that all agents must implement
type Actor interface {
	actor.Actor
}

// ActorContext provides context for actor operations
type ActorContext struct {
	actor.Context
	Network *AgentNetwork
}

// ActorBehavior defines the behavior of an actor
type ActorBehavior interface {
	// OnStart is called when the actor starts
	OnStart(ctx *ActorContext) error

	// OnStop is called when the actor stops
	OnStop(ctx *ActorContext) error

	// OnReceive is called when the actor receives a message
	OnReceive(ctx *ActorContext, message interface{}) error
}

// ActorOptions contains configuration for creating an actor
type ActorOptions struct {
	ID          string
	Name        string
	Description string
	Behavior    ActorBehavior
}

// Constants for timeouts and other configurations
const (
	DefaultTimeout = 5 * time.Second
	MaxRetries     = 3
)

// Message types for actor communication
const (
	MessageTypeCommand = "command"
	MessageTypeEvent   = "event"
	MessageTypeQuery   = "query"
	MessageTypeReply   = "reply"
)
