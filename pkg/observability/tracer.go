package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracerProvider wraps the OpenTelemetry tracer
type TracerProvider struct {
	tracer trace.Tracer
}

// NewTracerProvider creates a new tracer provider
func NewTracerProvider(serviceName string) *TracerProvider {
	return &TracerProvider{
		tracer: otel.Tracer(serviceName),
	}
}

// StartSpan starts a new trace span
func (t *TracerProvider) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// ActorTracer provides tracing capabilities for actors
type ActorTracer struct {
	provider *TracerProvider
}

// NewActorTracer creates a new actor tracer
func NewActorTracer(provider *TracerProvider) *ActorTracer {
	return &ActorTracer{
		provider: provider,
	}
}

// TraceMessage traces a message through the actor system
func (t *ActorTracer) TraceMessage(ctx context.Context, actorID string, msgType string, fn func(context.Context) error) error {
	spanName := fmt.Sprintf("actor.%s.handle.%s", actorID, msgType)
	ctx, span := t.provider.StartSpan(ctx, spanName)
	defer span.End()

	startTime := time.Now()

	// Add basic attributes
	span.SetAttributes(
		attribute.String("actor.id", actorID),
		attribute.String("message.type", msgType),
	)

	// Execute the handler
	err := fn(ctx)

	// Record duration and error status
	duration := time.Since(startTime)
	span.SetAttributes(
		attribute.Int64("duration_ms", duration.Milliseconds()),
		attribute.Bool("error", err != nil),
	)

	if err != nil {
		span.RecordError(err)
	}

	return err
}

// TraceWorkflow traces a complete workflow execution
func (t *ActorTracer) TraceWorkflow(ctx context.Context, workflowID string) (context.Context, trace.Span) {
	return t.provider.StartSpan(ctx, fmt.Sprintf("workflow.%s", workflowID),
		trace.WithAttributes(attribute.String("workflow.id", workflowID)))
}

// TraceStep traces a single step in a workflow
func (t *ActorTracer) TraceStep(ctx context.Context, stepID string) (context.Context, trace.Span) {
	return t.provider.StartSpan(ctx, fmt.Sprintf("step.%s", stepID),
		trace.WithAttributes(attribute.String("step.id", stepID)))
}

// TraceTool traces a tool execution
func (t *ActorTracer) TraceTool(ctx context.Context, toolName string) (context.Context, trace.Span) {
	return t.provider.StartSpan(ctx, fmt.Sprintf("tool.%s", toolName),
		trace.WithAttributes(attribute.String("tool.name", toolName)))
}
