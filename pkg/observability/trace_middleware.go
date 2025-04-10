package observability

import (
	"context"
	"reflect"

	"github.com/louloulin/gostra/pkg/actor"
)

// TraceMiddleware provides tracing capabilities for actors
type TraceMiddleware struct {
	tracer *ActorTracer
}

// NewTraceMiddleware creates a new tracing middleware
func NewTraceMiddleware(tracer *ActorTracer) *TraceMiddleware {
	return &TraceMiddleware{
		tracer: tracer,
	}
}

// Receive handles incoming messages and traces their execution
func (m *TraceMiddleware) Receive(ctx context.Context, envelope *actor.MessageEnvelope, next actor.ReceiverFunc) error {
	msgType := reflect.TypeOf(envelope.Message).String()
	actorID := envelope.Recipient.ID()

	return m.tracer.TraceMessage(ctx, actorID, msgType, func(ctx context.Context) error {
		return next(ctx, envelope)
	})
}

// WithTracing adds tracing to an actor
func WithTracing(tracer *ActorTracer) actor.MiddlewareFunc {
	middleware := NewTraceMiddleware(tracer)
	return func(next actor.ReceiverFunc) actor.ReceiverFunc {
		return func(ctx context.Context, envelope *actor.MessageEnvelope) error {
			return middleware.Receive(ctx, envelope, next)
		}
	}
}
