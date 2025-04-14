package observability

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type mockSpan struct {
	name       string
	attributes map[string]interface{}
	ended      bool
	errors     []error
}

func (s *mockSpan) End(options ...trace.SpanEndOption) {
	s.ended = true
}

func (s *mockSpan) AddEvent(name string, opts ...trace.EventOption) {}
func (s *mockSpan) IsRecording() bool                               { return true }
func (s *mockSpan) RecordError(err error, opts ...trace.EventOption) {
	s.errors = append(s.errors, err)
}
func (s *mockSpan) SpanContext() trace.SpanContext                { return trace.SpanContext{} }
func (s *mockSpan) SetStatus(code codes.Code, description string) {}
func (s *mockSpan) SetName(name string)                           { s.name = name }
func (s *mockSpan) SetAttributes(kv ...attribute.KeyValue) {
	if s.attributes == nil {
		s.attributes = make(map[string]interface{})
	}
	for _, attr := range kv {
		s.attributes[string(attr.Key)] = attr.Value.AsInterface()
	}
}
func (s *mockSpan) TracerProvider() trace.TracerProvider { return nil }

type mockTracer struct {
	spans []*mockSpan
}

func (t *mockTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := &mockSpan{name: name}
	t.spans = append(t.spans, span)
	return ctx, span
}

func TestActorTracer(t *testing.T) {
	mockTracer := &mockTracer{}
	provider := &TracerProvider{tracer: mockTracer}
	tracer := NewActorTracer(provider)

	ctx := context.Background()
	actorID := "test_actor"
	msgType := "test_message"

	// Test successful message tracing
	err := tracer.TraceMessage(ctx, actorID, msgType, func(ctx context.Context) error {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(mockTracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(mockTracer.spans))
	}

	span := mockTracer.spans[0]
	expectedName := "actor.test_actor.handle.test_message"
	if span.name != expectedName {
		t.Errorf("Expected span name %s, got %s", expectedName, span.name)
	}

	if !span.ended {
		t.Error("Expected span to be ended")
	}

	// Verify attributes
	attrs := span.attributes
	if attrs["actor.id"] != actorID {
		t.Errorf("Expected actor.id %s, got %v", actorID, attrs["actor.id"])
	}
	if attrs["message.type"] != msgType {
		t.Errorf("Expected message.type %s, got %v", msgType, attrs["message.type"])
	}
	if attrs["error"] != false {
		t.Error("Expected error attribute to be false")
	}
	if duration, ok := attrs["duration_ms"].(int64); !ok || duration <= 0 {
		t.Error("Expected positive duration")
	}

	// Test error tracing
	mockTracer.spans = nil // Reset spans
	testError := errors.New("test error")

	err = tracer.TraceMessage(ctx, actorID, msgType, func(ctx context.Context) error {
		return testError
	})

	if err != testError {
		t.Errorf("Expected error %v, got %v", testError, err)
	}

	if len(mockTracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(mockTracer.spans))
	}

	span = mockTracer.spans[0]
	if attrs := span.attributes; attrs["error"] != true {
		t.Error("Expected error attribute to be true")
	}
	if len(span.errors) != 1 || span.errors[0] != testError {
		t.Error("Expected error to be recorded in span")
	}

	// Test workflow tracing
	mockTracer.spans = nil
	workflowID := "test_workflow"
	var traceSpan trace.Span // Declare traceSpan
	ctx, traceSpan = tracer.TraceWorkflow(ctx, workflowID)
	span, ok := traceSpan.(*mockSpan) // Type assertion
	if !ok {
		t.Fatalf("TraceWorkflow did not return a *mockSpan")
	}

	if len(mockTracer.spans) != 1 {
		t.Fatalf("Expected 1 span, got %d", len(mockTracer.spans))
	}

	span = mockTracer.spans[0]
	expectedName = "workflow.test_workflow"
	if span.name != expectedName {
		t.Errorf("Expected span name %s, got %s", expectedName, span.name)
	}
}
