package actor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	proactor "github.com/asynkron/protoactor-go/actor"
	gostrerrors "github.com/louloulin/gostra/pkg/errors"
)

// Context represents the execution context for an actor message
type Context struct {
	actorContext    context.Context
	cancelFunc      context.CancelFunc
	system          *ActorSystem
	sender          *PID
	receiver        *PID
	message         interface{}
	metadata        map[string]interface{}
	responseChannel chan interface{}
	deadline        time.Time
	errorHandler    *gostrerrors.Supervisor
	mu              sync.RWMutex
}

// NewContext creates a new actor context
func NewContext(system *ActorSystem, sender, receiver *PID, message interface{}) *Context {
	ctx, cancel := context.WithCancel(context.Background())

	return &Context{
		actorContext:    ctx,
		cancelFunc:      cancel,
		system:          system,
		sender:          sender,
		receiver:        receiver,
		message:         message,
		metadata:        make(map[string]interface{}),
		responseChannel: make(chan interface{}, 1),
	}
}

// WithDeadline sets a deadline for the context
func (c *Context) WithDeadline(d time.Time) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadline = d
	return c
}

// WithErrorHandler sets an error handler for the context
func (c *Context) WithErrorHandler(handler *gostrerrors.Supervisor) *Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorHandler = handler
	return c
}

// System returns the actor system
func (c *Context) System() *ActorSystem {
	return c.system
}

// Sender returns the sender PID
func (c *Context) Sender() *PID {
	return c.sender
}

// Receiver returns the receiver PID
func (c *Context) Receiver() *PID {
	return c.receiver
}

// Message returns the message
func (c *Context) Message() interface{} {
	return c.message
}

// SetMetadata sets metadata in the context
func (c *Context) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = value
}

// GetMetadata retrieves metadata from the context
func (c *Context) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, exists := c.metadata[key]
	return value, exists
}

// Context returns the underlying context.Context
func (c *Context) Context() context.Context {
	return c.actorContext
}

// Send sends a message to another actor
func (c *Context) Send(target *PID, message interface{}) error {
	if target != nil {
		// Instead of using LogMessage directly, log the message here
		fmt.Printf("Sending message %T to %s-%s\n", message, target.Type, target.ID)
	}
	// Implementation would use the actor system to route the message
	return nil
}

// Request sends a message and waits for a response
func (c *Context) Request(target *PID, message interface{}, timeout time.Duration) (interface{}, error) {
	if target != nil {
		// Instead of using LogMessage directly, log the message here
		fmt.Printf("Sending request %T to %s-%s\n", message, target.Type, target.ID)
	}

	// Set up a timeout context
	ctx, cancel := context.WithTimeout(c.actorContext, timeout)
	defer cancel()

	// Implementation would send the message and wait for response
	select {
	case response := <-c.responseChannel:
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Respond sends a response to the original sender
func (c *Context) Respond(response interface{}) error {
	if c.sender == nil {
		return errors.New("no sender to respond to")
	}
	return c.Send(c.sender, response)
}

// HandleError handles an error using the configured error handler
func (c *Context) HandleError(err error) error {
	if c.errorHandler != nil && c.receiver != nil {
		// Convert our PID to protoactor PID
		protoPID := &proactor.PID{
			Address: "", // Our PID doesn't have an Address field
			Id:      c.receiver.ID,
		}

		return c.errorHandler.HandleFailure(c.actorContext, protoPID, c.receiver.Type, err)
	}
	return err
}

// Cancel cancels the context
func (c *Context) Cancel() {
	c.cancelFunc()
}

// Done returns a channel that's closed when the context is done
func (c *Context) Done() <-chan struct{} {
	return c.actorContext.Done()
}

// Err returns the context's error
func (c *Context) Err() error {
	return c.actorContext.Err()
}

// String returns a string representation of the context
func (c *Context) String() string {
	return fmt.Sprintf("Context{sender=%v, receiver=%v}", c.sender, c.receiver)
}
